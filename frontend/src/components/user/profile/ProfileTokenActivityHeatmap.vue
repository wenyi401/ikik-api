<template>
  <section
    data-testid="profile-token-activity-heatmap"
    class="min-w-0 overflow-visible rounded-xl border border-[var(--claude-border)] bg-[var(--claude-surface)] p-3 shadow-none sm:p-4 md:p-5"
  >
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div class="min-w-0">
        <h3 class="text-sm font-semibold leading-snug text-[var(--claude-text)] md:text-base">
          {{ t('profile.activity.githubTitle', { total: formatCompactNumber(summary.totalTokens) }) }}
        </h3>
        <p class="mt-1 text-xs text-[var(--claude-muted)]">
          {{ t('profile.activity.mobileMeta', {
            total: formatCompactNumber(summary.totalTokens),
            streak: summary.currentStreak
          }) }}
        </p>
      </div>
      <div class="grid w-full shrink-0 grid-cols-3 items-center gap-1 rounded-md border border-[var(--claude-border)] bg-[var(--claude-surface-muted)] p-1 text-xs text-[var(--claude-muted)] sm:inline-flex sm:w-auto sm:self-start">
        <button
          v-for="mode in modes"
          :key="mode.key"
          type="button"
          :class="[
            'inline-flex justify-center whitespace-nowrap rounded px-2 py-1 transition',
            activityMode === mode.key
              ? '!bg-[var(--claude-text)] font-semibold !text-[var(--claude-bg)] shadow-sm'
              : 'text-[var(--claude-muted)] hover:bg-white/70 hover:text-[var(--claude-text)] dark:hover:bg-white/10'
          ]"
          @click="activityMode = mode.key"
        >
          {{ mode.label }}
        </button>
      </div>
    </div>

    <div class="relative -mx-1 mt-3 min-w-0 md:mt-4">
      <div class="pointer-events-none absolute inset-y-0 left-0 z-10 w-5 bg-gradient-to-r from-white/90 to-transparent dark:from-[var(--claude-surface)]" />
      <div class="pointer-events-none absolute inset-y-0 right-0 z-10 w-5 bg-gradient-to-l from-white/90 to-transparent dark:from-[var(--claude-surface)]" />
      <div
        ref="heatmapScroller"
        :class="[
          'min-w-0 overflow-x-auto overflow-y-visible overscroll-x-contain px-1 pb-2 [-webkit-overflow-scrolling:touch] [scrollbar-width:thin]',
          selectedCellKey ? 'pt-11' : 'pt-2'
        ]"
      >
        <div class="min-w-[620px] md:min-w-[760px]">
          <div class="mb-2 grid grid-cols-[22px_1fr] gap-1 md:grid-cols-[28px_1fr] md:gap-2">
            <div />
            <div class="relative h-4">
              <span
                v-for="month in monthLabels"
                :key="`${month.label}-${month.index}`"
                class="absolute top-0 text-[10px] font-medium text-[var(--claude-muted)] md:text-[11px]"
                :style="{ left: `${month.left}%` }"
              >
                {{ month.label }}
              </span>
            </div>
          </div>

          <div class="grid grid-cols-[22px_1fr] gap-1 md:grid-cols-[28px_1fr] md:gap-2">
            <div class="grid grid-rows-7 gap-[3px] text-[9px] leading-[0.55rem] text-[var(--claude-muted)] md:gap-1 md:text-[10px] md:leading-3">
              <span />
              <span>{{ t('profile.activity.weekdays.mon') }}</span>
              <span />
              <span>{{ t('profile.activity.weekdays.wed') }}</span>
              <span />
              <span>{{ t('profile.activity.weekdays.fri') }}</span>
              <span />
            </div>

            <div
              class="relative grid grid-flow-col grid-rows-7 gap-[3px] overflow-visible [--profile-heatmap-cell:0.55rem] md:gap-1 md:[--profile-heatmap-cell:0.75rem]"
              :style="{ gridTemplateColumns: `repeat(${displayWeeks.length}, var(--profile-heatmap-cell))` }"
            >
              <button
                v-for="cell in displayCells"
                :key="`${activityMode}-${cell.date}`"
                type="button"
                class="relative h-[var(--profile-heatmap-cell)] w-[var(--profile-heatmap-cell)] rounded-[2px] ring-1 ring-black/[0.03] transition-transform hover:scale-125 hover:ring-[#216e39] focus:outline-none focus:ring-2 focus:ring-[#216e39] dark:ring-white/[0.04] md:rounded-[3px]"
                :style="{ backgroundColor: levelColor(cell.level) }"
                :title="cellTitle(cell)"
                :aria-label="cellTitle(cell)"
                @click.stop="toggleSelectedCell(cell)"
                @keydown.esc.stop="selectedCellKey = ''"
                >
                <span
                  v-if="selectedCellKey === cellKey(cell)"
                  class="pointer-events-none absolute left-1/2 top-[-3.45rem] z-30 min-w-28 -translate-x-1/2 whitespace-nowrap rounded-lg bg-[var(--claude-text)] px-2.5 py-1.5 text-center text-[var(--claude-bg)] shadow-lg ring-1 ring-black/10"
                >
                  <span class="block text-[11px] font-semibold leading-tight">
                    {{ cellTooltip(cell).tokens }}
                  </span>
                  <span class="mt-0.5 block text-[9px] font-medium leading-tight opacity-75">
                    {{ cellTooltip(cell).label }}
                  </span>
                  <span class="mt-0.5 block text-[9px] leading-tight opacity-70">
                    {{ cellTooltip(cell).range }}
                  </span>
                </span>
              </button>
            </div>
          </div>

          <div class="mt-2 flex items-center justify-end gap-1.5 text-[10px] font-medium text-[var(--claude-muted)] md:mt-3 md:gap-2 md:text-[11px]">
            <span>{{ t('profile.activity.less') }}</span>
            <span
              v-for="level in 5"
              :key="level"
              class="h-2.5 w-2.5 rounded-[2px] ring-1 ring-black/[0.03] dark:ring-white/[0.04] md:h-3 md:w-3 md:rounded-[3px]"
              :style="{ backgroundColor: levelColor(level - 1) }"
            />
            <span>{{ t('profile.activity.more') }}</span>
          </div>
        </div>
      </div>
    </div>

    <p
      v-if="loading"
      class="mt-4 text-sm text-[var(--claude-muted)]"
    >
      {{ t('common.loading') }}
    </p>
    <p
      v-else-if="errorMessage"
      class="mt-4 text-sm text-red-600 dark:text-red-400"
    >
      {{ errorMessage }}
    </p>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { usageAPI, type UserDashboardStats } from '@/api/usage'
