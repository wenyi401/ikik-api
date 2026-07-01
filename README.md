# ikik-api

![Go](https://img.shields.io/badge/Go-1.26.2-00ADD8?logo=go&logoColor=white)
![Vue](https://img.shields.io/badge/Vue-3-42b883?logo=vuedotjs&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-4169E1?logo=postgresql&logoColor=white)
![Redis](https://img.shields.io/badge/Redis-7+-DC382D?logo=redis&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)
![License](https://img.shields.io/badge/License-LGPL--3.0-blue)

ikik-api is a self-hosted AI API gateway and subscription management platform based on Sub2API. It provides account pooling, API key management, multi-provider request forwarding, usage accounting, subscription billing, moderation controls, and admin operations for AI API services.

English | [中文](README_CN.md) | [日本語](README_JA.md)

This repository is intended for private deployment, customization, and secondary development. It does not include production secrets, private server configuration, hosted-service credentials, or commercial operation data.

## Important Notice

Please read the following carefully before deploying or operating this project:

- Terms of service risk: routing requests through subscription or account-based upstreams may violate the terms of service of some upstream providers. Review the relevant provider agreements before use.
- Compliance: use this project only in compliance with the laws and regulations of your country or region.
- Account risk: account bans, quota resets, service interruptions, upstream policy changes, and billing errors are operational risks that must be handled by the deployer.
- Disclaimer: this project is provided for technical learning, self-hosting, and secondary development. You are responsible for your own deployment, data, users, payments, and upstream accounts.

## Features

- OpenAI-compatible gateway endpoints for chat, responses, models, embeddings, image, and streaming workloads.
- Multi-provider routing for OpenAI-compatible channels and account-based upstreams.
- Account pool management with public, private, owned, and carpool-style scheduling concepts.
- API key management with group routing, quota controls, usage records, and billing metadata.
- User subscriptions, recharge flows, redeem codes, invitation rewards, and shop/card-key workflows.
- Admin dashboard for users, accounts, channels, payments, moderation, risk events, data management, and system settings.
- Content moderation and risk-control integration points for request/response auditing.
- Built-in release workflow for tagged builds, Docker images, archives, and GitHub Releases.
- Frontend console built with Vue 3, TypeScript, Pinia, Vue Router, Tailwind CSS, and Vite.
- Backend service built with Go, Gin, Ent, PostgreSQL, Redis, and modular service boundaries.

## Tech Stack

- Backend: Go 1.26.2, Gin, Ent, PostgreSQL, Redis
- Frontend: Vue 3, TypeScript, Vite, Pinia, Tailwind CSS
- Testing: Go test, Vitest, vue-tsc, ESLint
- Deployment: Docker or source build, with external PostgreSQL and Redis recommended

## Repository Layout

```text
.
├── backend/              # Go backend, migrations, services, handlers, repositories
├── frontend/             # Vue 3 admin/user console
├── deploy/               # Deployment examples and configuration template
├── docs/                 # Additional integration and operation documents
├── assets/               # Static project assets
├── Makefile              # Common build and test entry points
└── Dockerfile            # Production image build
```

## Requirements

- Go 1.26.2
- Node.js 20+
- pnpm 9+
- PostgreSQL
- Redis
- Docker, optional but recommended for deployment

## Version and Updates

The current source version is `1.0.1`. The version file is located at `backend/cmd/server/VERSION` and is updated by the release workflow when a release tag is published.

Tagged releases build backend binaries, frontend assets, archive packages, Docker images, and multi-architecture manifests through GoReleaser. Docker images are tagged with the exact version and the configured rolling tags such as `latest`.

For public-facing version history and upgrade notes, see [CHANGELOG.md](CHANGELOG.md).

## Configuration

Start from the example configuration:

```bash
cp deploy/config.example.yaml deploy/config.yaml
```

Edit the generated configuration for your environment:

- `server`: host, port, frontend URL, request body limits, CORS, and security headers.
- `database`: PostgreSQL connection settings.
- `redis`: cache and queue backend settings.
- `gateway`: upstream timeout, body-size limits, routing, and model behavior.
- `security`: URL allowlist, response header filtering, proxy fallback, and CSP.
- payment, email, storage, moderation, and OAuth sections as needed.

Never commit real production credentials. Local and deployment-specific config files are intentionally ignored by git.

## Development

Install frontend dependencies:

```bash
pnpm --dir frontend install
```

Run the frontend dev server:

```bash
pnpm --dir frontend run dev
```

Run the backend from source:

```bash
cd backend
go run ./cmd/server
```

On first run, the backend may start the setup flow if no valid configuration or installation state is detected.

## Build

Build backend and frontend:

```bash
make build
```

Build only the backend:

```bash
make build-backend
```

Build only the frontend:

```bash
make build-frontend
```

Build a Docker image:

```bash
docker build -t ikik-api:local .
```

## Tests

Run all configured checks:

```bash
make test
```

Run backend tests:

```bash
cd backend
go test -tags=unit ./...
go test -tags=integration ./...
```

Run frontend checks:

```bash
pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
pnpm --dir frontend run i18n:audit:strict
pnpm --dir frontend exec vitest run
```

Run golangci-lint with the repository configuration:

```bash
cd backend
golangci-lint run ./... --timeout=30m
```

If `golangci-lint` is not installed locally, use the same version as CI:

```bash
cd backend
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.9.0 run ./... --timeout=30m
```

## Deployment Notes

For production, run ikik-api behind a reverse proxy such as Nginx, Caddy, or a managed load balancer.

### Nginx Reverse Proxy Note

When using Nginx with account scheduling, sticky sessions, Codex CLI, or clients that send headers containing underscores, enable underscore headers in the Nginx `http` block:

```nginx
underscores_in_headers on;
```

Nginx drops headers containing underscores by default. That can break session routing and some native client compatibility paths.

Recommended production basics:

- Use PostgreSQL and Redis outside the application container.
- Mount a production config file instead of baking secrets into the image.
- Terminate TLS at the reverse proxy or load balancer.
- Keep `/api/*`, `/v1/*`, streaming, and gateway routes out of CDN cache.
- Configure request body limits consistently across the reverse proxy and backend.
- Back up PostgreSQL before applying migrations or upgrading the application.

## Security

- Do not commit API keys, OAuth secrets, payment keys, database passwords, or server credentials.
- Review `deploy/config.example.yaml` before exposing the service publicly.
- Restrict admin access with strong passwords, MFA where available, and trusted reverse-proxy rules.
- Keep payment, storage, moderation, and email credentials scoped to the minimum required permissions.
- Run `make secret-scan` before publishing changes.

## License

This project follows the license included in [LICENSE](LICENSE).

## Acknowledgements

ikik-api is based on Sub2API and extends it for self-hosted AI gateway, subscription, accounting, and operation workflows.
