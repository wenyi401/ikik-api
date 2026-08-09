import { nextTick, onBeforeUnmount } from 'vue'
import { driver, type Driver, type DriveStep } from 'driver.js'
import { useI18n } from 'vue-i18n'
import 'driver.js/dist/driver.css'

export interface PageTutorialOptions {
  beforeNext?: (step: DriveStep, index: number) => void | Promise<void>
  beforePrevious?: (step: DriveStep, index: number) => void | Promise<void>
  onClose?: () => void | Promise<void>
}

export function usePageTutorial() {
  const { t } = useI18n()
  let instance: Driver | null = null

  async function startPageTutorial(
    steps: DriveStep[],
    onComplete?: () => void,
    options: PageTutorialOptions = {}
  ): Promise<void> {
    if (steps.length === 0) return
    instance?.destroy()
    await nextTick()
    let transitioning = false

    instance = driver({
      steps,
      showProgress: true,
      animate: true,
      allowClose: true,
      stagePadding: 6,
      popoverClass: 'theme-tour-popover theme-page-tutorial',
      nextBtnText: t('common.next'),
      prevBtnText: t('common.back'),
      doneBtnText: t('common.confirm'),
      onNextClick: (_element, _step, { state, config }) => {
        const lastIndex = (config.steps?.length ?? 1) - 1
        const currentIndex = state.activeIndex ?? 0
        if (currentIndex >= lastIndex) {
          instance?.destroy()
          instance = null
          onComplete?.()
          return
        }
        if (transitioning) return
        transitioning = true
        void (async () => {
          try {
            await options.beforeNext?.(steps[currentIndex], currentIndex)
            if (!instance?.isActive()) return
            instance.moveNext()
          } finally {
            transitioning = false
          }
        })()
      },
      onPrevClick: (_element, _step, { state }) => {
        const currentIndex = state.activeIndex ?? 0
        if (currentIndex <= 0 || transitioning) return
        transitioning = true
        void (async () => {
          try {
            await options.beforePrevious?.(steps[currentIndex], currentIndex)
            if (!instance?.isActive()) return
            instance.movePrevious()
          } finally {
            transitioning = false
          }
        })()
      },
      onCloseClick: () => {
        instance?.destroy()
        instance = null
        void options.onClose?.()
      }
    })
    instance.drive()
  }

  onBeforeUnmount(() => {
    instance?.destroy()
    instance = null
  })

  return { startPageTutorial }
}
