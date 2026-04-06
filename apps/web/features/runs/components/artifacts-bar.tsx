"use client";

import type { RunArtifact } from "@/shared/types";
import { FileText, BarChart3, Paperclip } from "lucide-react";

interface ArtifactsBarProps {
  artifacts: RunArtifact[];
  onArtifactClick: (artifact: RunArtifact) => void;
}

function artifactIcon(type: string) {
  switch (type) {
    case "report":
      return <BarChart3 className="h-3.5 w-3.5 text-indigo-500" />;
    case "diff":
      return <FileText className="h-3.5 w-3.5 text-green-500" />;
    default:
      return <Paperclip className="h-3.5 w-3.5 text-muted-foreground" />;
  }
}

export function ArtifactsBar({ artifacts, onArtifactClick }: ArtifactsBarProps) {
  if (artifacts.length === 0) return null;

  return (
    <div className="border-t px-4 py-2 flex items-center gap-2" data-testid="artifacts-bar">
      <span className="text-[10px] font-semibold text-muted-foreground uppercase tracking-wide">
        Artifacts
      </span>
      <div className="flex items-center gap-1.5">
        {artifacts.map((artifact) => (
          <button
            key={artifact.id}
            type="button"
            onClick={() => onArtifactClick(artifact)}
            className="flex items-center gap-1.5 rounded-md border px-2 py-1 text-[11px] hover:bg-muted transition-colors"
          >
            {artifactIcon(artifact.artifact_type)}
            <span className="truncate max-w-[120px]">{artifact.name}</span>
          </button>
        ))}
      </div>
    </div>
  );
}
