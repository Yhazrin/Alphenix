"use client";

import { useParams } from "next/navigation";
import { ChannelSettingsPage } from "@/features/channels";

export default function ChannelDetailPage() {
  const params = useParams();
  const channelId = typeof params.channelId === "string" ? params.channelId : "";
  if (!channelId) {
    return null;
  }
  return <ChannelSettingsPage channelId={channelId} />;
}
