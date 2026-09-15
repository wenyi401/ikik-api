// v0.2.4 merge: keys referenced by merged views but absent from en/ modules (mirrors merge-additions-zh; values recovered from legacy en base by key name).
export default {
  admin: {
    accounts: {
      accountCreatedSuccess: "Account added successfully",
      accountDeletedSuccess: "Account deleted",
      accountUpdatedSuccess: "Account updated",
      cookieRefreshedSuccess: "Cookie refreshed successfully",
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
        credentialsLabel: "Credentials",
        credentialsPlaceholder: "Enter a cookie or API key",
        nameLabel: "Name",
        namePlaceholder: "Example: Personal Claude account",
        platformLabel: "Platform",
        priorityHint: "Lower value accounts are used first",
        priorityLabel: "Priority",
        selectPlatform: "Select platform",
        selectType: "Select type",
        statusLabel: "Account Status",
        typeLabel: "Type",
        weightHint: "Weight used for load balancing",
        weightLabel: "Weight",
      },
      noAccounts: "No accounts in this group",
      noAccountsDescription: "Add an AI platform account to start using the API gateway.",
      refreshCookie: "Refresh cookie",
      refreshing: "Refreshing…",
      saving: "Saving...",
      testAccount: "Test account",
      testSuccess: "Connection test passed",
      types: {
        api_key: "API Key",
        cookie: "Cookie",
      },
    },
    groups: {
      compositeRoutes: {
        upstreamModelHint: "Upstream model sent to the target platform after route rewrite",
      },
    },
  },
}
