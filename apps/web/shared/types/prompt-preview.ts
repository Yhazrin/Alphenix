export interface PromptSection {
  name: string;
  phase: string;
  content: string;
  order: number;
}

export interface PromptPreviewResponse {
  full_prompt: string;
  sections: PromptSection[];
  agent_id: string;
  agent_name: string;
}

export interface TaskContextPreviewResponse {
  full_prompt: string;
  sections: PromptSection[];
  agent_id: string;
  agent_name: string;
  task_id: string;
  issue_id: string;
  issue_title: string;
}
