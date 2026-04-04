import type { WSMessage, WSEventType } from "@/shared/types";
import { type Logger, noopLogger } from "@/shared/logger";

type EventHandler = (payload: unknown) => void;

export const enum ConnectionState {
  Idle = "idle",
  Connecting = "connecting",
  Connected = "connected",
  Reconnecting = "reconnecting",
  Failed = "failed",
  Unauthorized = "unauthorized",
}

type ConnectionStateChangeHandler = (state: ConnectionState, prevState: ConnectionState) => void;

interface AuthExpiredMessage {
  type: "auth_expired";
  code?: string;
}

interface TokenExpiredMessage {
  type?: string;
  code: "TOKEN_EXPIRED";
}

function isAuthExpired(msg: unknown): boolean {
  const m = msg as Partial<AuthExpiredMessage & TokenExpiredMessage>;
  return m.type === "auth_expired" || m.code === "TOKEN_EXPIRED";
}

export class WSClient {
  private ws: WebSocket | null = null;
  private baseUrl: string;
  private token: string | null = null;
  private workspaceId: string | null = null;
  private handlers = new Map<WSEventType, Set<EventHandler>>();
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private hasConnectedBefore = false;
  private onReconnectCallbacks = new Set<() => void>();
  private anyHandlers = new Set<(msg: WSMessage) => void>();
  private logger: Logger;
  private reconnectAttempt = 0;
  private readonly maxReconnectDelay = 30000;
  private readonly maxReconnectAttempts = 10;
  private readonly baseReconnectDelay = 1000;

  private _connectionState: ConnectionState = ConnectionState.Idle;
  private readonly _connectionStateHandlers = new Set<ConnectionStateChangeHandler>();
  private readonly _unauthorizedHandlers = new Set<() => void>();

  private _visibilityHidden = false;

  constructor(url: string, options?: { logger?: Logger }) {
    this.baseUrl = url;
    this.logger = options?.logger ?? noopLogger;
    this._setupVisibilityTracking();
  }

  private _setState(state: ConnectionState) {
    const prev = this._connectionState;
    if (prev === state) return;
    this._connectionState = state;
    for (const cb of this._connectionStateHandlers) {
      try {
        cb(state, prev);
      } catch {
        // ignore callback errors
      }
    }
  }

  private _setupVisibilityTracking() {
    if (typeof document === "undefined") return;
    const onVisibilityChange = () => {
      this._visibilityHidden = document.visibilityState === "hidden";
    };
    document.addEventListener("visibilitychange", onVisibilityChange);
  }

  get connectionState(): ConnectionState {
    return this._connectionState;
  }

  onConnectionStateChange(handler: ConnectionStateChangeHandler): () => void {
    this._connectionStateHandlers.add(handler);
    return () => this._connectionStateHandlers.delete(handler);
  }

  onUnauthorized(handler: () => void): () => void {
    this._unauthorizedHandlers.add(handler);
    return () => this._unauthorizedHandlers.delete(handler);
  }

  setAuth(token: string, workspaceId: string) {
    this.token = token;
    this.workspaceId = workspaceId;
  }

  connect() {
    if (this._connectionState === ConnectionState.Connected || this._connectionState === ConnectionState.Connecting) {
      return;
    }
    if (this._connectionState === ConnectionState.Unauthorized) {
      return;
    }

    this._setState(ConnectionState.Connecting);

    const url = new URL(this.baseUrl);
    if (this.token) url.searchParams.set("token", this.token);
    if (this.workspaceId)
      url.searchParams.set("workspace_id", this.workspaceId);

    this.ws = new WebSocket(url.toString());

    this.ws.onopen = () => {
      this.logger.info("connected");
      this.reconnectAttempt = 0;
      this._setState(ConnectionState.Connected);
      if (this.hasConnectedBefore) {
        for (const cb of this.onReconnectCallbacks) {
          try {
            cb();
          } catch {
            // ignore reconnect callback errors
          }
        }
      }
      this.hasConnectedBefore = true;
    };

    this.ws.onmessage = (event) => {
      let msg: WSMessage;
      try {
        msg = JSON.parse(event.data as string) as WSMessage;
      } catch {
        this.logger.warn("failed to parse WebSocket message, skipping");
        return;
      }

      if (isAuthExpired(msg)) {
        this.logger.warn("auth expired, transitioning to unauthorized");
        this._setState(ConnectionState.Unauthorized);
        this._stopReconnect();
        for (const cb of this._unauthorizedHandlers) {
          try {
            cb();
          } catch {
            // ignore callback errors
          }
        }
        return;
      }

      this.logger.debug("received", msg.type);
      const eventHandlers = this.handlers.get(msg.type);
      if (eventHandlers) {
        for (const handler of eventHandlers) {
          handler(msg.payload);
        }
      }
      for (const handler of this.anyHandlers) {
        handler(msg);
      }
    };

    this.ws.onclose = () => {
      if (this._connectionState === ConnectionState.Unauthorized) {
        return;
      }
      if (this._connectionState === ConnectionState.Connecting) {
        this._setState(ConnectionState.Reconnecting);
      }
      this._scheduleReconnect();
    };

    this.ws.onerror = () => {
      // Suppress — onclose handles reconnect; errors during StrictMode
      // double-fire are expected in dev and harmless.
    };
  }

  private _stopReconnect() {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }

  private _scheduleReconnect() {
    this._stopReconnect();
    if (this.reconnectAttempt >= this.maxReconnectAttempts) {
      this.logger.error(`reached max reconnection attempts (${this.maxReconnectAttempts}), giving up`);
      this._setState(ConnectionState.Failed);
      return;
    }

    const backoffMs = this.baseReconnectDelay * 2 ** this.reconnectAttempt;
    const cappedMs = Math.min(backoffMs, this.maxReconnectDelay);
    const jitter = 0.2;
    const jitterRange = cappedMs * jitter;
    const delay = cappedMs + (Math.random() * 2 - 1) * jitterRange;
    const multiplier = this._visibilityHidden ? 3 : 1;
    const finalDelay = delay * multiplier;

    this.reconnectAttempt++;
    this.logger.warn(`disconnected, reconnecting in ${Math.round(finalDelay)}ms (attempt ${this.reconnectAttempt}, hidden=${this._visibilityHidden})`);
    this.reconnectTimer = setTimeout(() => this.connect(), finalDelay);
  }

  disconnect() {
    this._stopReconnect();
    if (this.ws) {
      this.ws.onclose = null;
      this.ws.onerror = null;
      this.ws.close();
      this.ws = null;
    }
    this._setState(ConnectionState.Idle);
    this.hasConnectedBefore = false;
    this.reconnectAttempt = 0;
    this.handlers.clear();
    this.anyHandlers.clear();
    this.onReconnectCallbacks.clear();
  }

  on(event: WSEventType, handler: EventHandler) {
    if (!this.handlers.has(event)) {
      this.handlers.set(event, new Set());
    }
    this.handlers.get(event)!.add(handler);
    return () => {
      this.handlers.get(event)?.delete(handler);
    };
  }

  onAny(handler: (msg: WSMessage) => void) {
    this.anyHandlers.add(handler);
    return () => {
      this.anyHandlers.delete(handler);
    };
  }

  onReconnect(callback: () => void) {
    this.onReconnectCallbacks.add(callback);
    return () => {
      this.onReconnectCallbacks.delete(callback);
    };
  }

  send(message: WSMessage) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message));
    }
  }
}
