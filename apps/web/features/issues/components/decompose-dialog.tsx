"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { Sparkles, Loader2, AlertCircle } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
  DialogClose,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { StatusIcon } from "./status-icon";
import { api } from "@/shared/api";
import { toast } from "sonner";
import type { Issue, DecomposePreview } from "@/shared/types";

interface DecomposeDialogProps {
  issueId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onComplete: (issues: Issue[]) => void;
}

type Phase = "loading" | "preview" | "confirming" | "error";

export function DecomposeDialog({ issueId, open, onOpenChange, onComplete }: DecomposeDialogProps) {
  const [phase, setPhase] = useState<Phase>("loading");
  const [preview, setPreview] = useState<DecomposePreview | null>(null);
  const [error, setError] = useState<string | null>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  const startDecompose = useCallback(async () => {
    setPhase("loading");
    setPreview(null);
    setError(null);

    try {
      const resp = await api.decomposeIssue(issueId);

      if (resp.status === "completed" && resp.preview) {
        setPreview(resp.preview);
        setPhase("preview");
        return;
      }

      if (resp.status === "failed") {
        setError(resp.error ?? "Decomposition failed");
        setPhase("error");
        return;
      }

      // Poll for result.
      pollRef.current = setInterval(async () => {
        try {
          const result = await api.getDecomposeResult(issueId, resp.run_id);
          if (result.status === "completed" && result.preview) {
            stopPolling();
            setPreview(result.preview);
            setPhase("preview");
          } else if (result.status === "failed") {
            stopPolling();
            setError(result.error ?? "Decomposition failed");
            setPhase("error");
          }
        } catch {
          // Keep polling on transient errors.
        }
      }, 2000);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to start decomposition");
      setPhase("error");
    }
  }, [issueId, stopPolling]);

  // Start decomposition when dialog opens.
  useEffect(() => {
    if (open) {
      startDecompose();
    }
    return () => stopPolling();
  }, [open, startDecompose, stopPolling]);

  const handleConfirm = async () => {
    if (!preview) return;
    setPhase("confirming");

    try {
      const result = await api.confirmDecompose(issueId, { subtasks: preview.subtasks });
      toast.success(`Created ${result.total} sub-issues`);
      onComplete(result.issues);
      onOpenChange(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to create sub-issues");
      setPhase("error");
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg max-h-[80vh] flex flex-col">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Sparkles className="h-4 w-4" aria-hidden="true" />
            Decompose Goal
          </DialogTitle>
          <DialogDescription>
            The Architect Agent will break this goal into executable sub-tasks.
          </DialogDescription>
        </DialogHeader>

        <div className="flex-1 overflow-y-auto min-h-0">
          {phase === "loading" && (
            <div className="flex flex-col items-center justify-center gap-3 py-8">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" aria-hidden="true" />
              <p className="text-sm text-muted-foreground">Analyzing goal and planning sub-tasks...</p>
              <div className="w-full space-y-2 mt-4">
                <Skeleton className="h-8 w-full" />
                <Skeleton className="h-8 w-5/6" />
                <Skeleton className="h-8 w-4/6" />
              </div>
            </div>
          )}

          {phase === "error" && (
            <div className="flex flex-col items-center justify-center gap-2 py-8 text-destructive">
              <AlertCircle className="h-6 w-6" aria-hidden="true" />
              <p className="text-sm">{error}</p>
            </div>
          )}

          {(phase === "preview" || phase === "confirming") && preview && (
            <div className="space-y-4">
              {preview.plan_summary && (
                <p className="text-xs text-muted-foreground leading-relaxed">{preview.plan_summary}</p>
              )}

              <div className="space-y-2">
                {preview.subtasks.map((st, _idx) => (
                  <div key={st.title} className="rounded-lg border p-3 space-y-1.5">
                    <div className="flex items-start gap-2">
                      <StatusIcon status="todo" className="h-3.5 w-3.5 mt-0.5 shrink-0" />
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium">{st.title}</p>
                        <p className="text-xs text-muted-foreground mt-0.5">{st.description}</p>
                      </div>
                      {st.assignee_type === "agent" && (
                        <Badge variant="secondary" className="text-[10px] shrink-0">agent</Badge>
                      )}
                    </div>
                    {st.deliverable && (
                      <p className="text-[11px] text-muted-foreground pl-5">
                        Deliverable: {st.deliverable}
                      </p>
                    )}
                    {st.depends_on.length > 0 && (
                      <div className="flex items-center gap-1 pl-5">
                        {st.depends_on.map((depIdx) => (
                          <Badge key={depIdx} variant="outline" className="text-[10px] px-1.5 py-0">
                            blocked by #{depIdx + 1}
                          </Badge>
                        ))}
                      </div>
                    )}
                  </div>
                ))}
              </div>

              {preview.risks.length > 0 && (
                <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-3">
                  <p className="text-xs font-medium text-destructive mb-1">Risks</p>
                  <ul className="space-y-0.5">
                    {preview.risks.map((risk, idx) => (
                      <li key={`risk-${idx}-${risk}`} className="text-xs text-muted-foreground">- {risk}</li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
          )}
        </div>

        <DialogFooter>
          {phase === "preview" && (
            <>
              <DialogClose render={<Button variant="ghost" size="sm" />}>
                Cancel
              </DialogClose>
              <Button size="sm" onClick={handleConfirm}>
                Create {preview?.subtasks?.length ?? 0} Sub-Issues
              </Button>
            </>
          )}
          {phase === "error" && (
            <>
              <DialogClose render={<Button variant="ghost" size="sm" />}>
                Close
              </DialogClose>
              <Button size="sm" variant="outline" onClick={startDecompose}>
                Retry
              </Button>
            </>
          )}
          {phase === "confirming" && (
            <Button size="sm" disabled>
              <Loader2 className="mr-1.5 h-3 w-3 animate-spin" aria-hidden="true" />
              Creating...
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
