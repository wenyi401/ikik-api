// v0.2.4 merge: keys referenced by merged views but absent from en/ modules (mirrors merge-additions-zh; values recovered from legacy en base by key name).
export default {
  admin: {
    accounts: {
      accountCreatedSuccess: "Account created successfully! Welcome to {siteName}.",
      accountDeletedSuccess: "Account deleted",
      accountUpdatedSuccess: "Account updated",
      cookieRefreshedSuccess: "Cookie 刷新成功",
      deleteConfirmMessage: "Delete account \"{name}\"? This action cannot be undone.",
      failedToSave: "Failed to save account",
      filters: {
        allPlatforms: "All Platforms",
        allStatuses: "All statuses",
        allTypes: "All Types",
        platform: "Platform",
        status: "Status",
        type: "Type",
      },
      form: {
        credentialsLabel: "凭证",
        credentialsPlaceholder: "请输入 Cookie 或 API Key",
        nameLabel: "Name",
        namePlaceholder: "Example: Personal Claude account",
        platformLabel: "平台",
        priorityHint: "Lower value accounts are used first",
        priorityLabel: "优先级",
        selectPlatform: "选择平台",
        selectType: "选择类型",
        statusLabel: "Account Status",
        typeLabel: "类型",
        weightHint: "用于负载均衡的权重值",
        weightLabel: "权重",
      },
      noAccounts: "No accounts in this group",
      noAccountsDescription: "添加 AI 平台账号以开始使用 API 网关。",
      refreshCookie: "刷新 Cookie",
      refreshing: "Refreshing…",
      saving: "Saving...",
      testAccount: "测试账号",
      testSuccess: "Connection test passed",
      types: {
        api_key: "API Key",
        cookie: "Cookie",
      },
    },
    groups: {
      compositeRoutes: {
        upstreamModelHint: "路由改写后发往目标平台的上游模型",
      },
    },
  },
}
