# Developer Account API / 开发者账号 API

## 中文

### 用途与启用

该 API 向所有正常用户开放，用于导入、查看、共享和删除自己的上游 OAuth 账号。它不使用模型调用 API Key，而是使用独立的开发者令牌。

启用步骤：

1. 用户在“账号管理 -> 开发者 API”中创建令牌并选择权限，无需管理员逐个开启。
2. 明文令牌只显示一次。服务端仅在 `developer_tokens` 表中保存 SHA-256 哈希。
3. 管理员仍可对异常用户单独关闭开发者 API。

> 安全说明：开发者令牌仅保存 SHA-256 哈希；账号凭证仍沿用既有 `accounts.credentials` JSONB 存储。管理员应定期审计开发者操作，并及时关闭异常用户的访问。

### 鉴权与权限

所有开发者接口的基础路径为 `/api/developer/v1`：

```http
Authorization: Bearer ikd_xxx
Content-Type: application/json
```

| Scope | 能力 |
| --- | --- |
| `accounts:read` | 查看自有账号和共享任务 |
| `accounts:write` | 导入和删除自有账号 |
| `accounts:share` | 提交公开共享申请或恢复私有模式 |
| `bot:access` | 允许 QQ 机器人只读访问本人的资料、用量、渠道、账号汇总和共享统计 |

用户被禁用、管理员针对该用户关闭开发者 API、令牌过期或令牌被撤销后，鉴权立即失效。

### 幂等性

以下写接口必须提供 `Idempotency-Key`：

- `POST /account-imports`
- `PUT /accounts/{id}/sharing`
- `DELETE /accounts/{id}`

重试同一操作时使用相同的 key 和完全相同的请求体。新操作必须生成新的 key。key 最长 128 字符，不能包含控制字符。

### 导入账号

`POST /api/developer/v1/account-imports`，需要 `accounts:write`。每次允许 1-50 个账号。

```bash
curl -X POST "https://example.com/api/developer/v1/account-imports" \
  -H "Authorization: Bearer ikd_xxx" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: import-20260731-001" \
  -d '{
    "accounts": [
      {
        "external_id": "account-001",
        "name": "OpenAI Account",
        "platform": "openai",
        "type": "oauth",
        "credentials": {
          "refresh_token": "***"
        }
      }
    ],
    "share_mode": "public",
    "proxy_id": 123
  }'
```

- 载荷采用 Sub2API 语义：`platform`、`type`、`credentials`。
- 每个账号项只接受 `external_id`、`name`、`notes`、`platform`、`type`、`credentials`、`extra`；其他字段一律拒绝。
- V1 仅允许 `type: "oauth"`。
- OpenAI 可以提交 `refresh_token`，导入时会在线换取并验证凭证；其他平台当前应提交有效的 `access_token` 及其所需 OAuth 元数据。
- `external_id` 仅用于对应本次响应项，不写入账号记录，也不替代 `Idempotency-Key`。
- `proxy_id` 必须是当前用户在 IKIK 中已有且有效的自有代理 ID。不能直接提交代理 URL。
- 禁止提交服务端管理字段：`owner_user_id`、`account_level`、`group_ids`、`share_status`、`share_policy_id`、`rate_multiplier`、`concurrency`、`priority`。

