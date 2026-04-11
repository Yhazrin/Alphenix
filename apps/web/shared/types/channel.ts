export interface Channel {
  id: string;
  workspace_id: string;
  name: string;
  slug: string;
  description: string | null;
  is_default: boolean;
  created_at: string;
  updated_at: string;
}

export interface ChannelParticipant {
  participant_type: "member" | "agent" | "team";
  participant_id: string;
  created_at: string;
}
