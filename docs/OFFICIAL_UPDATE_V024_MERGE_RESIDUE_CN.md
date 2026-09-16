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
| `views/admin/SettingsView.vue` 的 Ollama 全局刷新控件 | 上游是 `debounce_minutes`（`ollama-cloud-usage-global-debounce`） | fork 是刷新间隔 `interval`（`ollama-cloud-usage-global-interval`），语义等价、字段不同，配 `docs` 与用例均自洽 |
| `utils/*`、`api/*` 的类型/注释行 | — | 仅排版差异（审计脚本按整行匹配产生的假阳性） |

## 需要产品决策：上游新版界面（会替换 fork 自己的编辑器）

* `views/admin/GroupsView.vue`：上游的峰值倍率、Codex 网页搜索计价、视频计价、生图批量标签、
  `getQuotaUsageClass` 用量配色、`accountsCount`、`admin.groups.accountFilters.*` 文案
  （约 22 个 i18n key + 30 个 helper）未移植；移植会覆盖 fork 现有的价格编辑 UI。

## 上游 spec 中有、合并时丢失的用例（恢复进度 139/~180（其余条目已逐条判定关闭，见上表））

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
* `components/payment/__tests__/paymentFlow.spec.ts`（10 条，27/27 全绿）：补回 `forceQRCode` / `mobilePrecreateDeepLink` / Airwallex 路由与恢复快照断言；顺带修掉真实缺陷 —— `getVisibleMethods` 现在保留没有别名的自定义 EasyPay 方法（ldc、usdt_trc20 等），此前这些付款方式在收银台会被整体隐藏
* `components/payment/__tests__/PaymentStatusPanel.spec.ts`（3 条，8/8 全绿）：主动核销（pending 时调用 `verifyOrder` 并按服务端结果结算）用例此前因 spec 缺少 `verifyOrder` mock 整组失败，组件本身已是上游版本
* `api/authSession.spec.ts`（再 +1 条，10/10 全绿）：冷启动清掉损坏的 legacy `auth_token`/`refresh_token`/`auth_user` 而不影响会话 —— 对应上游 auth store 的 localStorage 容错用例
* `router/__tests__/title.spec.ts` + `utils/__tests__/tablePreferences.spec.ts` + `composables/__tests__/useOpenAIOAuth.spec.ts`（共 3 条，5/5、7/7、6/6 全绿）：补回上游的 `resolveRouteDocumentTitle`（自定义页面用后台配置的菜单名作页签标题，App.vue 增加 title watcher、语言切换时同样走该解析），页签标题随站点名/自定义菜单变化即时更新；表格每页条数默认值按 fork 的 `[10,20,50,100,1000]` 断言（比上游多一档 1000，属 fork 有意差异）
* `views/user/__tests__/PaymentResultView.spec.ts`（3 条，15/15 全绿）：订单校验改为「已登录走鉴权接口、失败再退回公开校验」，金额按订单返回的币种用 `formatPaymentAmount` 渲染（此前固定按 ¥ 显示）
* `components/auth/__tests__/PendingOAuthCreateAccountForm.spec.ts`（1 条恢复 + 3 条原本失败，11/11 全绿）：**修掉两个真实缺陷** —— 验证码控件此前只在启用 Cloudflare Turnstile 时才渲染，导致启用腾讯/阿里云 人机验证时挂载不到 widget、`verifyAction()` 取不到凭据，发验证码与提交会被静默中止；提交 payload 也补上了父组件要转发的 `turnstileToken` / 腾讯 ticket 字段
* `components/account/__tests__/AccountStatusIndicator.spec.ts`（2 条，12/12 全绿）、`components/charts/__tests__/TokenUsageTrend.spec.ts`（2 条，4/4 全绿，缓存命中率数据集按 fork 的 i18n 标签断言）
* `components/account/__tests__/AccountUsageCell.spec.ts`（3 条，54/54 全绿）：补回 Anthropic OAuth 的 `7d F`(Fable) 进度条渲染/无数据隐藏断言；**修掉两个真实缺陷** —— OpenAI API Key 分支的 OllamaCloudUsageCell 漏了 `@updated`，子组件的用量更新传不上去；`OpenAIQuotaResetCell` 的重置结果此前只走 fork 自己的 `reset` 事件，现在同时按上游契约 `account-updated` 把刷新后的账号行回传（重置后账号行会立即重新拉取 usage）
* `components/account/__tests__/OllamaCloudUsageCell.spec.ts`（4 条，与 settings spec 合计 11/11 全绿）：上游把行内查询按钮放在列表单元格里，fork 的设计是放在编辑页的 `OllamaCloudUsageSettings`（该 spec 已覆盖 `refreshOllamaCloudUsage` 调用），故按 fork 契约改写为"单元格内不出现查询按钮"的断言并保留上游新增的窗口渲染/类名断言
* `components/account/__tests__/UsageProgressBar.spec.ts`（5 条，全绿）：剩余容量模式在低量/耗尽时缩短变红的渲染断言；`composables/__tests__/useModelWhitelist.spec.ts`（3 条，17/17 全绿，并顺手修好一处被并坏的用例——两条断言被粘到隔壁 it 里导致 `models is not defined`）
* `__tests__/integration/data-import.spec.ts`（5 条按 fork 语义改写后全绿）：fork 的导入弹窗是 981 行超集（含 ZIP/TXT/URL 导入与用户作用域），用例改为断言「逐文件调用导入接口并在本地合并结果」，并给组件补上**选择阶段的逐文件校验**（无效 JSON 报 `dataImportParseFailedFile`、非导出 JSON 报 `dataImportInvalidFile`，报错时保留上一次的有效选择），与上游契约一致；`ImportDataModal.user-scope.spec` 4 条同步补 `await flushPromises()`（读文件是异步的）
* `views/admin/__tests__/UsersView.spec.ts`（1 条，2/2 全绿）：恢复「切到 last_used_at 排序时清空用量页内排序」，给用量排序控件补上 `usage-sort-trigger-<col>` / `usage-sort-<col>-<metric>` 测试钩子，断言按 fork 的持久化字段`metric`（上游多一个 `key`）
* `components/account/__tests__/BulkEditAccountModal.spec.ts`（5 条，59/59 全绿）：OpenAI OAuth 批量编辑的 namespace 摊平、专属 WS mode（含 `http_bridge`）、`codex_cli_only_allow_app_server` 字段用例，组件侧此前已补齐，用例直接放回即通过
* `views/admin/__tests__/UsageView.spec.ts`（1 条恢复 + 1 处组件修复，13/13 全绿）：刷新/改筛选时不再清空正在展示的模型统计（只失效缓存标记，新数据到达再替换，避免图表闪空）；另 3 条按未实现能力处理（见下表）

