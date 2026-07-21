export default {
  common: {
    required: '必填',
    tryAgain: '重试',
    sending: '发送中...',
    creating: '创建中...',
    clear: '清除',
    apply: '应用',
    uncategorized: '未分类',
  },
  admin: {
    accounts: {
      fromModel: '源模型',
      toModel: '目标模型',
      status: {
        expired: '已过期',
      },
      oauth: {
        openai: {
          accessTokenAuth: '访问令牌',
          mobileRefreshTokenAuth: '移动端刷新令牌',
        },
      },
    },
    users: {
      passwordCopied: '密码已复制',
    },
    channels: {
      emptyModelsInPricing: '暂无已配置价格的模型',
      noGroupsSelected: '请至少选择一个分组',
    },
    settings: {
      openaiFastPolicy: {
        addUserId: '添加用户 ID',
        removeUserId: '移除用户 ID',
        userIdPlaceholder: '输入用户 ID',
      },
    },
    ops: {
      runtime: {
        metricThresholds: '指标阈值',
        metricThresholdsHint: '用于运行状态和告警判断的阈值。',
        requestErrorRateMaxPercent: '请求错误率上限',
        requestErrorRateMaxPercentHint: '请求错误率超过该比例时判定为异常。',
        slaMinPercent: 'SLA 下限',
        slaMinPercentHint: '可用率低于该比例时判定为异常。',
        ttftP99MaxMs: 'P99 首字延迟上限',
        ttftP99MaxMsHint: 'P99 首字延迟超过该值时判定为异常。',
        upstreamErrorRateMaxPercent: '上游错误率上限',
        upstreamErrorRateMaxPercentHint: '上游错误率超过该比例时判定为异常。',
      },
    },
  },
  store: {
    errors: {
      SHOP_ORDER_NOT_FOUND: '订单不存在',
    },
  },
}
