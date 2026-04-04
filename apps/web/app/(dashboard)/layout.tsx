"use client";

import { useEffect } from "react";
import { useRouter, usePathname } from "next/navigation";
import { MulticodeIcon } from "@/components/multicode-icon";
import { useNavigationStore } from "@/features/navigation";
import { SidebarProvider, SidebarInset } from "@/components/ui/sidebar";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "@/features/workspace";
import { ErrorBoundary } from "@/components/error-boundary";
import { AppSidebar } from "./_components/app-sidebar";
import { KeyboardShortcuts } from "./_components/keyboard-shortcuts";
import { AIAssistant } from "@/components/ai-assistant/ai-assistant";

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const user = useAuthStore((s) => s.user);
  const isLoading = useAuthStore((s) => s.isLoading);
  const workspace = useWorkspaceStore((s) => s.workspace);

  useEffect(() => {
    if (!isLoading && !user) {
      router.replace("/");
    }
  }, [user, isLoading, router]);

  useEffect(() => {
    useNavigationStore.getState().onPathChange(pathname);
  }, [pathname]);

  if (isLoading) {
    return (
      <div className="flex h-svh items-center justify-center" role="status" aria-label="Loading">
        <MulticodeIcon className="size-6" aria-hidden="true" />
      </div>
    );
  }

  if (!user) return null;

  return (
    <SidebarProvider className="h-svh">
      <AppSidebar />
      <SidebarInset className="overflow-hidden">
        <ErrorBoundary>
          {workspace ? (
            children
          ) : (
            <div className="flex flex-1 items-center justify-center" role="status" aria-label="Loading workspace">
              <MulticodeIcon className="size-6 animate-pulse" aria-hidden="true" />
            </div>
          )}
        </ErrorBoundary>
      </SidebarInset>
      <AIAssistant />
      <KeyboardShortcuts />
    </SidebarProvider>
  );
}
