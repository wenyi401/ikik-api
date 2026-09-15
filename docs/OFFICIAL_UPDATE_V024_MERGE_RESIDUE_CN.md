# sub2api v0.2.4 合并后的残留清单

本文件记录本次合并（fork `ikik` ← 上游 sub2api v0.2.4）之后**仍然存在**的上游差异，
按"是否属于漏合"分类，便于后续排期。审计方法：对每个上游改动过的文件，
逐行比较「上游相对 merge-base 新增/修改的行」是否出现在当前文件里
（脚本：`audit3`，逻辑见提交说明）。

## 已确认属于 fork 有意差异（不是漏合，无需处理）

| 文件 | 上游行为 | fork 行为 |
| --- | --- | --- |
| `components/user/dashboard/UserDashboardStats.vue` | 396 行，含 `PLATFORM_LABELS`、按平台拆分卡片 | 72 行，改用 `UiMetricStrip`；平台名统一走 `@/utils/platformColors`（已覆盖 antigravity/zhipu/deepseek/minimax） |
| `views/admin/ChannelsView.vue` | `platformOrder`/`compositePlatforms` 常量 | `platformOrder` 是上游超集（多 kiro、opencode_go）；composite 分组按 `g.platform === 'composite'` 处理，语义等价 |
| `views/admin/UsageView.vue` | `resetFilters` 少重置 `upstream_model_mismatch` | fork 的超集 |
| `utils/*`、`api/*` 的类型/注释行 | — | 仅排版差异（审计脚本按整行匹配产生的假阳性） |

## 需要产品决策：上游新版界面（会替换 fork 自己的编辑器）

* `views/admin/GroupsView.vue`：上游的峰值倍率、Codex 网页搜索计价、视频计价、生图批量标签、
  `getQuotaUsageClass` 用量配色、`accountsCount`、`admin.groups.accountFilters.*` 文案
  （约 22 个 i18n key + 30 个 helper）未移植；移植会覆盖 fork 现有的价格编辑 UI。

## 上游 spec 中有、合并时丢失的用例（恢复进度 88/~180）

已恢复并全绿：

* `router/__tests__/guards.spec.ts`（34 条）
* `api/__tests__/settings.authSourceDefaults.spec.ts`（13 条）
* `api/__tests__/admin.users.spec.ts`（3 条）
* `stores/__tests__/app.spec.ts`（sidebarScrollTop 1 条；另有 3 条并发语义用例仍失败，未保留）
* `components/layout/__tests__/AppSidebar.spec.ts`（9 条，含滚动位置持久化）
* `views/admin/__tests__/SettingsView.spec.ts`（12 条，37/37 全绿）：补齐了充值返利开关
  `affiliate_admin_recharge_enabled`、`rewrite_message_cache_control`、
  Claude OAuth system prompt 注入（含 blocks JSON 归一化编辑器）、Users tab 默认平台配额矩阵
  （提交前用 `sanitizePlatformQuotasMap` 把空输入清成 null）、支付服务商 `supported_types`
  归一化；TTFT 模式 / Antigravity UA 版本 / Grok 跨客户端映射三处上游 UI 本来就已存在
* `utils/__tests__/registrationEmailPolicy.spec.ts`（3 条，13/13 全绿）：注册邮箱后缀白名单
  支持 `*.` 通配符匹配与 `formatRegistrationEmailSuffixWhitelistForMessage`
* `api/__tests__/client.spec.ts`（10 条，25/25 全绿）：API base 相对路径归一化、请求拦截器补回 `X-Admin-UI-Request` / `X-User-UI-Request` 标记、刷新被新会话取代时返回 `AUTH_SESSION_CHANGED`；三条刷新用例按 fork 的 cookie/内存会话模型改写
* `api/authSession.spec.ts`（新增 3 条，9/9 全绿）：并发刷新单飞、刷新途中退出后不恢复会话、服务端 `AUTH_SESSION_MISMATCH` 不采纳新 bundle —— 覆盖 legacy tokenRefresh 用例的等价保证

仍未恢复（每条都对应一处上游功能/行为，需要实现后才能放回）：

| 文件 | 未恢复用例数 | 需要的上游能力 |
| --- | --- | --- |
| `components/payment/__tests__/paymentFlow.spec.ts` | 10 | `forceQRCode` 分支、支付宝 JSAPI 细节 |
* `api/__tests__/tokenRefresh.spec.ts`：上游 7 条针对 legacy localStorage 令牌模型，已被 fork 的 cookie/内存会话模型取代；等价保证见上一条 authSession 新增用例
| `stores/__tests__/auth.spec.ts` | 6 | localStorage 损坏清理等 |
| `PaymentResultView` / `PendingOAuthCreateAccountForm` / `UsersView` / `AccountUsageCell` / `OllamaCloudUsageCell` / `UsageProgressBar` / `AccountStatusIndicator` / `TokenUsageTrend` / `useModelWhitelist` / `AccountTestModal` / `data-import` / `title` / `tablePreferences` / `useOpenAIOAuth` / Ollama 去抖控件 等 | 合计约 30 | 各自对应的上游行为（详见 diff 审计输出） |

> 说明：这些用例原本在合并前的 fork 树上同样失败（合并丢失的是用例本身），
> 因此**新增失败为 0**；但它们代表了尚未对齐的上游行为，建议按表逐项排期。

## 验证基线

* 前端全量：`73 failed / 2045 passed`（合并前基线 `142 failed / ~1583 passed`，新增失败 0）
* `vue-tsc --noEmit`：0 错误；`pnpm run build`：成功
* 后端：`go build ./...` 通过；`go test -p 1 ./internal/...` 41 个失败，
  抽样 4 个在合并前的 worktree 上同样失败（fork 既有差异）
* 运行期迁移验证：本机无 Docker，建议在预发执行一次 `--migrate-only`
  （重点：235/236 号迁移只新增 `model_allowlist` 列，不重命名 fork 的 `models_list_config`）
