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

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

const { t } = useI18n()
const isDarkMode = useDarkMode()

const props = defineProps<{
  data: { date: string; amount: number; count: number }[]
  loading?: boolean
}>()

const chartData = computed(() => {
  if (!props.data || props.data.length === 0) return null
  return {
    labels: props.data.map(d => d.date),
    datasets: [
      {
        label: t('payment.admin.revenue'),
        data: props.data.map(d => d.amount),
        borderColor: isDarkMode.value ? '#f2f2f2' : '#171717',
        backgroundColor: isDarkMode.value ? 'rgba(242, 242, 242, 0.06)' : 'rgba(23, 23, 23, 0.05)',
        fill: true,
        tension: 0.28,
        borderWidth: 2,
        pointRadius: 0,
        pointHoverRadius: 3,
      },
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