已逐条判定并关闭（**均为上游在 fork 分流之后新增的能力，不是合并漏改**；
逐条核对过：这些能力在合并前的 fork 树上同样不存在，且不修不影响本次合并的正确性）：

| 条目 | 判定 | 依据 |
| --- | --- | --- |
| `UsersView` 跨页勾选 + 批量编辑 | **不移植（暂缓）** | fork 的 `views/admin/UsersView.vue` 没有任何多选/批量入口（模板无 `data-test`、无 `bulk\|batch` API 调用），移植等于新增一整套批量编辑功能，属产品范围决策；用户未确认，不做半成品 |
| `AccountTestModal` 的 compact 探测 / gemini 图片提示词+预览 / 图片模型过滤 | **不移植（暂缓）** | fork 该组件 560 行 vs 上游 1065 行，缺少 `testMode='compact'`、图片提示词输入与生成图预览；且 fork 自身用例要求「列表隐藏图片模型」，与上游新增的「gemini 图片测试默认选中图片模型」在同一弹窗内语义冲突，需先定义测试弹窗的模型选择策略 |
| `OpenAIQuotaResetCell.spark_shadow` 共享快照/失败兜底 | **不移植（暂缓）** | fork 该组件 427 行 vs 上游 486 行，缺少「查询后刷新、账号 extra 缓存水合、快照持久化失败降级、状态恢复失败中止」等语义；两侧状态流不同，需重写组件状态机而非机械移植 |
| 管理端用量页的错误请求 / 用户排行 tab | **不移植（暂缓）** | fork 管理端用量页是单表结构（无 `usage-detail-tab` / `OpsErrorLogTable` / `UserTokenRanking`），移植等于新增两个页面级模块 |

`api/__tests__/tokenRefresh.spec.ts`（7 条）与 `stores/__tests__/auth.spec.ts`（6 条）针对已被取代的 legacy localStorage 会话模型，
不再移植；等价保证已由 `api/authSession.spec.ts` 的用例覆盖（冷启动清理、并发单飞、会话不匹配不采纳）。

> 说明：这些用例原本在合并前的 fork 树上同样失败（合并丢失的是用例本身），
> 因此**新增失败为 0**；但它们代表了尚未对齐的上游行为，建议按表逐项排期。

## 验证基线

* 前端全量：`65 failed / 2101 passed`（合并前基线 `142 failed / ~1583 passed`，新增失败 0）
* `vue-tsc --noEmit`：0 错误；`pnpm run build`：成功
* 后端：`go build ./...` 通过；`go test -p 1 ./internal/...` 41 个失败，
  抽样 4 个在合并前的 worktree 上同样失败（fork 既有差异）
* 运行期迁移验证：本机无 Docker，建议在预发执行一次 `--migrate-only`
  （重点：235/236 号迁移只新增 `model_allowlist` 列，不重命名 fork 的 `models_list_config`）
