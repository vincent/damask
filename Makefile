VERSION := $(shell git describe --tags --always 2>/dev/null || echo dev)

.PHONY: dev dev-server dev-web build build-web build-demo build-dev desktop desktop-dev test test-integration lint generate migrate admin admin-run admin-install swagger

# Run both server and web dev servers concurrently
dev:
	$(MAKE) -j2 dev-server dev-web

# Run the Go backend with dev build tag (uses Vite proxy, no embedded SPA)
dev-server:
	air --build.cmd "go build -tags=dev,demo -o bin/damask-server-demo ./cmd/server" --build.entrypoint "./bin/damask-server-demo" --build.include_dir cmd,internal --build.exclude_dir cmd/server/web --build.stop_on_error "true"

# Run the Svelte frontend
dev-web:
	cd cmd/server/web && npm run dev

# Build the frontend (SvelteKit)
build-web:
	cd cmd/server/web && npm install && npm run build

# Build the Go server binary (includes embedded SPA)
build: build-web
	/usr/local/go/bin/go build -ldflags="-X damask/server/internal/api.Version=$(VERSION)" -o bin/damask-server ./cmd/server

# Build the Go server binary with demo jobs
build-demo: build-web
	/usr/local/go/bin/go build -tags=demo -ldflags="-X damask/server/internal/api.Version=$(VERSION)" -o bin/damask-server-demo ./cmd/server

# Build the Go server binary
build-dev: build-web
	/usr/local/go/bin/go build -ldflags="-X damask/server/internal/api.Version=$(VERSION)" -o bin/damask-server-dev ./cmd/server

desktop: build-web
	rm -rf cmd/desktop/frontend/dist
	mkdir -p cmd/desktop/frontend
	cp -R cmd/server/web/build cmd/desktop/frontend/dist
	mkdir -p bin
	/usr/local/go/bin/go build -tags=desktop -ldflags="-X main.version=$(VERSION) -X damask/server/internal/api.Version=$(VERSION)" -o bin/damask-desktop ./cmd/desktop

desktop-dev:
	wails3 dev -config ./build/config.yml

# Update Swagger docs
swagger:
	swag init -g cmd/server/main.go -o internal/docs

# Run all tests (excludes integration tests)
test:
	go test ./...

# Run integration tests (internal/api handlers against real SQLite)
test-integration:
	go test -tags=integration ./internal/api/...

# Run linters
lint:
	golangci-lint run --config .golangci.yaml
	cd cmd/server/web && npm run check

# Run sqlc code generation
generate:
	sqlc generate -f internal/db/sqlc.yaml

# Apply DB migrations (for manual use; server auto-migrates on start)
migrate:
	go run ./cmd/server --migrate-only

## Admin TUI
admin:
	go build -ldflags="-s -w" -o bin/damask-admin ./cmd/admin

admin-run:
	go run ./cmd/admin

admin-install:
	go install ./cmd/admin
