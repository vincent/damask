# Installation

Damask ships as a single static binary with no runtime dependencies.

## Docker (recommended)

```yaml
# docker-compose.yml
services:
  damask:
    image: ghcr.io/your-org/damask:latest
    ports:
      - '8080:8080'
    volumes:
      - ./data:/data
    environment:
      DATA_DIR: /data
      BASE_URL: https://dam.example.com
```

```bash
docker compose up -d
```

## Binary

Download the latest release from the [releases page](https://github.com/your-org/damask/releases).

```bash
chmod +x damask-linux-amd64
DATA_DIR=/var/lib/damask BASE_URL=https://dam.example.com ./damask-linux-amd64
```

## Reverse proxy (Caddy)

```caddyfile
dam.example.com {
    reverse_proxy localhost:8080
}
```

Caddy handles TLS automatically via Let's Encrypt.

## Reverse proxy (nginx)

```nginx
server {
    listen 443 ssl;
    server_name dam.example.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        client_max_body_size 500M;
    }
}
```

## First run

On first start Damask runs all migrations and creates the database. Visit `BASE_URL` and register the first account - this account automatically becomes the workspace owner.
