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

export interface TaskContextSection {
  key: string;
  title: string;
  source: string;
  content: string;
}

export interface TaskContextPreviewResponse {
  sections: TaskContextSection[];
  final_prompt: string;
  task_id: string;
  agent_id: string;
  agent_name: string;
  issue_id: string;
  issue_title: string;
}
