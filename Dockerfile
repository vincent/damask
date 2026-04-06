# ── Go build ──────────────────────────────────────────────────────────────────
FROM golang:1.25-bookworm AS go-build
WORKDIR /build/server
COPY server/ .
RUN CGO_ENABLED=0 go build -mod=mod -trimpath -ldflags="-s -w" -o /out/damask-server ./cmd/server
RUN CGO_ENABLED=0 go build -tags=demo -mod=mod -trimpath -ldflags="-s -w" -o /out/damask-server-demo ./cmd/server
RUN CGO_ENABLED=0 go build -mod=mod -trimpath -ldflags="-s -w" -o /out/damask-admin ./cmd/admin

# ── Web build ─────────────────────────────────────────────────────────────────
FROM node:24-bookworm-slim AS web-build
WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN VITE_API_URL="" npm run build

# ── Runtime ───────────────────────────────────────────────────────────────────
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates ffmpeg imagemagick && rm -rf /var/lib/apt/lists/*

# remove coders polices from imagemagick config
RUN sed -i '/disable ghostscript format types/,+6d' /etc/ImageMagick-6/policy.xml

# /data holds the database, uploaded files, and optionally a .env file.
# Mount this directory as a persistent volume.
RUN mkdir -p /data/storage

COPY --from=go-build /out/damask-server /app/damask-server
COPY --from=go-build /out/damask-server-demo /app/damask-server-demo
COPY --from=go-build /out/damask-admin /app/damask-admin
COPY --from=web-build /build/web/build/ /app/web/

WORKDIR /app
EXPOSE 8080

ENV PORT=8080 \
    DB_PATH=/data/damask.db \
    STORAGE_PATH=/data/storage \
    APP_ENV=production \
    FRONTEND_PATH=/app/web

# JWT_SECRET must be supplied at runtime via environment variable.
CMD ["/app/damask-server"]
