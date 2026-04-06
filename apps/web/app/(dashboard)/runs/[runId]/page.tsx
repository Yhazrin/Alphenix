"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import type { Run, RunStep, RunTodo, RunArtifact } from "@/shared/types";
import { RunHeader } from "@/features/runs/components/run-header";
import { StepStream } from "@/features/runs/components/step-stream";
import { TodoPanel } from "@/features/runs/components/todo-panel";
import { ArtifactDrawer } from "@/features/runs/components/artifact-drawer";
import { ArtifactsBar } from "@/features/runs/components/artifacts-bar";
import { mockRun, mockSteps, mockTodos, mockArtifacts } from "@/shared/mocks/timeline";
import { ErrorBoundary } from "@/components/error-boundary";

export default function RunDetailPage() {
  const params = useParams();
  const runId = params.runId as string;

  // Mock state — will be replaced with useRunSubscription hook when #22 WS is ready
  const [run] = useState<Run>(mockRun);
  const [steps] = useState<RunStep[]>(mockSteps);
  const [todos] = useState<RunTodo[]>(mockTodos);
  const [artifacts] = useState<RunArtifact[]>(mockArtifacts);
  const [selectedArtifact, setSelectedArtifact] = useState<RunArtifact | null>(null);

  return (
    <div className="flex h-full flex-col">
      <ErrorBoundary>
        <RunHeader run={run} steps={steps} />
      </ErrorBoundary>

      <div className="flex flex-1 overflow-hidden">
        {/* Main timeline */}
        <div className="flex-1 overflow-y-auto p-4">
          <ErrorBoundary>
            <StepStream
              steps={steps}
              artifacts={artifacts}
              onArtifactClick={setSelectedArtifact}
            />
          </ErrorBoundary>
        </div>

        {/* Right sidebar — todo progress */}
        <div className="w-72 border-l overflow-y-auto p-4">
          <ErrorBoundary>
            <TodoPanel todos={todos} />
          </ErrorBoundary>
        </div>
      </div>

      {/* Artifact detail drawer */}
      {selectedArtifact && (
        <ArtifactDrawer
          artifact={selectedArtifact}
          onClose={() => setSelectedArtifact(null)}
        />
      )}

      {/* Artifacts bar — shows at bottom when artifacts exist */}
      <ArtifactsBar
        artifacts={artifacts}
        onArtifactClick={setSelectedArtifact}
      />
    </div>
  );
}
