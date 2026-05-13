# Damask

A self-hosted Digital Asset Management (DAM) system. Single binary, local-first, multi-tenant.

## Stack

- **Backend:** Go + Fiber v2 + SQLite (CGO-free via modernc)
- **Frontend:** SvelteKit 5 (runes) + Tailwind CSS v4 + SPA mode
- **Auth:** HMAC-SHA256 JWTs stored as httpOnly cookies (7-day expiry)
- **Storage:** Local filesystem (`Storage` interface, swappable for S3)
- **Jobs:** In-process SQLite-backed job queue (thumbnails, variants, ingress)
- **Ingress:** IMAP, SFTP, WebDAV, S3, Email API sources with rule engine
- **Mail server:** Bundled SMTP server (port 2525) forward emails directly into asset ingress
- **Real-time:** SSE event stream for live thumbnail updates

> **Single-node only.** Damask currently uses SQLite and an in-process job queue: running multiple instances behind a load balancer is not supported and will cause data corruption.

## Getting Started with Docker

```yaml
damask:
    container_name: damask
    image: ghcr.io/vincent/damask:latest
    hostname: damask
    restart: unless-stopped
    environment:
      APP_SECRET: change-me-to-a-long-random-secret
      JWT_SECRET: change-me-to-a-long-random-secret
      BASE_URL: https://your.own.damask.com
    expose:
      - 2525
    ports:
      - 80:8080 # web
      - 25:2525 # to enable email ingress
    volumes:
      - /srv/damask/data:/data
```
or
```sh
docker run --name damask -h damask --restart unless-stopped -e APP_SECRET=change-me-to-a-long-random-secret -e JWT_SECRET=change-me-to-a-long-random-secret -e BASE_URL=https://your.own.damask.com --expose 2525 -p 80:8080 -p 25:2525 -v /srv/damask/data:/data ghcr.io/vincent/damask:latest
```

## Getting Started with local install

### Prerequisites

- Go 1.22+
- Node.js 20+
- ImageMagick 6 (for image processing)
- FFmpeg (for video/audio processing)

### Setup

```bash
# 1. Clone and enter the repo
git clone https://github.com/vincent/damask && cd damask

# 2. Copy and configure env
cp .env.example .env
# Edit JWT_SECRET and APP_SECRET, both must be >= 32 chars

# 3. Start dev servers (Go API + SvelteKit)
make dev
```

API: http://localhost:8080  
Web: http://localhost:5173

### First run

1. Open http://localhost:5173 and register the first account; this creates your workspace.
2. Upload assets via drag-and-drop or the upload button.
3. Invite team members from the workspace settings.

### Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | API server port |
| `MAIL_PORT` | `2525` | Bundled SMTP server port |
| `DB_PATH` | `./damask.db` | SQLite database path |
| `STORAGE_LOCAL_PATH` | `./storage` | Asset storage directory |
| `BASE_URL` | `http://localhost:5173` | Public URL for share links and email webhooks |
| `JWT_SECRET` | - | **Required.** HMAC key, >= 32 chars |
| `APP_SECRET` | - | **Required.** AES-256-GCM key for ingress config encryption, >= 32 chars |
| `APP_ENV` | `development` | `development` or `production` (controls secure cookies) |
| `QUEUE_WORKERS` | `4` | Job worker pool size (transcode capped at 2) |
| `FFMPEG_PATH` | - | Optional. Absolute or resolvable path to the `ffmpeg` binary; `ffprobe` is inferred from the same install when possible |
| `FFMPEG_HW_ACCEL` | - | Optional. Video decode hw accel hint for ffmpeg. Supported: `videotoolbox`, `vaapi`, `qsv`, `cuda` |
| `tesseract` | - | Optional runtime dependency for OCR text extraction. Install `tesseract-ocr` plus needed language packs such as `tesseract-ocr-eng` |
| `REMOVEBG_API_KEY` | - | Optional. Enables background removal via remove.bg |
| `ENABLE_SCHEDULER` | `true` | Enable automatic ingress polling |

## Commands

| Command | Description |
|---------|-------------|
| `make dev` | Start Go server + SvelteKit dev server |
| `make build` | Compile Go binary to `bin/server` |
| `make test` | Run Go tests |
| `make lint` | Run golangci-lint + ESLint |
| `make generate` | Run sqlc code generation |

## Structure

```
damask/
├── cmd/server/         # Go entry point + embedded web assets
│   └── web/            # SvelteKit frontend
├── internal/
│   ├── api/            # Fiber handlers and route registration
│   ├── auth/           # JWT maker + middleware
│   ├── config/         # Env-based config
│   ├── db/             # SQLite open + migrations
│   ├── db/gen/         # sqlc-generated queries (do not edit)
│   ├── events/         # SSE broadcast hub
│   ├── ingress/        # Ingress sources, rules engine, scheduler
│   ├── jobs/           # Background job handlers
│   ├── queue/          # SQLite-backed job queue + worker pool
│   ├── services/       # Asset upload, SMTP server
│   ├── storage/        # Storage interface + LocalStorage
│   └── transform/      # Image/video/audio/PDF processing
└── Makefile
```

## Bundled mail server

Damask includes an SMTP server (default port `2525`) that accepts inbound email and routes attachments directly into asset ingress. This enables workflows like emailing photos or documents to a dedicated address and having them appear in your library automatically.

**Setup:**

1. Configure an **Email API** ingress source in the UI and note the generated ingest token.
2. Point your MX record (or mail relay) at the Damask host, port `2525`.
3. Expose port `2525` (or `25` via port mapping in Docker requires root/CAP_NET_BIND_SERVICE).
4. Emails sent to `<ingest-token>@your.domain.com` are processed and attachments ingested according to the source's rule set.

## Reverse proxy

Damask binds to a single port (default `8080`). Place it behind a reverse proxy for TLS and large upload support.

> Set the upload limit generously : assets can be large video/audio files.

### Caddy

```caddy
your.domain.com {
    request_body {
        max_size 1GB
    }
    reverse_proxy localhost:8080
}
```

### nginx

```nginx
server {
    listen 443 ssl;
    server_name your.domain.com;

    client_max_body_size 1G;

    location / {
        proxy_pass         http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header   Host              $host;
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;

        # Required for SSE (/api/v1/events)
        proxy_buffering    off;
        proxy_cache        off;
        proxy_read_timeout 3600s;
    }
}
```

## Troubleshooting

### ImageMagick PDF/PS policies

Remove these policies from `/etc/ImageMagick-6/policy.xml` to enable PDF thumbnail generation:

```xml
<policy domain="coder" rights="none" pattern="PS" />
<policy domain="coder" rights="none" pattern="PS2" />
<policy domain="coder" rights="none" pattern="PS3" />
<policy domain="coder" rights="none" pattern="EPS" />
<policy domain="coder" rights="none" pattern="PDF" />
<policy domain="coder" rights="none" pattern="XPS" />
```

### Dirty DB migration

If the DB is stuck in a dirty migration state:

```bash
sqlite3 damask.db "UPDATE schema_migrations SET version=<last_clean_version>, dirty=0;"
```

Then restart the server to reapply the migration.