响应按账号返回成功或失败，批次中的单项失败不会回滚其他成功项：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 1,
    "created": 1,
    "failed": 0,
    "items": [
      {
        "index": 1,
        "external_id": "account-001",
        "account_id": 456,
        "status": "created"
      }
    ],
    "share_task": {
      "id": 789,
      "scope": "user",
      "operation": "user_set_public_share",
      "status": "pending"
    }
  }
}
```

`share_mode: "public"` 还需要 `accounts:share`。账号始终先以私有模式导入，再异步执行测试和共享审批；创建成功不代表已经进入公开池。使用 `GET /account-tasks/{task_id}` 查询任务。

### 其他接口

| 方法与路径 | Scope | 说明 |
| --- | --- | --- |
| `GET /health` | 任一有效令牌 | 鉴权与版本检查 |
| `GET /accounts` | `accounts:read` | 分页查看自有账号，支持 `page`、`page_size`、`platform`、`type`、`status`、`search`、`privacy_mode` |
| `GET /accounts/{id}` | `accounts:read` | 查看一个自有账号 |
| `GET /account-tasks/{task_id}` | `accounts:read` | 查看自己的异步共享任务 |
| `PUT /accounts/{id}/sharing` | `accounts:share` | 请求 `{"mode":"public"}` 或设置 `{"mode":"private"}` |
| `DELETE /accounts/{id}` | `accounts:write` | 删除自己的账号 |

所有账号和任务查询都校验所有权。访问其他用户的资源会按不存在处理。

### QQ 机器人接口

用户可创建只包含 `bot:access` 的开发者令牌，并在 QQ 私聊中发送 `/绑定 ikd_xxx`。令牌代表创建它的用户，机器人接口不能传入或切换 `user_id`。

| 方法与路径 | 说明 |
| --- | --- |
| `GET /bot/profile` | 当前令牌用户的个人资料和分享卡设置 |
| `GET /bot/usage/stats` | 累计及今日用量汇总 |
| `GET /bot/usage/trend` | 按日期范围或 `period=today/week` 查询趋势 |
| `GET /bot/channels` | 管理员已启用的渠道监控，以及对应公开共享号池的可调度账号、并发、5 小时和 7 天额度汇总 |
| `GET /bot/channels/openai` | OpenAI 官方状态 |
| `GET /bot/channels/providers` | Claude、Grok、Gemini 官方状态 |
| `GET /bot/accounts/summary` | 自有账号总数和失效数量，不返回账号详情 |
| `GET /bot/sharing` | 自有账号共享审核及收益统计 |

这些接口全部需要 `bot:access`，并且只使用令牌所属用户的身份。撤销令牌后，QQ 机器人绑定会在下一次调用时失效。

### 错误格式

```json
{
  "code": 403,
  "message": "developer token does not have the required scope",
  "reason": "DEVELOPER_TOKEN_SCOPE_REQUIRED",
  "metadata": {
    "scope": "accounts:share"
  }
}
```

常见 `reason`：`INVALID_DEVELOPER_TOKEN`、`DEVELOPER_TOKEN_EXPIRED`、`DEVELOPER_TOKEN_SCOPE_REQUIRED`、`IDEMPOTENCY_KEY_REQUIRED`、`ACCOUNT_IMPORT_FIELD_NOT_ALLOWED`、`ACCOUNT_IMPORT_CREDENTIALS_INVALID`。

完整机器可读定义见 [`developer-account-api.openapi.yaml`](developer-account-api.openapi.yaml)。

## English

### Purpose and enablement

This API is available to every active user for importing, inspecting, sharing, and deleting their own upstream OAuth accounts. It uses dedicated developer tokens, not model API keys.

1. The user creates a scoped token under **Account Management -> Developer API** without waiting for per-user administrator approval.
2. The plaintext token is shown once. Only its SHA-256 hash is stored in `developer_tokens`.
3. Administrators can still disable developer API access for an individual abusive account.

> Security note: developer tokens are stored only as SHA-256 hashes; account credentials still use the existing `accounts.credentials` JSONB storage. Administrators should audit developer operations and disable access for abusive users when necessary.

The base path is `/api/developer/v1`. Send `Authorization: Bearer ikd_xxx`. Available scopes are `accounts:read`, `accounts:write`, `accounts:share`, and `bot:access`. The bot scope provides read-only access to the token owner's profile, usage, configured channel-monitor and shared-pool summaries, official provider status, account summary, and sharing statistics. Disabling the user or explicitly disabling developer API access for that user, revoking the token, or reaching its expiry invalidates access immediately.

All write endpoints require `Idempotency-Key`. Reuse the same key only when retrying the exact same request; use a new key for a new operation.

The import payload and behavior match the Chinese example above and use Sub2API semantics: `platform`, `type`, and `credentials`. Each account item only accepts `external_id`, `name`, `notes`, `platform`, `type`, `credentials`, and `extra`; all other fields are rejected. V1 accepts OAuth accounts only. OpenAI refresh tokens are exchanged and validated during import; other platforms currently require an access token and relevant OAuth metadata.

Public sharing is asynchronous. Accounts are imported privately first, then submitted through a `user_set_public_share` task. Poll `GET /account-tasks/{task_id}` and do not treat import success as public-share approval.

All account and task reads enforce ownership. The route and scope table above is language-neutral, and the complete contract is available in [`developer-account-api.openapi.yaml`](developer-account-api.openapi.yaml).
