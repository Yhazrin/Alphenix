import type { Issue, IssueStatus, IssuePriority, IssueAssigneeType } from "./issue";
import type { Channel, ChannelParticipant } from "./channel";
import type { MemberRole } from "./workspace";

// Issue API
export interface CreateIssueRequest {
  title: string;
  description?: string;
  status?: IssueStatus;
  priority?: IssuePriority;
  assignee_type?: IssueAssigneeType;
  assignee_id?: string;
  parent_issue_id?: string;
  repo_id?: string;
  due_date?: string;
  channel_id?: string;
}

export interface UpdateIssueRequest {
  title?: string;
  description?: string;
  status?: IssueStatus;
  priority?: IssuePriority;
  assignee_type?: IssueAssigneeType | null;
  assignee_id?: string | null;
  position?: number;
  repo_id?: string | null;
  due_date?: string | null;
  channel_id?: string;
}

export interface ListIssuesParams {
  limit?: number;
  offset?: number;
  cursor?: string;
  workspace_id?: string;
  status?: IssueStatus;
  priority?: IssuePriority;
  assignee_id?: string;
  open_only?: boolean;
  channel_id?: string;
}

export interface ListIssuesResponse {
  issues: Issue[];
  total: number;
  next_cursor?: string;
  doneTotal?: number;
}

export interface UpdateMeRequest {
  name?: string;
  avatar_url?: string;
}

export interface CreateMemberRequest {
  email: string;
  role?: MemberRole;
}

export interface UpdateMemberRequest {
  role: MemberRole;
}

// Personal Access Tokens
export interface PersonalAccessToken {
  id: string;
  name: string;
  token_prefix: string;
  expires_at: string | null;
  last_used_at: string | null;
  created_at: string;
}

export interface CreatePersonalAccessTokenRequest {
  name: string;
  expires_in_days?: number;
}

export interface CreatePersonalAccessTokenResponse extends PersonalAccessToken {
  token: string;
}

// Channels (workspace projects)
export interface CreateChannelRequest {
  name: string;
  slug?: string;
  description?: string;
}

export interface ListChannelsResponse {
  channels: Channel[];
}

export interface ListChannelParticipantsResponse {
  participants: ChannelParticipant[];
}

export interface AddChannelParticipantRequest {
  participant_type: "member" | "agent" | "team";
  participant_id: string;
}

// Pagination
export interface PaginationParams {
  limit?: number;
  offset?: number;
}
