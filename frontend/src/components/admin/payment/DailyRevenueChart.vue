<template>
  <section class="border-b border-[var(--ui-border)] pb-6 pt-1">
    <h3 class="mb-4 text-sm font-semibold text-[var(--ui-text)]">
      {{ t('payment.admin.dailyRevenue') }}
    </h3>
    <div class="h-64">
      <div v-if="loading" class="flex h-full items-center justify-center">
        <LoadingSpinner size="md" />
      </div>
      <Line v-else-if="chartData" :data="chartData" :options="chartOptions" />
      <div
        v-else
        class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-gray-400"
      >
        {{ t('payment.admin.noData') }}
      </div>
    </div>
  </section>
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
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { useDarkMode } from '@/composables/useDarkMode'
import type { DailyPaymentStats } from '@/types/payment'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

const { t } = useI18n()
const isDarkMode = useDarkMode()

const props = defineProps<{
  data: DailyPaymentStats[]
  loading?: boolean
}>()

const colors = [
  ['#10a37f', 'rgba(16, 163, 127, 0.1)'],
  ['#418fce', 'rgba(65, 143, 206, 0.1)'],
  ['#d17b2f', 'rgba(209, 123, 47, 0.1)'],
  ['#a15d84', 'rgba(161, 93, 132, 0.1)'],
]

const chartData = computed(() => {
  if (!props.data || props.data.length === 0) return null
  const currencies = [...new Set(props.data.flatMap((day) => Object.keys(day.amount)))].sort()

  return {
    labels: props.data.map(d => d.date),
    datasets: [
      ...currencies.map((currency, index) => {
        const [borderColor, backgroundColor] = colors[index % colors.length]
        return {
          label: `${currency.toUpperCase()} ${t('payment.admin.revenue')}`,
          data: props.data.map(day => day.amount[currency] || 0),
          borderColor,
          backgroundColor,
          fill: true,
          tension: 0.28,
          borderWidth: 2,
          pointRadius: 0,
          pointHoverRadius: 3,
        }
      }),
      {
        label: t('payment.admin.orderCount'),
        data: props.data.map(d => d.count),
        borderColor: isDarkMode.value ? '#19c37d' : '#10a37f',
        backgroundColor: 'transparent',
        fill: false,
        tension: 0.28,
        borderWidth: 2,
        pointRadius: 0,
        pointHoverRadius: 3,
        yAxisID: 'y1',
      }
    ]
  }
})

const chartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index' as const, intersect: false },
  scales: {
    y: {
      type: 'linear' as const,
      display: true,
      position: 'left' as const,
      grid: { color: isDarkMode.value ? '#343434' : '#ececec' },
      ticks: { color: isDarkMode.value ? '#b4b4b4' : '#676767' },
    },
    y1: {
      type: 'linear' as const,
      display: true,
      position: 'right' as const,
      grid: { drawOnChartArea: false },
      ticks: { color: isDarkMode.value ? '#b4b4b4' : '#676767' },
    },
    x: {
      grid: { display: false },
      ticks: { color: isDarkMode.value ? '#b4b4b4' : '#676767', maxRotation: 0 },
    }
  },
  plugins: {
    legend: {
      position: 'top' as const,
      align: 'start' as const,
      labels: {
        color: isDarkMode.value ? '#b4b4b4' : '#676767',
        usePointStyle: true,
        pointStyle: 'circle',
      }
    },
  }
}))
</script>
