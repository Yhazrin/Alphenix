"use client";

import { useEffect } from "react";
import { AlertTriangle } from "lucide-react";
import { Button } from "@/components/ui/button";

export default function RootError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("Root error:", error);
  }, [error]);

  return (
    <div className="flex h-screen flex-col items-center justify-center gap-4">
      <AlertTriangle className="h-10 w-10 text-destructive" />
      <div className="text-center">
        <h2 className="text-lg font-semibold">Something went wrong</h2>
        <p className="text-sm text-muted-foreground mt-1">
          {error.message || "An unexpected error occurred"}
        </p>
      </div>
      <Button onClick={reset}>Try again</Button>
    </div>
  );
}
