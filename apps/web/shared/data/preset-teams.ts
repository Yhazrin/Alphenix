export interface PresetTeam {
  id: string;
  name: string;
  description: string;
  icon: string;
  presetAgentSlugs: string[]; // references to PresetAgent.id
  leadSlug: string; // which agent is the team lead
}

export const PRESET_TEAMS: PresetTeam[] = [
  {
    id: "fullstack-dev",
    name: "全栈开发团队",
    description: "前后端 + DevOps 全栈开发组合，适合从零搭建产品",
    icon: "Code2",
    presetAgentSlugs: [
      "engineering-frontend-developer",
      "engineering-backend-architect",
      "engineering-devops-automator",
      "engineering-database-optimizer",
      "engineering-code-reviewer",
    ],
    leadSlug: "engineering-backend-architect",
  },
  {
    id: "product-launch",
    name: "产品上线团队",
    description: "从调研到上线的完整产品交付组合",
    icon: "Rocket",
    presetAgentSlugs: [
      "product-trend-researcher",
      "design-ui-designer",
      "design-ux-researcher",
      "engineering-frontend-developer",
      "engineering-backend-architect",
      "testing-api-tester",
      "testing-reality-checker",
    ],
    leadSlug: "product-trend-researcher",
  },
  {
    id: "qa-team",
    name: "质量保障团队",
    description: "全方位质量保障——功能、性能、安全、无障碍",
    icon: "ShieldCheck",
    presetAgentSlugs: [
      "testing-api-tester",
      "testing-performance-benchmarker",
      "testing-accessibility-auditor",
      "testing-evidence-collector",
      "testing-reality-checker",
      "testing-test-results-analyzer",
    ],
    leadSlug: "testing-api-tester",
  },
  {
    id: "marketing-growth",
    name: "增长营销团队",
    description: "内容 + SEO + 社交媒体全渠道增长组合",
    icon: "TrendingUp",
    presetAgentSlugs: [
      "marketing-content-creator",
      "marketing-seo-specialist",
      "marketing-social-media-strategist",
      "marketing-growth-hacker",
      "marketing-douyin-strategist",
    ],
    leadSlug: "marketing-social-media-strategist",
  },
  {
    id: "design-team",
    name: "设计团队",
    description: "品牌 + UI + UX + 视觉的完整设计组合",
    icon: "Palette",
    presetAgentSlugs: [
      "design-brand-guardian",
      "design-ui-designer",
      "design-ux-architect",
      "design-ux-researcher",
      "design-visual-storyteller",
      "design-whimsy-injector",
    ],
    leadSlug: "design-ui-designer",
  },
  {
    id: "sales-team",
    name: "销售团队",
    description: "从获客到赢单的完整销售组合",
    icon: "Target",
    presetAgentSlugs: [
      "sales-outbound-strategist",
      "sales-discovery-coach",
      "sales-deal-strategist",
      "sales-pipeline-analyst",
      "sales-proposal-strategist",
    ],
    leadSlug: "sales-deal-strategist",
  },
  {
    id: "mobile-team",
    name: "移动开发团队",
    description: "iOS/Android/小程序移动端开发组合",
    icon: "Smartphone",
    presetAgentSlugs: [
      "engineering-mobile-app-builder",
      "engineering-frontend-developer",
      "engineering-backend-architect",
      "engineering-wechat-mini-program-developer",
    ],
    leadSlug: "engineering-mobile-app-builder",
  },
  {
    id: "security-team",
    name: "安全团队",
    description: "安全工程 + 威胁检测 + 合规审计",
    icon: "Lock",
    presetAgentSlugs: [
      "engineering-security-engineer",
      "engineering-threat-detection-engineer",
      "compliance-auditor",
    ],
    leadSlug: "engineering-security-engineer",
  },
  {
    id: "project-management",
    name: "项目管理团队",
    description: "PM + 实验追踪 + 工作流优化的管理组合",
    icon: "ClipboardList",
    presetAgentSlugs: [
      "project-manager-senior",
      "project-management-experiment-tracker",
      "project-management-project-shepherd",
      "testing-workflow-optimizer",
    ],
    leadSlug: "project-manager-senior",
  },
  {
    id: "game-dev",
    name: "游戏开发团队",
    description: "游戏设计 + 程序 + 技术美术",
    icon: "Gamepad2",
    presetAgentSlugs: [
      "game-designer",
      "technical-artist",
      "game-audio-engineer",
      "narrative-designer",
      "level-designer",
    ],
    leadSlug: "game-designer",
  },
  {
    id: "data-ai",
    name: "数据与 AI 团队",
    description: "数据分析 + AI 工程 + 数据工程的智能组合",
    icon: "Brain",
    presetAgentSlugs: [
      "support-analytics-reporter",
      "engineering-ai-engineer",
      "engineering-data-engineer",
      "specialized-model-qa",
    ],
    leadSlug: "engineering-ai-engineer",
  },
  {
    id: "paid-media",
    name: "付费投放团队",
    description: "PPC + 社交广告 + 追踪归因的投放组合",
    icon: "DollarSign",
    presetAgentSlugs: [
      "paid-media-ppc-strategist",
      "paid-media-paid-social-strategist",
      "paid-media-tracking-specialist",
      "paid-media-creative-strategist",
    ],
    leadSlug: "paid-media-ppc-strategist",
  },
  {
    id: "china-marketing",
    name: "中国市场营销团队",
    description: "抖音 + 小红书 + 微信 + 微博的中国社交媒体矩阵",
    icon: "Globe",
    presetAgentSlugs: [
      "marketing-douyin-strategist",
      "marketing-xiaohongshu-specialist",
      "marketing-wechat-operator",
      "marketing-weibo-strategist",
      "marketing-bilibili-strategist",
    ],
    leadSlug: "marketing-douyin-strategist",
  },
  {
    id: "incident-response",
    name: "应急响应团队",
    description: "故障响应 + SRE + 安全的事件处理组合",
    icon: "Siren",
    presetAgentSlugs: [
      "engineering-incident-response-commander",
      "engineering-sre",
      "engineering-security-engineer",
      "support-infrastructure-maintainer",
    ],
    leadSlug: "engineering-incident-response-commander",
  },
];

export function getPresetTeamById(id: string): PresetTeam | undefined {
  return PRESET_TEAMS.find(t => t.id === id);
}

export function searchPresetTeams(query: string): PresetTeam[] {
  const q = query.toLowerCase();
  return PRESET_TEAMS.filter(
    t => t.name.toLowerCase().includes(q) || t.description.toLowerCase().includes(q)
  );
}
