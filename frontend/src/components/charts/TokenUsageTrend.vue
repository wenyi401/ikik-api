<template>
  <UiSection :title="t('admin.dashboard.tokenUsageTrend')">
    <div v-if="loading" class="flex h-48 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div
      v-else-if="trendData.length > 0 && chartData"
      class="token-trend-chart"
      :class="{ 'token-trend-chart--large': size === 'large' }"
    >
      <Line :data="chartData" :options="lineOptions" />
    </div>
    <div
      v-else
      class="flex h-48 items-center justify-center text-sm text-[var(--app-muted)]"
    >
      {{ t('admin.dashboard.noDataAvailable') }}
    </div>
  </UiSection>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { UiSection } from '@/ui'
import { useDarkMode } from '@/composables/useDarkMode'
import { DASHBOARD_TREND_COLORS } from '@/utils/chartPalette'
import type { TrendDataPoint } from '@/types'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
)

const { t } = useI18n()

const props = defineProps<{
  trendData: TrendDataPoint[]
  loading?: boolean
  size?: 'default' | 'large'
}>()

const isDarkMode = useDarkMode()

const chartColors = computed(() => ({
  text: isDarkMode.value ? '#b4b4b4' : '#676767',
  grid: isDarkMode.value ? '#343434' : '#ececec',
  ...DASHBOARD_TREND_COLORS
}))

const chartData = computed(() => {
  if (!props.trendData?.length) return null

  return {
    labels: props.trendData.map((d) => formatChartDate(d.date)),
    datasets: [
      {
        label: t('dashboard.input'),
        data: props.trendData.map((d) => d.input_tokens),
        borderColor: chartColors.value.input,
        backgroundColor: `${chartColors.value.input}20`,
        fill: true,
        tension: 0.28,
        borderWidth: 2,
        pointRadius: 0,
        pointHoverRadius: 3
      },
      {
        label: t('dashboard.output'),
        data: props.trendData.map((d) => d.output_tokens),
        borderColor: chartColors.value.output,
        backgroundColor: `${chartColors.value.output}20`,
        fill: false,
        tension: 0.28,
        borderWidth: 2,
        pointRadius: 0,
        pointHoverRadius: 3
      },
      {
        label: t('keyUsage.cacheCreationTokens'),
        data: props.trendData.map((d) => d.cache_creation_tokens),
        borderColor: chartColors.value.cacheCreation,
        backgroundColor: 'transparent',
        fill: false,
        tension: 0.28,
        borderWidth: 1.5,
        pointRadius: 0,
        hidden: true
      },
      {
        label: t('keyUsage.cacheReadTokens'),
        data: props.trendData.map((d) => d.cache_read_tokens),
        borderColor: chartColors.value.cacheRead,
        backgroundColor: 'transparent',
        fill: false,
        tension: 0.28,
        borderWidth: 1.5,
        pointRadius: 0,
        hidden: true
      },
      {
        label: `${t('dashboard.cache')} %`,
        data: props.trendData.map((d) => {
          const totalPromptTokens = d.input_tokens + d.cache_read_tokens + d.cache_creation_tokens
          return totalPromptTokens > 0 ? (d.cache_read_tokens / totalPromptTokens) * 100 : 0
        }),
        borderColor: chartColors.value.cacheHitRate,
        backgroundColor: `${chartColors.value.cacheHitRate}20`,
        borderDash: [5, 5],
        fill: false,
        tension: 0.28,
        borderWidth: 1.5,
        pointRadius: 0,
        hidden: true,
        yAxisID: 'yPercent'
      }
    ]
  }
})

const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  plugins: {
    legend: {
      position: 'top' as const,
      align: 'start' as const,
      labels: {
        color: chartColors.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        padding: 18,
        font: {
          size: 11
        }
      }
    },
    tooltip: {
      callbacks: {
        label: (context: any) => {
          if (context.dataset.yAxisID === 'yPercent') {
            return `${context.dataset.label}: ${context.raw.toFixed(1)}%`
          }
          return `${context.dataset.label}: ${formatTokens(context.raw)}`
        },
        footer: (tooltipItems: any) => {
          const dataIndex = tooltipItems[0]?.dataIndex
          if (dataIndex !== undefined && props.trendData[dataIndex]) {
            const data = props.trendData[dataIndex]
            return `${t('dashboard.actual')}: $${formatCost(data.actual_cost)} · ${t('dashboard.standard')}: $${formatCost(data.cost)}`
          }
          return ''
        }
      }
    }
  },
  scales: {
    x: {
      grid: {
        display: false
      },
      ticks: {
        color: chartColors.value.text,
        maxRotation: 0,
        autoSkip: true,
        maxTicksLimit: 8,
        font: {
          size: 10
        }
      }
    },
    y: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        },
        callback: (value: string | number) => formatTokens(Number(value))
      }
    },
    yPercent: {
      position: 'right' as const,
      min: 0,
      max: 100,
      grid: {
        drawOnChartArea: false
      },
      ticks: {
        color: chartColors.value.cacheHitRate,
        font: {
          size: 10
        },
        callback: (value: string | number) => `${value}%`
      }
    }
  }
}))

const formatTokens = (value: number): string => {
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  } else if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  } else if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}K`
  }
  return value.toLocaleString()
}

const formatChartDate = (value: string): string => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value

  const hasTime = /T\d{2}:\d{2}/.test(value)
  return new Intl.DateTimeFormat(undefined, hasTime
    ? { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }
    : { month: '2-digit', day: '2-digit' }
  ).format(date)
}

const formatCost = (value: number): string => {
  if (value >= 1000) {
    return (value / 1000).toFixed(2) + 'K'
  } else if (value >= 1) {
    return value.toFixed(2)
  } else if (value >= 0.01) {
    return value.toFixed(3)
  }
  return value.toFixed(4)
}
</script>

<style scoped>
.token-trend-chart {
  height: 12rem;
}

.token-trend-chart--large {
  height: 17rem;
}

@media (max-width: 640px) {
  .token-trend-chart--large {
    height: 15rem;
  }
}
</style>
