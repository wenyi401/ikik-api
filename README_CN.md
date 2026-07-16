# ikik-api

![Go](https://img.shields.io/badge/Go-1.26.5-00ADD8?logo=go&logoColor=white)
![Vue](https://img.shields.io/badge/Vue-3-42b883?logo=vuedotjs&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-4169E1?logo=postgresql&logoColor=white)
![Redis](https://img.shields.io/badge/Redis-7+-DC382D?logo=redis&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)
![License](https://img.shields.io/badge/License-LGPL--3.0-blue)

ikik-api 是基于 Sub2API 二次开发的自托管 AI API 网关与订阅管理平台，提供账号池、API Key 管理、多供应商请求转发、用量计费、订阅充值、风控审查和后台运营能力。

[English](README.md) | 中文 | [日本語](README_JA.md)

站点：[https://ikik.net](https://ikik.net)

QQ 群：`146499741`

本仓库用于私有部署、定制和二次开发，不包含生产密钥、私有服务器配置、托管服务凭据或商业运营数据。

## 重要说明

部署或运营本项目前，请仔细阅读以下内容：

- 服务条款风险：通过订阅账号或账号型上游转发请求，可能违反部分上游供应商的服务条款。使用前请自行核对相关协议。
- 合规要求：请仅在符合所在国家或地区法律法规的前提下使用本项目。
- 账号风险：账号封禁、额度重置、服务中断、上游策略调整和计费异常都属于部署者需要自行承担和处理的运营风险。
- 免责声明：本项目仅用于技术学习、自托管和二次开发。你的部署、数据、用户、支付和上游账号均由你自行负责。

## 功能特性

- 提供 OpenAI 兼容网关接口，支持 chat、responses、models、embeddings、image 和流式请求等场景。
- 支持 Grok OAuth、Kiro OAuth、免费模型供应商接入和可配置的私有账号接入流程。
- 支持 OpenAI 兼容渠道和账号型上游的多供应商路由。
- 账号池管理，包含公共、私有、自有和拼车等调度概念。
- API Key 管理，支持多分组路由、IP 访问控制、额度控制、使用记录和计费元数据。
- 用户订阅、充值流程、兑换码、邀请奖励和商城/卡密流程。
- 管理后台覆盖用户、账号、渠道、支付、风控、风险事件、数据管理和系统设置。
- 内容审查与风控接入点，支持请求/响应审计。
- 内置发布流程，支持标签构建、Docker 镜像、归档包和 GitHub Releases。
- 前端控制台基于 Vue 3、TypeScript、Pinia、Vue Router、Tailwind CSS 和 Vite。
- 后端服务基于 Go、Gin、Ent、PostgreSQL、Redis 和模块化服务边界。

## 1.0.3 更新内容

- 后端工具链升级到 Go 1.26.5，并更新存储集成相关的 AWS SDK 安全依赖。
- 新增 Grok OAuth、Kiro OAuth、K12 账号等级支持，并补充视频相关网关端点覆盖。
- 新增免费模型供应商接入、多分组 API Key 路由和 API Key IP 访问控制支持。
- 优化拼车池、私有账号、订阅、计费、推理 Token 和用量统计相关流程。
- 更新 CI、安全扫描和前端审计处理，匹配当前依赖版本。

## 技术栈

- 后端：Go 1.26.5、Gin、Ent、PostgreSQL、Redis
- 前端：Vue 3、TypeScript、Vite、Pinia、Tailwind CSS
- 测试：Go test、Vitest、vue-tsc、ESLint
- 部署：Docker 或源码构建，推荐外置 PostgreSQL 和 Redis

## 仓库结构

```text
.
├── backend/              # Go 后端、迁移、服务、处理器、仓储层
├── frontend/             # Vue 3 管理端/用户端控制台
├── deploy/               # 部署示例和配置模板
├── docs/                 # 额外集成和运维文档
├── assets/               # 项目静态资源
├── Makefile              # 常用构建和测试入口
└── Dockerfile            # 生产镜像构建
```

## 环境要求

- Go 1.26.5
- Node.js 20+
- pnpm 9+
- PostgreSQL
- Redis
- Docker，可选但推荐用于部署

## 配置

从示例配置开始：

```bash
cp deploy/config.example.yaml deploy/config.yaml
```

根据你的环境修改生成的配置：

- `server`：监听地址、端口、前端地址、请求体限制、CORS 和安全响应头。
- `database`：PostgreSQL 连接设置。
- `redis`：缓存和队列后端设置。
- `gateway`：上游超时、请求体大小限制、路由和模型行为。
- `security`：URL 白名单、响应头过滤、代理兜底和 CSP。
- 按需配置 payment、email、storage、moderation 和 OAuth 等部分。

不要提交真实生产凭据。本地和部署专用配置文件已被 git 忽略。

## 开发

安装前端依赖：

```bash
pnpm --dir frontend install
```

启动前端开发服务器：

```bash
pnpm --dir frontend run dev
```

从源码运行后端：

```bash
cd backend
go run ./cmd/server
```

首次运行时，如果没有有效配置或安装状态，后端可能会进入初始化设置流程。

## 构建

构建后端和前端：

```bash
make build
```

仅构建后端：

```bash
make build-backend
```

仅构建前端：

```bash
make build-frontend
```

构建 Docker 镜像：

```bash
docker build -t ikik-api:local .
```

## 测试

运行全部已配置检查：

```bash
make test
```

运行后端测试：

```bash
cd backend
go test -tags=unit ./...
go test -tags=integration ./...
```

运行前端检查：

```bash
pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
pnpm --dir frontend run i18n:audit:strict
pnpm --dir frontend exec vitest run
```

使用仓库配置运行 golangci-lint：

```bash
cd backend
golangci-lint run ./... --timeout=30m
```

如果本地没有安装 `golangci-lint`，可以使用和 CI 相同的版本：

```bash
cd backend
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.9.0 run ./... --timeout=30m
```

## 部署说明

生产环境建议将 ikik-api 运行在 Nginx、Caddy 或托管负载均衡器等反向代理之后。

### Nginx 反向代理说明

如果使用 Nginx，并且启用了账号调度、粘性会话、Codex CLI，或客户端会发送带下划线的请求头，请在 Nginx 的 `http` 块中启用：

```nginx
underscores_in_headers on;
```

Nginx 默认会丢弃带下划线的请求头，这可能破坏会话路由和部分原生客户端兼容路径。

推荐的生产基础设置：

- 使用应用容器外部的 PostgreSQL 和 Redis。
- 挂载生产配置文件，不要把密钥写入镜像。
- 在反向代理或负载均衡器处终止 TLS。
- 不要让 `/api/*`、`/v1/*`、流式接口和网关路由进入 CDN 缓存。
- 统一配置反向代理和后端的请求体大小限制。
- 在执行迁移或升级应用前备份 PostgreSQL。

## 安全

- 不要提交 API Key、OAuth Secret、支付密钥、数据库密码或服务器凭据。
- 在公开服务前仔细检查 `deploy/config.example.yaml`。
- 使用强密码、可用时启用 MFA，并通过可信反向代理规则限制后台访问。
- 支付、存储、风控和邮件凭据应只授予最低必要权限。
- 发布变更前运行 `make secret-scan`。

## 许可证

本项目遵循 [LICENSE](LICENSE) 中包含的许可证。

## 致谢

ikik-api 基于 Sub2API 构建，并在其基础上扩展了自托管 AI 网关、订阅、计费和运营流程。

- [PIXEL-API/PixelAPI](https://github.com/PIXEL-API/PixelAPI)
- [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api)
