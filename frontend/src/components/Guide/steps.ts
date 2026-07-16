import { DriveStep } from 'driver.js'

function cleanTourText(value: string): string {
  return value
    .replace(/[\p{Extended_Pictographic}\uFE0F]/gu, '')
    .replace(/\s+([，。！？；：,.!?;:])/g, '$1')
    .replace(/\s{2,}/g, ' ')
    .trim()
}

/**
 * 管理员完整引导流程
 * 交互式引导：指引用户实际操作
 * @param t 国际化函数
 * @param isSimpleMode 是否为简易模式（简易模式下会过滤分组相关步骤）
 */
export const getAdminSteps = (t: (key: string) => string, isSimpleMode = false): DriveStep[] => {
  const tr = (key: string) => cleanTourText(t(key))
  const allSteps: DriveStep[] = [
  // ========== 欢迎介绍 ==========
  {
    popover: {
      title: tr('onboarding.admin.welcome.title'),
      description: tr('onboarding.admin.welcome.description'),
      align: 'center',
      nextBtnText: tr('onboarding.admin.welcome.nextBtn'),
      prevBtnText: tr('onboarding.admin.welcome.prevBtn')
    }
  },

  // ========== 第一部分：创建分组 ==========
  {
    element: '#sidebar-group-manage',
    popover: {
      title: tr('onboarding.admin.groupManage.title'),
      description: tr('onboarding.admin.groupManage.description'),
      side: 'right',
      align: 'center',
      showButtons: ['close'],
    }
  },
  {
    element: '[data-tour="groups-create-btn"]',
    popover: {
      title: tr('onboarding.admin.createGroup.title'),
      description: tr('onboarding.admin.createGroup.description'),
      side: 'bottom',
      align: 'end',
      showButtons: ['close']
    }
  },
  {
    element: '[data-tour="group-form-name"]',
    popover: {
      title: tr('onboarding.admin.groupName.title'),
      description: tr('onboarding.admin.groupName.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="group-form-platform"]',
    popover: {
      title: tr('onboarding.admin.groupPlatform.title'),
      description: tr('onboarding.admin.groupPlatform.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="group-form-multiplier"]',
    popover: {
      title: tr('onboarding.admin.groupMultiplier.title'),
      description: tr('onboarding.admin.groupMultiplier.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="group-form-exclusive"]',
    popover: {
      title: tr('onboarding.admin.groupExclusive.title'),
      description: tr('onboarding.admin.groupExclusive.description'),
      side: 'top',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="group-form-submit"]',
    popover: {
      title: tr('onboarding.admin.groupSubmit.title'),
      description: tr('onboarding.admin.groupSubmit.description'),
      side: 'left',
      align: 'center',
      showButtons: ['close']
    }
  },

  // ========== 第二部分：创建账号授权 ==========
  {
    element: '#sidebar-channel-manage',
    popover: {
      title: tr('onboarding.admin.accountManage.title'),
      description: tr('onboarding.admin.accountManage.description'),
      side: 'right',
      align: 'center',
      showButtons: ['close']
    }
  },
  {
    element: '[data-tour="accounts-create-btn"]',
    popover: {
      title: tr('onboarding.admin.createAccount.title'),
      description: tr('onboarding.admin.createAccount.description'),
      side: 'bottom',
      align: 'end',
      showButtons: ['close']
    }
  },
  {
    element: '[data-tour="account-form-name"]',
    popover: {
      title: tr('onboarding.admin.accountName.title'),
      description: tr('onboarding.admin.accountName.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="account-form-platform"]',
    popover: {
      title: tr('onboarding.admin.accountPlatform.title'),
      description: tr('onboarding.admin.accountPlatform.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="account-form-type"]',
    popover: {
      title: tr('onboarding.admin.accountType.title'),
      description: tr('onboarding.admin.accountType.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="account-form-priority"]',
    popover: {
      title: tr('onboarding.admin.accountPriority.title'),
      description: tr('onboarding.admin.accountPriority.description'),
      side: 'top',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="account-form-groups"]',
    popover: {
      title: tr('onboarding.admin.accountGroups.title'),
      description: tr('onboarding.admin.accountGroups.description'),
      side: 'top',
      align: 'center',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="account-form-submit"]',
    popover: {
      title: tr('onboarding.admin.accountSubmit.title'),
      description: tr('onboarding.admin.accountSubmit.description'),
      side: 'left',
      align: 'center',
      showButtons: ['close']
    }
  },

  // ========== 第三部分：创建API密钥 ==========
  {
    element: '[data-tour="sidebar-my-keys"]',
    popover: {
      title: tr('onboarding.admin.keyManage.title'),
      description: tr('onboarding.admin.keyManage.description'),
      side: 'right',
      align: 'center',
      showButtons: ['close']
    }
  },
  {
    element: '[data-tour="keys-create-btn"]',
    popover: {
      title: tr('onboarding.admin.createKey.title'),
      description: tr('onboarding.admin.createKey.description'),
      side: 'bottom',
      align: 'end',
      showButtons: ['close']
    }
  },
  {
    element: '[data-tour="key-form-name"]',
    popover: {
      title: tr('onboarding.admin.keyName.title'),
      description: tr('onboarding.admin.keyName.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="key-form-group"]',
    popover: {
      title: tr('onboarding.admin.keyGroup.title'),
      description: tr('onboarding.admin.keyGroup.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="key-form-submit"]',
    popover: {
      title: tr('onboarding.admin.keySubmit.title'),
      description: tr('onboarding.admin.keySubmit.description'),
      side: 'left',
      align: 'center',
      showButtons: ['close']
    }
  }
  ]

  // 简易模式下过滤分组相关步骤
  if (isSimpleMode) {
    return allSteps.filter(step => {
      const element = step.element as string | undefined
      // 过滤掉分组管理和账号分组选择相关步骤
      return !element || (
        !element.includes('sidebar-group-manage') &&
        !element.includes('groups-create-btn') &&
        !element.includes('group-form-') &&
        !element.includes('account-form-groups')
      )
    })
  }

  return allSteps
}

/**
 * 普通用户引导流程
 */
export const getUserSteps = (t: (key: string) => string): DriveStep[] => {
  const tr = (key: string) => cleanTourText(t(key))
  return [
    {
      popover: {
        title: tr('onboarding.user.welcome.title'),
        description: tr('onboarding.user.welcome.description'),
        align: 'center',
        nextBtnText: tr('onboarding.user.welcome.nextBtn'),
        prevBtnText: tr('onboarding.user.welcome.prevBtn')
      }
    },
    {
      element: '[data-tour="sidebar-my-keys"]',
      popover: {
        title: tr('onboarding.user.keyManage.title'),
        description: tr('onboarding.user.keyManage.description'),
        side: 'right',
        align: 'center',
        showButtons: ['close']
      }
    },
    {
      element: '[data-tour="keys-create-btn"]',
      popover: {
        title: tr('onboarding.user.createKey.title'),
        description: tr('onboarding.user.createKey.description'),
        side: 'bottom',
        align: 'end',
        showButtons: ['close']
      }
    },
    {
      element: '[data-tour="key-form-name"]',
      popover: {
        title: tr('onboarding.user.keyName.title'),
        description: tr('onboarding.user.keyName.description'),
        side: 'right',
        align: 'start',
        showButtons: ['next', 'previous']
      }
    },
    {
      element: '[data-tour="key-form-group"]',
      popover: {
        title: tr('onboarding.user.keyGroup.title'),
        description: tr('onboarding.user.keyGroup.description'),
        side: 'right',
        align: 'start',
        showButtons: ['next', 'previous']
      }
    },
    {
      element: '[data-tour="key-form-submit"]',
      popover: {
        title: tr('onboarding.user.keySubmit.title'),
        description: tr('onboarding.user.keySubmit.description'),
        side: 'left',
        align: 'center',
        showButtons: ['close']
      }
    }
  ]
}
