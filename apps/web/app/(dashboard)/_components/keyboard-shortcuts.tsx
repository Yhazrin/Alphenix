"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useModalStore } from "@/features/modals";

export function KeyboardShortcuts() {
  const router = useRouter();

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      // Cmd/Ctrl+K — open search (works even in inputs)
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        useModalStore.getState().open("search");
        return;
      }

      // Don't trigger when typing in inputs/textareas
      const target = e.target as HTMLElement;
      if (target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.isContentEditable) return;
      // Don't trigger with modifier keys
      if (e.metaKey || e.ctrlKey || e.altKey) return;

      switch (e.key) {
        case "c":
          e.preventDefault();
          useModalStore.getState().open("create-issue");
          break;
        case "/":
          e.preventDefault();
          useModalStore.getState().open("search");
          break;
        case "1":
          e.preventDefault();
          router.push("/inbox");
          break;
        case "2":
          e.preventDefault();
          router.push("/my-issues");
          break;
        case "3":
          e.preventDefault();
          router.push("/issues");
          break;
      }
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [router]);

  return null;
}
