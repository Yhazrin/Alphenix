"use client";

import { useState } from "react";
import { ThumbsUp, ThumbsDown, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api } from "@/shared/api";
import { toast } from "sonner";
import { CollapsibleSection } from "./collapsible-section";

interface ReviewSectionProps {
  taskId: string;
}

export function ReviewSection({ taskId }: ReviewSectionProps) {
  const [reviewVerdict, setReviewVerdict] = useState<"pass" | "fail" | "retry">("pass");
  const [reviewFeedback, setReviewFeedback] = useState("");
  const [showReview, setShowReview] = useState(false);

  const handleSubmitReview = async () => {
    if (!taskId) return;
    try {
      await api.submitReview(taskId, { verdict: reviewVerdict, feedback: reviewFeedback.trim() || undefined });
      setReviewFeedback("");
      setShowReview(false);
      toast.success(`Review submitted: ${reviewVerdict}`);
    } catch {
      toast.error("Failed to submit review");
    }
  };

  return (
    <CollapsibleSection
      title="Review"
      icon={<ThumbsUp className="h-3.5 w-3.5 text-muted-foreground" aria-hidden="true" />}
      defaultOpen={false}
    >
      {showReview ? (
        <div className="space-y-2">
          <div className="flex gap-1.5">
            <Button
              size="sm"
              variant={reviewVerdict === "pass" ? "default" : "outline"}
              className="h-7 text-xs flex-1"
              onClick={() => setReviewVerdict("pass")}
            >
              <ThumbsUp className="h-3 w-3 mr-1" aria-hidden="true" /> Pass
            </Button>
            <Button
              size="sm"
              variant={reviewVerdict === "fail" ? "destructive" : "outline"}
              className="h-7 text-xs flex-1"
              onClick={() => setReviewVerdict("fail")}
            >
              <ThumbsDown className="h-3 w-3 mr-1" aria-hidden="true" /> Fail
            </Button>
            <Button
              size="sm"
              variant={reviewVerdict === "retry" ? "secondary" : "outline"}
              className="h-7 text-xs flex-1"
              onClick={() => setReviewVerdict("retry")}
            >
              <RotateCcw className="h-3 w-3 mr-1" aria-hidden="true" /> Retry
            </Button>
          </div>
          <Input
            value={reviewFeedback}
            onChange={(e) => setReviewFeedback(e.target.value)}
            placeholder="Feedback (optional)..."
            className="h-8 text-xs"
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                handleSubmitReview();
              }
              if (e.key === "Escape") {
                setShowReview(false);
                setReviewFeedback("");
              }
            }}
            autoFocus
          />
          <div className="flex gap-1.5">
            <Button size="sm" className="h-7 text-xs flex-1" onClick={handleSubmitReview}>
              Submit review
            </Button>
            <Button
              size="sm"
              variant="ghost"
              className="h-7 text-xs"
              onClick={() => {
                setShowReview(false);
                setReviewFeedback("");
              }}
            >
              Cancel
            </Button>
          </div>
        </div>
      ) : (
        <Button
          variant="ghost"
          size="sm"
          className="h-6 text-xs text-muted-foreground w-full"
          onClick={() => setShowReview(true)}
        >
          <ThumbsUp className="h-3 w-3 mr-1" aria-hidden="true" />
          Submit review
        </Button>
      )}
    </CollapsibleSection>
  );
}
