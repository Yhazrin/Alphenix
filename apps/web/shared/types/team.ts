export interface Team {
  id: string;
  workspace_id: string;
  name: string;
  description: string | null;
  avatar_url: string | null;
  lead_agent_id: string | null;
  members: TeamMember[];
  created_at: string;
  updated_at: string;
  created_by: string | null;
  archived_at: string | null;
  archived_by: string | null;
}

export interface TeamMember {
  id: string;
  team_id: string;
  agent_id: string;
  agent?: Agent;
  role: "member" | "lead";
  joined_at: string;
}

export interface CreateTeamRequest {
  name: string;
  description?: string;
  avatar_url?: string;
  lead_agent_id?: string;
  member_agent_ids?: string[];
}

export interface UpdateTeamRequest {
  name?: string;
  description?: string;
  avatar_url?: string;
  lead_agent_id?: string;
}

// Import Agent type for nested agent in TeamMember
import type { Agent } from "./agent";
