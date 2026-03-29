# BaDAM

A self-hosted Digital Asset Management (DAM) system. Single binary, local-first, multi-tenant.

## Stack

- **Backend:** Go + Fiber + SQLite (CGO-free via modernc)
- **Frontend:** SvelteKit + TypeScript + shadcn-svelte
- **Auth:** Paseto v4 tokens (httpOnly cookies)
- **Storage:** Local filesystem (S3-compatible interface, swap at config level)

## Quick Start

```bash
# 1. Copy env
cp .env.example .env
# Edit JWT_SECRET in .env

# 2. Run dev servers (Go + SvelteKit)
make dev
```

Server: http://localhost:8080
Web: http://localhost:5173

## Structure

```
badam-dam/
├── server/    # Go API + job runner
├── web/       # SvelteKit frontend
└── Makefile   # dev / build / test / generate
```

## Commands

| Command | Description |
|---------|-------------|
| `make dev` | Start Go server + SvelteKit dev server |
| `make build` | Compile Go binary to `server/bin/server` |
| `make test` | Run Go tests |
| `make lint` | Run golangci-lint + ESLint |
| `make generate` | Run sqlc code generation |
