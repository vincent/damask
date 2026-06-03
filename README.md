# Damask

A self-hosted Digital Asset Management (DAM) system. Single binary, local-first, multi-tenant. [Try the demo](https://staging.damask.studio/login)

![Assets grid & folders](https://raw.githubusercontent.com/vincent/damask/refs/heads/main/cmd/server/web/static/docs/screenshot_asset_folders_drop.png)
![Open asset](https://raw.githubusercontent.com/vincent/damask/refs/heads/main/cmd/server/web/static/docs/screenshot_asset_open.dark.png)
![Workflow editor](https://raw.githubusercontent.com/vincent/damask/refs/heads/main/cmd/server/web/static/docs/screenshot_workflows_editor.png)

## Features

**Asset management**

- Upload, search, paginate, bulk tag, bulk move
- Projects and folders for organisation
- Tags with optional controlled vocabulary
- EXIF metadata extraction from photos (camera, exposure, GPS opt-in, date)
- Media tag extraction from audio/video (ID3, Vorbis, iTunes atoms, RIFF INFO)
- Custom metadata fields on assets and projects (text, number, date, boolean, select, URL)
- Asset versioning with non-destructive rollback and content-hash deduplication
- Collections — workspace-scoped named sets that cross project boundaries
- Append-only activity log (renames, retags, moves, field changes, version restores, shares)

**Processing**

- ImageMagick for image thumbnails and transforms
- FFmpeg for video and audio transcoding
- PDF thumbnail generation
- OCR text extraction with full-text search
- Background removal via ImageRouter

**Variants**

- Multiple output variants per asset: resize, crop, format conversion, background removal, AI prompt
- Promote variant to main asset or set as thumbnail
- Variant thumbnails and rerun support
- Variant drafts — preview-and-pick before committing
- Shared variants — per-variant public links with visitor name gate and ZIP export
- Lightroom-style creation sidebar in the asset lightbox

**Sharing**

- Password-protected share links
- Expiring share links
- Client review mode — public commenting without an account

**Ingress**

- Sources: IMAP, SFTP, WebDAV, S3-compatible, Email API
- Bundled SMTP server — email attachments ingested directly
- Rule engine per source (filename patterns, MIME types, metadata)
- Scheduled polling and manual trigger

**Integrations**

- OIDC login, Google OAuth, Canva OAuth
- Google Drive and Canva ingress sources
- ImageRouter AI model integration (workspace-scoped API key)

**Workflows**

- Visual trigger/action builder with async job execution

**Mobile**

- Responsive layout with bottom navigation, slide-in drawer, touch-friendly interactions

**Observability**

- OpenTelemetry tracing (plugs into any OTel-compatible backend)

## Stack

- **Backend:** Go + Fiber v3 + SQLite (CGO-free via modernc)
- **Frontend:** SvelteKit 5 (runes) + Tailwind CSS v4 + SPA mode
- **Auth:** HMAC-SHA256 JWTs stored as httpOnly cookies (7-day expiry)
- **Storage:** Local filesystem (`Storage` interface, swappable for S3)
- **Jobs:** In-process SQLite-backed job queue (thumbnails, variants, ingress)
- **Ingress:** IMAP, SFTP, WebDAV, S3, Email API sources with rule engine
- **Mail server:** Bundled SMTP server (port 2525) forward emails directly into asset ingress
- **Real-time:** SSE event stream for live thumbnail updates

> **Single-node only.** Damask currently uses SQLite and an in-process job queue: running multiple instances behind a load balancer is not supported and will cause data corruption.

## Getting Started with Docker (recommended)

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
    - damask_data:/data
```

or

```sh
docker run --name damask -h damask \
           --restart unless-stopped \
           -e APP_SECRET=change-me-to-a-long-random-secret \
           -e JWT_SECRET=change-me-to-a-long-random-secret \
           -e BASE_URL=https://your.own.damask.com \
           --expose 2525 \
           -p 80:8080 -p 25:2525 \ 
           -v /srv/damask/data:/data \
           ghcr.io/vincent/damask:latest
```

## Getting Started with local install

### Prerequisites

- Go 1.22+
- Node.js 20+
- ImageMagick 6 (for image processing)
- FFmpeg (optional, for video/audio processing)
- LibreOffice (optional, for document processing)
- Tesseract (optional, for OCR text extraction — install language packs e.g. `tesseract-ocr-eng`)

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

| Variable                               | Default                                  | Description                                                                          |
| -------------------------------------- | ---------------------------------------- | ------------------------------------------------------------------------------------ |
| `PORT`                                 | `8080`                                   | API server port                                                                      |
| `BASE_URL`                             | `http://localhost:5173`                  | Public URL for share links and email webhooks                                        |
| `JWT_SECRET`                           | -                                        | **Required.** HMAC key, >= 32 chars                                                  |
| `APP_SECRET`                           | -                                        | **Required.** AES-256-GCM key for ingress config encryption, >= 32 chars             |
| `APP_ENV`                              | `development`                            | `development` or `production` (controls secure cookies)                              |
| `DB_PATH`                              | `./damask.db`                            | SQLite database path                                                                 |
| `QUEUE_WORKERS`                        | `4`                                      | Job worker pool size (transcode capped at 2)                                         |
| `FRONTEND_PATH`                        | -                                        | Serve frontend from disk instead of the embedded build                               |
| `STORAGE`                              | `local`                                  | Storage backend: `local`, `sftp`, or `s3`                                            |
| `STORAGE_LOCAL_PATH`                   | `./storage`                              | Asset storage directory (local backend)                                              |
| `STORAGE_SFTP_HOST`                    | -                                        | SFTP host                                                                            |
| `STORAGE_SFTP_PORT`                    | `22`                                     | SFTP port                                                                            |
| `STORAGE_SFTP_USER`                    | -                                        | SFTP username                                                                        |
| `STORAGE_SFTP_AUTH_METHOD`             | `password`                               | `password` or `key`                                                                  |
| `STORAGE_SFTP_PASSWORD`                | -                                        | SFTP password (auth method: password)                                                |
| `STORAGE_SFTP_PRIVATE_KEY`             | -                                        | PEM private key (auth method: key)                                                   |
| `STORAGE_SFTP_BASE_PATH`               | `/`                                      | Base path on the SFTP server                                                         |
| `STORAGE_SFTP_INSECURE_HOST_KEY`       | `false`                                  | Skip host key verification (not recommended)                                         |
| `STORAGE_S3_BUCKET`                    | -                                        | S3 bucket name                                                                       |
| `STORAGE_S3_REGION`                    | -                                        | S3 region                                                                            |
| `STORAGE_S3_ACCESSKEY`                 | -                                        | S3 access key                                                                        |
| `STORAGE_S3_SECRETKEY`                 | -                                        | S3 secret key                                                                        |
| `STORAGE_S3_BASE_PATH`                 | `/`                                      | Key prefix within the bucket                                                         |
| `SMTP_HOST`                            | -                                        | Outbound SMTP relay host                                                             |
| `SMTP_PORT`                            | `587`                                    | Outbound SMTP port                                                                   |
| `SMTP_SENDER`                          | -                                        | From address for outbound email                                                      |
| `SMTP_USER`                            | -                                        | SMTP auth username                                                                   |
| `SMTP_PASS`                            | -                                        | SMTP auth password                                                                   |
| `MAIL_PORT`                            | `2525`                                   | Bundled inbound SMTP server port                                                     |
| `MAIL_HOST`                            | -                                        | Override hostname advertised by the inbound SMTP server                              |
| `FFMPEG_PATH`                          | -                                        | Absolute or resolvable path to `ffmpeg`; `ffprobe` is inferred from the same install |
| `FFMPEG_HW_ACCEL`                      | -                                        | Video decode hw accel hint. Supported: `videotoolbox`, `vaapi`, `qsv`, `cuda`        |
| `SCRATCH_PURGE_TIME`                   | `03:00`                                  | Daily time (HH:MM) to purge variant draft scratch storage                            |
| `ENABLE_SCHEDULER`                     | `true`                                   | Enable automatic ingress polling                                                     |
| `IMAGEROUTER_API_KEY`                  | -                                        | Global fallback ImageRouter API key                                                  |
| `IMAGEROUTER_DEFAULT_MODEL`            | `black-forest-labs/FLUX-2-klein-4b:free` | Default image generation model                                                       |
| `IMAGEROUTER_DEFAULT_BG_REMOVE_MODEL`  | `bria/remove-background:free`            | Default background removal model                                                     |
| `IMAGEROUTER_RETRY_PAID_ON_FREE_LIMIT` | `false`                                  | Retry with a paid model when free tier is exhausted                                  |
| `OIDC_ISSUER_URL`                      | -                                        | OIDC provider issuer URL                                                             |
| `OIDC_CLIENT_ID`                       | -                                        | OIDC client ID                                                                       |
| `OIDC_CLIENT_SECRET`                   | -                                        | OIDC client secret                                                                   |
| `OIDC_LABEL`                           | `Sign in with SSO`                       | Login button label                                                                   |
| `GOOGLE_CLIENT_ID`                     | -                                        | Google OAuth client ID                                                               |
| `GOOGLE_CLIENT_SECRET`                 | -                                        | Google OAuth client secret                                                           |
| `CANVA_CLIENT_ID`                      | -                                        | Canva OAuth client ID                                                                |
| `CANVA_CLIENT_SECRET`                  | -                                        | Canva OAuth client secret                                                            |
| `OTEL_ENABLED`                         | `false`                                  | Enable OpenTelemetry tracing                                                         |
| `OTEL_ENDPOINT`                        | `http://localhost:8082/api/otel/v1`      | OTel collector endpoint                                                              |
| `OTEL_TOKEN`                           | `dev-token`                              | Bearer token for the OTel endpoint                                                   |

**Demo mode**

| Variable                    | Default              | Description                                  |
| --------------------------- | -------------------- | -------------------------------------------- |
| `DEMO_MODE`                 | `false`              | Enable demo mode (auto-reset, public access) |
| `DEMO_RESET_INTERVAL_HOURS` | -                    | Hours between demo resets                    |
| `DEMO_USER_EMAIL`           | `demo@damask.studio` | Demo workspace owner email                   |
| `DEMO_WORKSPACE_NAME`       | `Demo Agency`        | Demo workspace display name                  |
| `DEMO_BANNER`               | `true`               | Show the demo banner in the UI               |
| `DEMO_SIGNUP_URL`           | `/signup`            | URL the demo banner links to for sign-up     |

## Commands

| Command         | Description                            |
| --------------- | -------------------------------------- |
| `make dev`      | Start Go server + SvelteKit dev server |
| `make build`    | Compile Go binary to `bin/server`      |
| `make test`     | Run Go tests                           |
| `make lint`     | Run golangci-lint + ESLint             |
| `make generate` | Run sqlc code generation               |

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

### Nginx

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

Remove policies from `/etc/ImageMagick-6/policy.xml` to enable PDF thumbnail generation:

```shell
sed -i \
    -e '/disable ghostscript format types/,+6d' \
    -e '/name="width"/d' \
    -e '/name="height"/d' \
    -e '/domain="path"/d' \
    /etc/ImageMagick-6/policy.xml
```

### Dirty DB migration

If the DB is stuck in a dirty migration state:

```bash
sqlite3 damask.db "UPDATE schema_migrations SET version=<last_clean_version>, dirty=0;"
```

Then restart the server to reapply the migration.
