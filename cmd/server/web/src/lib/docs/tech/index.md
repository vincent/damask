# Self-hosting guide

Everything you need to run Damask on your own infrastructure.

## Sections

- [Installation](installation) - Docker, binary, reverse proxy setup
- [Configuration](configuration) - All environment variables and their defaults
- [Storage](storage) - Local disk, S3-compatible, SFTP backends
- [Upgrading](upgrade) - Safe upgrade procedure and migration notes
- [OIDC / SSO](oidc) - Single sign-on with Keycloak, Authelia, Authentik
- [Ingress sources](ingress) - Automated ingest from email, SFTP, S3, Google Drive, Canva

## Requirements

- A Linux host (x86_64 or arm64)
- At least 512 MB RAM, 1 CPU core
- A writable directory for the SQLite database and file storage (or S3 credentials)
- Optionally: a reverse proxy (Caddy recommended) for TLS