import { useAppStore } from '@/stores/app'
import type { TrendDataPoint } from '@/types'

type HeatmapCell = {
  date: string
  tokens: number
  level: number
  inRange: boolean
}

type ActivitySummary = {
  cells: HeatmapCell[]
  totalTokens: number
  peakDayTokens: number
  currentStreak: number
  longestStreak: number
}

const emit = defineEmits<{
  (event: 'summary', value: ActivitySummary): void
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const errorMessage = ref('')
const trend = ref<TrendDataPoint[]>([])
const stats = ref<UserDashboardStats | null>(null)
const activityMode = ref<'day' | 'week' | 'total'>('day')
const heatmapScroller = ref<HTMLDivElement | null>(null)
const selectedCellKey = ref('')

const today = computed(() => startOfLocalDay(new Date()))
const rangeStart = computed(() => addDays(today.value, -364))
const gridStart = computed(() => addDays(rangeStart.value, -rangeStart.value.getDay()))
const dateTokenMap = computed(() => {
  const map = new Map<string, number>()
  trend.value.forEach((point) => {
    const key = normalizeDateKey(point.date)
    if (key) {
      map.set(key, Number(point.total_tokens || 0))
    }
  })
  return map
})

const maxDailyTokens = computed(() => {
  let max = 0
  dateTokenMap.value.forEach((value) => {
    max = Math.max(max, value)
  })
  return max
})

const cells = computed<HeatmapCell[]>(() => {
  const result: HeatmapCell[] = []
  const end = today.value
  for (let cursor = gridStart.value; cursor <= end; cursor = addDays(cursor, 1)) {
    const dateKey = formatDateKey(cursor)
    const tokens = dateTokenMap.value.get(dateKey) || 0
    result.push({
      date: dateKey,
      tokens,
      level: resolveLevel(tokens, maxDailyTokens.value),
      inRange: cursor >= rangeStart.value && cursor <= end
    })
  }
  return result
})

const weeks = computed(() => {
  const result: HeatmapCell[][] = []
  for (let index = 0; index < cells.value.length; index += 7) {
    result.push(cells.value.slice(index, index + 7))
  }
  return result
})

const displayCells = computed<HeatmapCell[]>(() => {
  if (activityMode.value === 'day') {
    return cells.value
  }

  if (activityMode.value === 'week') {
    const weekTotals = weeks.value.map((week) => week.reduce((sum, cell) => sum + cell.tokens, 0))
    const maxWeekTokens = Math.max(...weekTotals, 0)
    return cells.value.map((cell, index) => {
      const weekTokens = weekTotals[Math.floor(index / 7)] || 0
      return {
        ...cell,
        tokens: weekTokens,
        level: resolveLevel(weekTokens, maxWeekTokens)
      }
    })
  }

  let running = 0
  const totals = cells.value.map((cell) => {
    running += cell.inRange ? cell.tokens : 0
    return running
  })
  const maxTotalTokens = Math.max(...totals, 0)
  return cells.value.map((cell, index) => ({
    ...cell,
    tokens: totals[index] || 0,
    level: resolveLevel(totals[index] || 0, maxTotalTokens)
  }))
})

const displayWeeks = computed(() => {
  const result: HeatmapCell[][] = []
  for (let index = 0; index < displayCells.value.length; index += 7) {
    result.push(displayCells.value.slice(index, index + 7))
  }
  return result
})

const summary = computed<ActivitySummary>(() => {
  const rangeCells = cells.value.filter((cell) => cell.inRange)
  const totalTokens = Number(stats.value?.total_tokens || 0) || rangeCells.reduce((sum, cell) => sum + cell.tokens, 0)
  let longestStreak = 0
  let runningStreak = 0
  for (const cell of rangeCells) {
    if (cell.tokens > 0) {
      runningStreak += 1
      longestStreak = Math.max(longestStreak, runningStreak)
    } else {
      runningStreak = 0
    }
  }

  let currentStreak = 0
  for (let index = rangeCells.length - 1; index >= 0; index -= 1) {
    if (rangeCells[index].tokens <= 0) {
      break
    }
    currentStreak += 1
  }

  return {
    cells: rangeCells,
    totalTokens,
    peakDayTokens: maxDailyTokens.value,
    currentStreak,
    longestStreak
  }
})

const modes = computed(() => [
  { key: 'day' as const, label: t('profile.activity.modes.day') },
  { key: 'week' as const, label: t('profile.activity.modes.week') },
  { key: 'total' as const, label: t('profile.activity.modes.total') }
])

const monthLabels = computed(() => {
  const labels: Array<{ label: string; index: number; left: number }> = []
  const seen = new Set<string>()
  const totalWeeks = Math.max(weeks.value.length - 1, 1)
  cells.value.forEach((cell, index) => {
    const date = parseDateKey(cell.date)
    const monthKey = `${date.getFullYear()}-${date.getMonth()}`
    if (seen.has(monthKey) || date.getDate() > 7) {
      return
    }
    seen.add(monthKey)
    labels.push({
      label: new Intl.DateTimeFormat(undefined, { month: 'short' }).format(date),
      index,
      left: Math.min(98, Math.max(0, (Math.floor(index / 7) / totalWeeks) * 100))
    })
  })
  return labels
})

watch(summary, (value) => emit('summary', value), { immediate: true })
watch(activityMode, () => {
  selectedCellKey.value = ''
})

onMounted(() => {
  void nextTick(scrollHeatmapToLatest)
  void loadActivity().finally(scrollHeatmapToLatest)
})

async function loadActivity() {
  loading.value = true
  errorMessage.value = ''
  try {
    const [trendResponse, statsResponse] = await Promise.all([
      usageAPI.getDashboardTrend({
        start_date: formatDateKey(rangeStart.value),
        end_date: formatDateKey(today.value),
        granularity: 'day'
      }),
      usageAPI.getDashboardStats()
    ])
    trend.value = trendResponse.trend || []
    stats.value = statsResponse
  } catch (error) {
    console.error('Failed to load profile token activity:', error)
    errorMessage.value = t('profile.activity.loadFailed')
    appStore.showError(t('profile.activity.loadFailed'))
  } finally {
    loading.value = false
  }
}

function scrollHeatmapToLatest() {
  void nextTick(() => {
    const scroller = heatmapScroller.value
    if (!scroller || scroller.scrollWidth <= scroller.clientWidth) {
      return
    }
    scroller.scrollLeft = scroller.scrollWidth - scroller.clientWidth
  })
}

function startOfLocalDay(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate())
}

