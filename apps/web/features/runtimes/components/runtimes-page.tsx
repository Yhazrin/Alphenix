"use client";

import { useEffect, useCallback } from "react";
import { AlertCircle, Server } from "lucide-react";
import { useDefaultLayout } from "react-resizable-panels";
import {
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from "@/components/ui/resizable";
import { Skeleton } from "@/components/ui/skeleton";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "@/features/workspace";
import { useWSEvent } from "@/features/realtime";
import { useRuntimeStore } from "../store";
import { RuntimeList } from "./runtime-list";
import { RuntimeDetail } from "./runtime-detail";

export default function RuntimesPage() {
  const isLoading = useAuthStore((s) => s.isLoading);
  const workspace = useWorkspaceStore((s) => s.workspace);
  const runtimes = useRuntimeStore((s) => s.runtimes);
  const selectedId = useRuntimeStore((s) => s.selectedId);
  const fetching = useRuntimeStore((s) => s.fetching);
  const error = useRuntimeStore((s) => s.error);
  const fetchRuntimes = useRuntimeStore((s) => s.fetchRuntimes);
  const setSelectedId = useRuntimeStore((s) => s.setSelectedId);

  const { defaultLayout, onLayoutChanged } = useDefaultLayout({
    id: "alphenix_runtimes_layout",
  });

  useEffect(() => {
    if (workspace) fetchRuntimes();
  }, [workspace, fetchRuntimes]);

  // Re-fetch on daemon register/deregister events.
  // Heartbeat events are not broadcast over WS, so no handler needed.
  const handleDaemonEvent = useCallback(() => {
    fetchRuntimes();
  }, [fetchRuntimes]);

  useWSEvent("daemon:register", handleDaemonEvent);

  const selected = runtimes.find((r) => r.id === selectedId) ?? null;

  const handleRuntimeUpdate = useCallback((updated: typeof selected) => {
    if (updated) {
      useRuntimeStore.getState().patchRuntime(updated.id, updated);
    }
  }, []);

  if (isLoading || fetching) {
    return (
      <div className="flex flex-1 min-h-0">
        {/* List skeleton */}
        <div className="w-72 border-r">
          <div className="flex h-12 items-center justify-between border-b px-4">
            <Skeleton className="h-4 w-20" />
          </div>
          <div className="divide-y">
            {Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="flex items-center gap-3 px-4 py-3">
                <Skeleton className="h-5 w-5 rounded" />
                <div className="flex-1 space-y-1.5">
                  <Skeleton className="h-4 w-28" />
                  <Skeleton className="h-3 w-20" />
                </div>
              </div>
            ))}
          </div>
        </div>
        {/* Detail skeleton */}
        <div className="flex-1 p-6 space-y-6">
          <div className="flex items-center gap-3">
            <Skeleton className="h-5 w-5 rounded" />
            <Skeleton className="h-5 w-32" />
          </div>
          <div className="space-y-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-16 w-full rounded-lg" />
            ))}
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-1 items-center justify-center">
        <div className="flex flex-col items-center gap-2 text-center">
          <AlertCircle className="h-8 w-8 text-destructive/60" aria-hidden="true" />
          <p className="text-sm font-medium text-destructive">Failed to load runtimes</p>
          <p className="text-xs text-destructive/70">{error}</p>
        </div>
      </div>
    );
  }

  return (
    <ResizablePanelGroup
      orientation="horizontal"
      className="flex-1 min-h-0"
      defaultLayout={defaultLayout}
      onLayoutChanged={onLayoutChanged}
    >
      <ResizablePanel
        id="list"
        defaultSize={280}
        minSize={240}
        maxSize={400}
        groupResizeBehavior="preserve-pixel-size"
      >
        <RuntimeList
          runtimes={runtimes}
          selectedId={selectedId}
          onSelect={setSelectedId}
        />
      </ResizablePanel>

      <ResizableHandle />

      <ResizablePanel id="detail" minSize="50%">
        {selected ? (
          <RuntimeDetail key={selected.id} runtime={selected} onUpdate={handleRuntimeUpdate} />
        ) : (
          <div className="flex h-full flex-col items-center justify-center text-muted-foreground">
            <Server className="h-10 w-10 text-muted-foreground/30" aria-hidden="true" />
            <p className="mt-3 text-sm">Select a runtime to view details</p>
          </div>
        )}
      </ResizablePanel>
    </ResizablePanelGroup>
  );
}
