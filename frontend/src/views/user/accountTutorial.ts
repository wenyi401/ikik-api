import type { DriveStep } from 'driver.js'
import type { OnboardingMode } from '@/types'

export const ACCOUNT_TUTORIAL_QUERY = 'accounts'
const ACCOUNT_TUTORIAL_VERSION = 'v2'

type Translate = (key: string) => string

export function accountTutorialStorageKey(userID: number | string): string {
  return `account_tutorial_${userID}_${ACCOUNT_TUTORIAL_VERSION}`
}

export function shouldAutoStartAccountTutorial(
  mode: OnboardingMode | undefined,
  hasSeenTutorial: boolean
): boolean {
  return mode === 'beginner' && !hasSeenTutorial
}

export function createAccountTutorialSteps(
  t: Translate,
  carpoolEnabled: boolean
): DriveStep[] {
  const steps: DriveStep[] = [
    {
      element: '[data-guide="accounts-list"]',
      popover: {
        title: t('onboarding.accounts.tour.modesTitle'),
        description: t('onboarding.accounts.tour.modesDescription'),
        side: 'top',
        align: 'start'
      }
    },
    {
      element: '[data-guide="accounts-create"]',
      popover: {
        title: t('onboarding.accounts.tour.createTitle'),
        description: t('onboarding.accounts.tour.createDescription'),
        side: 'bottom',
        align: 'end'
      }
    },
    {
      element: '[data-guide="account-share-mode"]',
      popover: {
        title: t('onboarding.accounts.tour.shareTitle'),
        description: t('onboarding.accounts.tour.shareDescription'),
        side: 'right',
        align: 'start'
      }
    },
    {
      element: '[data-guide="account-proxy-field"]',
      popover: {
        title: t('onboarding.accounts.tour.proxyTitle'),
        description: t('onboarding.accounts.tour.proxyDescription'),
        side: 'right',
        align: 'start'
      }
    },
    {
      element: '[data-guide="accounts-tools"] [role="menu"]',
      popover: {
        title: t('onboarding.accounts.tour.toolsTitle'),
        description: t('onboarding.accounts.tour.toolsDescription'),
        side: 'left',
        align: 'end'
      }
    },
    {
      element: '[data-guide="accounts-list"]',
      popover: {
        title: t('onboarding.accounts.tour.statusTitle'),
        description: t('onboarding.accounts.tour.statusDescription'),
        side: 'top',
        align: 'start'
      }
    }
  ]

  if (carpoolEnabled) {
    steps.push({
      element: '[data-guide="accounts-carpool"]',
      popover: {
        title: t('onboarding.accounts.tour.carpoolTitle'),
        description: t('onboarding.accounts.tour.carpoolDescription'),
        side: 'bottom',
        align: 'end'
      }
    })
  }

  return steps
}
