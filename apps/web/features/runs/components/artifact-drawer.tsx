"use client";

import type { RunArtifact } from "@/shared/types";
import { X } from "lucide-react";

interface ArtifactDrawerProps {
  artifact: RunArtifact;
  onClose: () => void;
}

export function ArtifactDrawer({ artifact, onClose }: ArtifactDrawerProps) {
  return (
    <div className="fixed inset-0 z-50 flex justify-end" data-testid="artifact-drawer">
      {/* Backdrop */}
      <button
        type="button"
        className="absolute inset-0 bg-background/80 backdrop-blur-sm"
        aria-label="Close drawer"
        onClick={onClose}
      />

      {/* Drawer */}
      <div className="relative w-full max-w-lg border-l bg-background shadow-lg flex flex-col">
        <div className="flex items-center justify-between border-b px-4 py-3">
          <div>
            <h2 className="text-sm font-semibold">{artifact.name}</h2>
            <p className="text-[10px] text-muted-foreground">
              {artifact.artifact_type} · {artifact.mime_type}
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded p-1 hover:bg-muted"
            aria-label="Close"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-4">
          <pre className="text-xs whitespace-pre-wrap font-mono">{artifact.content}</pre>
        </div>
      </div>
    </div>
  );
}
