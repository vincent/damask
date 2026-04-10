---
outline: deep
---

# Local-First, Remote-Optional

Damask is designed to run entirely on your own machine or server. There is no cloud dependency, no mandatory account, and no data that leaves your control by default. When you're ready to add remote storage or collaborate with a team across the internet, you can - on your terms.

## What "local-first" means in practice

- **No account required to self-host.** Download the binary, point it at a folder, run it.
- **No internet connection required.** Everything works offline - uploads, transforms, search, the full product.
- **Your files stay where you put them.** By default, assets are stored in a directory on the same machine running Damask. Nothing is synced or uploaded anywhere without your explicit configuration.
- **The database is a single file.** SQLite - on your filesystem, easy to back up, easy to move.

## Running Damask locally

### Quick start

```bash
# Download the latest archive for your platform from the releases page
open https://github.com/vincent/damask/releases

# Extract, update config, and run it - that's it
./damask-server
```

Damask starts on `http://localhost:8080` by default. Open it in your browser.

On first run, it creates:
- `./damask.db` - the SQLite database
- `./storage/` - the directory where your asset files are stored

Both locations are configurable via environment variables (see below).

### Configuration

Damask is configured entirely through environment variables. No config files required.

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP port to listen on |
| `DB_PATH` | `./damask.db` | Path to the SQLite database file |
| `STORAGE_PATH` | `./storage` | Directory for local file storage |
| `JWT_SECRET` | - | **Required.** A random secret for signing auth tokens. Generate with `openssl rand -hex 32` |
| `BASE_URL` | `http://localhost:8080` | Public URL - used in share links and emails |

Copy `.env.example` from the repository and fill in `JWT_SECRET` at minimum.

### Running as a service

To run Damask persistently on a Linux server, create a systemd unit:

```ini
[Unit]
Description=Damask DAM
After=network.target

[Service]
User=damask
WorkingDirectory=/opt/damask
EnvironmentFile=/opt/damask/.env
ExecStart=/opt/damask/damask-server
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable --now damask
```

---

## Storage backends

Damask abstracts file storage behind a simple interface. The backend is configured by setting `STORAGE_BACKEND` in your environment.

### Local filesystem (default)

```bash
STORAGE_BACKEND=local
STORAGE_PATH=./storage
```

Files are stored as-is on the local filesystem. Simple, fast, no dependencies. Back up this directory along with `damask.db` to have a complete backup.

<!--
### S3-compatible (AWS S3, Cloudflare R2, MinIO, Backblaze B2)

```bash
STORAGE_BACKEND=s3
S3_ENDPOINT=           # leave blank for AWS; set for R2/MinIO/B2
S3_REGION=us-east-1
S3_BUCKET=my-damask-bucket
S3_ACCESS_KEY_ID=your-key
S3_SECRET_ACCESS_KEY=your-secret
S3_PATH_STYLE=false    # set to true for MinIO
```

When using an S3 backend, the database (`damask.db`) still lives locally. Only asset files are stored in the bucket.

### Switching backends

Changing `STORAGE_BACKEND` on an existing instance does not migrate existing files. Assets uploaded before the switch remain accessible at their original location. New uploads go to the new backend.

A migration utility to move files between backends is planned for a future release.
-->

## Remote deployment

Damask's single-binary design makes it straightforward to deploy on a VPS, a home server, or any Linux machine accessible over the internet.

### Reverse proxy with HTTPS

Place Damask behind a reverse proxy such as Caddy or nginx. Caddy handles TLS automatically:

```
# Caddyfile
damask.yourdomain.com {
    reverse_proxy localhost:8080
}
```

Set `BASE_URL=https://damask.yourdomain.com` in your Damask environment so that share links and email notifications use the correct public address.

### Docker

```dockerfile
docker run -d \
  --name damask \
  -p 8080:8080 \
  -v /data/damask:/data \
  -e DB_PATH=/data/damask.db \
  -e STORAGE_PATH=/data/storage \
  -e JWT_SECRET=your-secret-here \
  ghcr.io/vincent/damask:latest
```

or

```yaml
 damask:
    image: ghcr.io/vincent/damask:main
    restart: unless-stopped
    environment:
      APP_SECRET: your_very_long_secure_jwt_application_key
      JWT_SECRET: your_very_long_secure_jwt_secret_key
      BASE_URL: https://base.url
    ports:
      - 25:2525 # to ingest mail
      - 80:8080 # to expose http
    volumes:
      - /local/path:/data
```

## Backups

A complete Damask backup consists of two things:

1. **`damask.db`** - the SQLite database. This contains all metadata, users, workspaces, tags, field definitions, share links, events, and job state.
2. **The storage directory** (or your S3 bucket) - the actual asset files.

Both are required. The database without the files is a library with broken links. The files without the database are an unorganised folder.

### Backup the database

SQLite databases are safe to copy while the server is running (WAL mode is enabled by default). A simple daily cron:

```bash
0 3 * * * cp /data/damask.db /backups/damask-$(date +%Y%m%d).db
```

Or use `sqlite3`'s online backup for a fully consistent snapshot:

```bash
sqlite3 /data/damask.db ".backup '/backups/damask.db'"
```

### Backup the storage

For local filesystem storage, `rsync` to a backup destination:

```bash
rsync -a /data/storage/ /backups/storage/
```

For S3 storage, use your provider's versioning or cross-region replication features, or sync to a second bucket.