function addDays(date: Date, amount: number): Date {
  const next = new Date(date)
  next.setDate(next.getDate() + amount)
  return next
}

function formatDateKey(date: Date): string {
  const year = date.getFullYear()
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  return `${year}-${month}-${day}`
}

function normalizeDateKey(value: string): string {
  const trimmed = String(value || '').trim()
  if (/^\d{4}-\d{2}-\d{2}/.test(trimmed)) {
    return trimmed.slice(0, 10)
  }
  const date = new Date(trimmed)
  if (Number.isNaN(date.getTime())) {
    return ''
  }
  return formatDateKey(date)
}

function parseDateKey(value: string): Date {
  const [year, month, day] = value.split('-').map((part) => Number(part))
  return new Date(year, (month || 1) - 1, day || 1)
}

function resolveLevel(tokens: number, maxTokens: number): number {
  if (tokens <= 0) {
    return 0
  }
  if (maxTokens <= 0) {
    return 1
  }
  const ratio = tokens / maxTokens
  if (ratio >= 0.75) return 4
  if (ratio >= 0.45) return 3
  if (ratio >= 0.18) return 2
  return 1
}

function levelColor(level: number): string {
  const colors = ['#ebedf0', '#9be9a8', '#40c463', '#30a14e', '#216e39']
  return colors[Math.min(Math.max(level, 0), colors.length - 1)]
}

