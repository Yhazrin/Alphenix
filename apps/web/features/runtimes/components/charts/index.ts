import dynamic from "next/dynamic";

export const DailyTokenChart = dynamic(
  () => import("./daily-token-chart").then((m) => m.DailyTokenChart),
  { ssr: false, loading: () => null },
);

export const DailyCostChart = dynamic(
  () => import("./daily-cost-chart").then((m) => m.DailyCostChart),
  { ssr: false, loading: () => null },
);

export const ModelDistributionChart = dynamic(
  () => import("./model-distribution-chart").then(
    (m) => m.ModelDistributionChart,
  ),
  { ssr: false, loading: () => null },
);

export const ActivityHeatmap = dynamic(
  () => import("./activity-heatmap").then((m) => m.ActivityHeatmap),
  { ssr: false, loading: () => null },
);

export const HourlyActivityChart = dynamic(
  () => import("./hourly-activity-chart").then((m) => m.HourlyActivityChart),
  { ssr: false, loading: () => null },
);
