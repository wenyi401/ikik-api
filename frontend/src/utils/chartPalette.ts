export const DASHBOARD_CHART_PALETTE = [
  '#2563EB',
  '#F43F5E',
  '#F59E0B',
  '#10B981',
  '#8B5CF6',
  '#06B6D4',
  '#EC4899',
  '#84CC16',
  '#F97316',
  '#6366F1',
  '#14B8A6',
  '#EAB308'
] as const

export const chartPaletteFor = (count: number): string[] =>
  Array.from({ length: Math.max(0, count) }, (_, index) =>
    DASHBOARD_CHART_PALETTE[index % DASHBOARD_CHART_PALETTE.length]
  )

export const DASHBOARD_TREND_COLORS = {
  input: DASHBOARD_CHART_PALETTE[0],
  output: DASHBOARD_CHART_PALETTE[1],
  cacheCreation: DASHBOARD_CHART_PALETTE[4],
  cacheRead: DASHBOARD_CHART_PALETTE[5],
  cacheHitRate: DASHBOARD_CHART_PALETTE[2]
} as const
