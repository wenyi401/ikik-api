export default {
  common: {
    required: 'Required',
    tryAgain: 'Try again',
    sending: 'Sending...',
    creating: 'Creating...',
    clear: 'Clear',
    apply: 'Apply',
    uncategorized: 'Uncategorized',
  },
  admin: {
    accounts: {
      fromModel: 'Source model',
      toModel: 'Target model',
      status: {
        expired: 'Expired',
      },
      oauth: {
        openai: {
          accessTokenAuth: 'Access token',
          mobileRefreshTokenAuth: 'Mobile refresh token',
        },
      },
    },
    users: {
      passwordCopied: 'Password copied',
    },
    channels: {
      emptyModelsInPricing: 'No priced models are available',
      noGroupsSelected: 'Select at least one group',
    },
    settings: {
      openaiFastPolicy: {
        addUserId: 'Add user ID',
        removeUserId: 'Remove user ID',
        userIdPlaceholder: 'Enter a user ID',
      },
    },
    ops: {
      runtime: {
        metricThresholds: 'Metric thresholds',
        metricThresholdsHint: 'Thresholds used by runtime health and alert evaluation.',
        requestErrorRateMaxPercent: 'Maximum request error rate',
        requestErrorRateMaxPercentHint: 'Mark the service unhealthy when the request error rate exceeds this percentage.',
        slaMinPercent: 'Minimum SLA',
        slaMinPercentHint: 'Mark the service unhealthy when availability falls below this percentage.',
        ttftP99MaxMs: 'Maximum P99 first-token latency',
        ttftP99MaxMsHint: 'Mark the service unhealthy when P99 first-token latency exceeds this value.',
        upstreamErrorRateMaxPercent: 'Maximum upstream error rate',
        upstreamErrorRateMaxPercentHint: 'Mark the service unhealthy when the upstream error rate exceeds this percentage.',
      },
    },
  },
  store: {
    errors: {
      SHOP_ORDER_NOT_FOUND: 'Order not found',
    },
  },
}
