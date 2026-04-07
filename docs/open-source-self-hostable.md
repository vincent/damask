---
outline: deep
---

# Open Source & Self-Hostable

Damask is Apache-2 licensed and ships as a single, self-contained binary. There are no runtime dependencies to install, no database server to manage, and no cloud service to subscribe to. Run it on your laptop, your home server, or a VPS - the experience is identical.

## License

Damask is released under the **Apache License 2.0**. You can use it, modify it, self-host it, and distribute it - commercially or otherwise - with few restrictions. The full license text is in the `LICENSE` file at the root of the repository.

This means:
- Self-hosting is always free, for any number of users and assets
- You can fork the project and customise it for your studio or team
- You can build commercial products on top of Damask
- You are not required to contribute changes back (though it's appreciated)
- You have to include the original copyright notice
- You have to disclose any significant changes made to the original code
- You are not granted rights to use the trademarks, logos, names of Damask software, or logos of the licensor (Apache Software Foundation)

## The single binary

Damask's Go backend compiles to a single static binary with no shared library dependencies. The SvelteKit frontend is embedded inside it at build time.

What you download is one file. Running it starts the full application - HTTP server, background job workers, file watcher, and a self-contained SQLite database layer.

```bash
./damask          # starts everything
./damask --help   # shows all flags
./damask --version
```

There is no `npm install`, no `pip install`, no system packages to configure. If the binary runs, Damask runs.

### Binary size

The release binary is approximately 25–35 MB depending on platform. It includes the compiled Go server and the embedded frontend assets.

### Platforms

Pre-built binaries are available for:

| Platform | Architecture |
|----------|-------------|
| Linux | amd64, arm64 |
| macOS | amd64 (Intel), arm64 (Apple Silicon) |
| Windows | amd64 |

## No infrastructure required

Damask uses **SQLite** as its database. SQLite is a file, not a server. There is nothing to install, configure, or monitor. The database lives at `./damask.db` by default and works correctly under high read concurrency thanks to WAL journal mode.

Everything is handled in-process. The job queue is SQLite-backed. Full-text search uses SQLite FTS5. Sessions are stateless tokens.

This keeps the operational burden minimal. A single `systemctl restart damask` command is all you need to update or recover.

## Optional dependencies

As of current version, Damask expects Ffmpeg and ImageMagick binaries to be installed and accessible in $PATH. A future release is planned to include them in the binary.

## Contributing

The source code is at `https://github.com/vincent/damask`.

### Getting started for development

```bash
git clone https://github.com/vincent/damask
cd damask

# Start the Go backend and SvelteKit frontend, both in watch mode
make dev
```

Both processes run in parallel. The frontend dev server proxies API requests to the Go server.

### Project structure

```
damask/
├── server/         ← Go backend
│   ├── cmd/server/ ← entry point
│   └── internal/
│       ├── api/        ← Fiber HTTP handlers
│       ├── db/         ← sqlc generated queries + migrations
│       ├── storage/    ← storage abstraction + local/S3 implementations
│       ├── transform/  ← image/video processing pipeline
│       ├── queue/      ← in-process job queue
│       ├── ingress/    ← ingestion sources
│       ├── demo/       ← demo seeder + reset loop
│       └── config/     ← environment config
└── web/            ← SvelteKit frontend
    └── src/
        ├── lib/
        │   ├── api/    ← typed API client
        │   └── stores/ ← Svelte stores
        └── routes/     ← pages and layouts
```

### Running tests

```bash
make test           # Go unit tests
make test-web       # SvelteKit component tests
make test-e2e       # end-to-end (requires a running instance)
```

### Submitting a pull request

1. Open an issue first for non-trivial changes so we can discuss approach
2. Fork the repository and create a feature branch
3. Write tests for new behaviour
4. Run `make lint` and fix any warnings
5. Open a pull request against `main`

---

## Roadmap

The public roadmap is maintained as GitHub Issues with milestone labels. Planned features include:

- Desktop app via Tauri (embedded server, folder watching, system tray)
- S3 remote sync for the storage backend
- Additional ingress sources (Google Drive, Dropbox)
- Whisper-based automatic subtitle extraction for video
- AI auto-tagging (local model, no API key required)
- Custom branding on client share galleries
- Per-project share permissions and team roles

If a feature you need is missing, open a GitHub Issue or start a Discussion. The roadmap is shaped by what users actually ask for.
