"use client";

import { use } from "react";
import dynamic from "next/dynamic";
import { Skeleton } from "@/components/ui/skeleton";

const TaskRunDetail = dynamic(
  () =>
    import("@/features/issues/components/task-run-detail").then((m) => ({
      default: m.TaskRunDetail,
    })),
  {
    ssr: false,
    loading: () => (
      <div className="flex flex-1 items-center justify-center p-8">
        <div className="w-full max-w-3xl space-y-4">
          <Skeleton className="h-8 w-1/2" />
          <Skeleton className="h-4 w-1/3" />
          <Skeleton className="h-64 w-full" />
        </div>
      </div>
    ),
  },
);

export default function TaskDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  return <TaskRunDetail taskId={id} />;
}
