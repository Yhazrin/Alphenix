"use client";

import type { RunStep, RunArtifact } from "@/shared/types";
import { StepCard } from "./step-card";

interface StepStreamProps {
  steps: RunStep[];
  artifacts: RunArtifact[];
  onArtifactClick: (artifact: RunArtifact) => void;
}

export function StepStream({ steps, artifacts, onArtifactClick }: StepStreamProps) {
  const artifactByStep = new Map<string, RunArtifact>();
  for (const a of artifacts) {
    if (a.step_id) artifactByStep.set(a.step_id, a);
  }

  return (
    <div className="space-y-0.5" data-testid="step-stream">
      {steps.map((step) => (
        <StepCard
          key={step.id}
          step={step}
          artifact={artifactByStep.get(step.id)}
          onArtifactClick={onArtifactClick}
        />
      ))}
    </div>
  );
}
