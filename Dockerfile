# ── Web build ─────────────────────────────────────────────────────────────────
FROM node:24-bookworm-slim AS web-build
WORKDIR /build
COPY cmd/server/web/package.json cmd/server/web/package-lock.json ./
RUN npm ci
COPY cmd/server/web/ .
RUN VITE_API_URL="" npm run build

# ── Go build ──────────────────────────────────────────────────────────────────
FROM golang:1.25-bookworm AS go-build
WORKDIR /build
COPY go.mod go.sum ./
COPY --from=web-build /build/build cmd/server/web/build
COPY cmd cmd/
COPY internal internal/
RUN CGO_ENABLED=0 go build -mod=mod -trimpath -ldflags="-s -w" -o /out/damask-server ./cmd/server
RUN CGO_ENABLED=0 go build -tags=demo -mod=mod -trimpath -ldflags="-s -w" -o /out/damask-server-demo ./cmd/server
RUN CGO_ENABLED=0 go build -mod=mod -trimpath -ldflags="-s -w" -o /out/damask-admin ./cmd/admin

# ── Runtime ───────────────────────────────────────────────────────────────────
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates ffmpeg ghostscript imagemagick && rm -rf /var/lib/apt/lists/*

# remove some policies from imagemagick config
RUN sed -i \
    -e '/disable ghostscript format types/,+6d' \
    -e '/name="width"/d' \
    -e '/name="height"/d' \
    -e '/domain="path"/d' \
    /etc/ImageMagick-6/policy.xml

# /data holds the database, uploaded files, and optionally a .env file.
# Mount this directory as a persistent volume.
RUN mkdir -p /data/storage

COPY --from=go-build /out/damask-server /app/damask-server
COPY --from=go-build /out/damask-server-demo /app/damask-server-demo
COPY --from=go-build /out/damask-admin /app/damask-admin

WORKDIR /app
EXPOSE 8080

ENV PORT=8080 \
    MAIL_PORT=2525 \
    DB_PATH=/data/damask.db \
    STORAGE_LOCAL_PATH=/data/storage \
    APP_ENV=production

# JWT_SECRET must be supplied at runtime via environment variable.
CMD ["/app/damask-server"]
