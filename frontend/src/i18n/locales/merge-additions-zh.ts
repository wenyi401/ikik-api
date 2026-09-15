// v0.2.4 merge: keys referenced by merged views but absent from en/ modules (recovered from legacy zh base).
export default {
  admin: {
    accounts: {
      accountCreatedSuccess: "账号添加成功",
      accountDeletedSuccess: "账号删除成功",
      accountUpdatedSuccess: "账号更新成功",
      cookieRefreshedSuccess: "Cookie 刷新成功",
      deleteConfirmMessage: "确定要删除账号 '{name}' 吗？",
      failedToSave: "保存账号失败",
      filters: {
        allPlatforms: "全部平台",
        allStatuses: "全部状态",
        allTypes: "全部类型",
        platform: "平台",
        status: "状态",
        type: "类型",
      },
      form: {
        credentialsLabel: "凭证",
        credentialsPlaceholder: "请输入 Cookie 或 API Key",
        nameLabel: "账号名称",
        namePlaceholder: "请输入账号名称",
        platformLabel: "平台",
        priorityHint: "数值越小优先级越高",
        priorityLabel: "优先级",
        selectPlatform: "选择平台",
        selectType: "选择类型",
        statusLabel: "状态",
        typeLabel: "类型",
        weightHint: "用于负载均衡的权重值",
        weightLabel: "权重",
      },
      noAccounts: "暂无账号",
      noAccountsDescription: "添加 AI 平台账号以开始使用 API 网关。",
      refreshCookie: "刷新 Cookie",
      refreshing: "刷新中...",
      saving: "保存中...",
      testAccount: "测试账号",
      testSuccess: "账号测试通过",
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
