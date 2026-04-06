"use client";

import { useEffect } from "react";
import { AlertTriangle } from "lucide-react";
import { Button } from "@/components/ui/button";

export default function RunDetailError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("Run detail error:", error);
  }, [error]);

  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-4 p-8">
      <AlertTriangle className="h-8 w-8 text-destructive" />
      <div className="text-center">
        <h2 className="text-sm font-semibold">Failed to load run</h2>
        <p className="text-xs text-muted-foreground mt-1">
          {error.message || "An unexpected error occurred"}
        </p>
      </div>
      <Button variant="outline" size="sm" onClick={reset}>
        Try again
      </Button>
    </div>
  );
}
