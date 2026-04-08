"use client";

import { useEffect, useState } from "react";
import type { WSEventType } from "@/shared/types";
import { useWS } from "./provider";
import { ConnectionState } from "@/shared/api/ws-client";

type EventHandler = (payload: unknown, actorId?: string) => void;

/**
 * Hook that subscribes to a WebSocket event and calls the handler.
 * Automatically unsubscribes on cleanup.
 */
export function useWSEvent(event: WSEventType, handler: EventHandler) {
  const { subscribe } = useWS();

  useEffect(() => {
    const unsub = subscribe(event, handler);
    return unsub;
  }, [event, handler, subscribe]);
}

/**
 * Hook that registers a callback to run on WebSocket reconnection.
 * Useful for refetching component-local data after a network interruption.
 */
export function useWSReconnect(callback: () => void) {
  const { onReconnect } = useWS();

  useEffect(() => {
    const unsub = onReconnect(callback);
    return unsub;
  }, [callback, onReconnect]);
}

/**
 * Hook that returns the current WebSocket connection state.
 * Re-renders when the state changes.
 */
export function useConnectionState(): ConnectionState {
  const { client } = useWS();
  const [state, setState] = useState<ConnectionState>(
    client?.connectionState ?? ConnectionState.Idle,
  );

  useEffect(() => {
    if (!client) {
      setState(ConnectionState.Idle);
      return;
    }
    setState(client.connectionState);
    return client.onConnectionStateChange((next) => setState(next));
  }, [client]);

  return state;
}