function cellKey(cell: HeatmapCell): string {
  return `${activityMode.value}:${cell.date}`
}

function toggleSelectedCell(cell: HeatmapCell) {
  const key = cellKey(cell)
  selectedCellKey.value = selectedCellKey.value === key ? '' : key
}

function cellTitle(cell: HeatmapCell): string {
  const tooltip = cellTooltip(cell)
  return `${tooltip.label} · ${tooltip.range} · ${tooltip.tokens}`
}

function cellTooltip(cell: HeatmapCell): { tokens: string; label: string; range: string } {
  if (activityMode.value === 'week') {
    const { start, end } = weekRangeForCell(cell)
    return {
      tokens: formatTokenAmount(cell.tokens),
      label: t('profile.activity.tooltip.weekLabel'),
      range: t('profile.activity.tooltip.weekRange', {
        start: formatDateLabel(start),
        end: formatDateLabel(end)
      })
    }
  }

  if (activityMode.value === 'total') {
    return {
      tokens: formatTokenAmount(cell.tokens),
      label: t('profile.activity.tooltip.totalLabel'),
      range: t('profile.activity.tooltip.totalRange', {
        date: formatDateLabel(parseDateKey(cell.date))
      })
    }
  }

  return {
    tokens: formatTokenAmount(cell.tokens),
    label: t('profile.activity.tooltip.dayLabel'),
    range: formatDateLabel(parseDateKey(cell.date))
  }
}

function weekRangeForCell(cell: HeatmapCell): { start: Date; end: Date } {
  const date = parseDateKey(cell.date)
  const weekStart = addDays(date, -date.getDay())
  const weekEnd = addDays(weekStart, 6)
  return {
    start: maxDate(weekStart, rangeStart.value),
    end: minDate(weekEnd, today.value)
  }
}

function minDate(left: Date, right: Date): Date {
  return left <= right ? left : right
}

function maxDate(left: Date, right: Date): Date {
  return left >= right ? left : right
}

function formatDateLabel(date: Date): string {
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric'
  }).format(date)
}

function formatTokenAmount(tokens: number): string {
  return `${formatCompactNumber(tokens)} Token`
}

function formatCompactNumber(value: number): string {
  const abs = Math.abs(value || 0)
  if (abs >= 1_000_000_000) {
    return `${trimNumber(value / 1_000_000_000)}B`
  }
  if (abs >= 1_000_000) {
    return `${trimNumber(value / 1_000_000)}M`
  }
  if (abs >= 1_000) {
    return `${trimNumber(value / 1_000)}K`
  }
  return `${Math.round(value || 0)}`
}

function trimNumber(value: number): string {
  return value.toFixed(value >= 10 ? 1 : 2).replace(/\.?0+$/, '')
}
</script>
