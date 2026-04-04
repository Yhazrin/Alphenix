"use client";

import { use } from "react";
import dynamic from "next/dynamic";
import { Skeleton } from "@/components/ui/skeleton";

const IssueDetail = dynamic(
  () =>
    import("@/features/issues/components").then((m) => ({
      default: m.IssueDetail,
    })),
  {
    ssr: false,
    loading: () => (
      <div className="flex flex-1 items-center justify-center p-8">
        <div className="w-full max-w-2xl space-y-4">
          <Skeleton className="h-8 w-3/4" />
          <Skeleton className="h-4 w-1/2" />
          <Skeleton className="h-48 w-full" />
        </div>
      </div>
    ),
  },
);

export default function IssueDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  return <IssueDetail issueId={id} />;
}
