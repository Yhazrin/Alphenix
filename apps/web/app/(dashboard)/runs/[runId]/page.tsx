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
      <RunHeader run={run} steps={steps} />

      <div className="flex flex-1 overflow-hidden">
        {/* Main timeline */}
        <div className="flex-1 overflow-y-auto p-4">
          <StepStream
            steps={steps}
            artifacts={artifacts}
            onArtifactClick={setSelectedArtifact}
          />
        </div>

        {/* Right sidebar — todo progress */}
        <div className="w-72 border-l overflow-y-auto p-4">
          <TodoPanel todos={todos} />
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
