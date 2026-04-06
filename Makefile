.PHONY: dev dev-server dev-web build test lint generate migrate admin admin-run admin-install

# Run both server and web dev servers concurrently
dev:
	$(MAKE) -j2 dev-server dev-web

# Run the Go backend
dev-server:
	cd server && air --build.cmd "go build -o bin/damask-server ./cmd/server" --build.entrypoint "./bin/damask-server" --build.include_dir internal

# Run the Svelte frontend
dev-web:
	cd web && npm run dev

# Build the Go server binary
build:
	cd server && go build -o bin/damask-server ./cmd/server

# Update Swagger docs
swagger:
	cd server && swag init -g cmd/server/main.go

# Run all tests
test:
	cd server && go test ./...

# Run linters
lint:
	cd server && golangci-lint run
	cd web && npm run check

# Run sqlc code generation
generate:
	cd server && sqlc generate -f internal/db/sqlc.yaml

# Apply DB migrations (for manual use; server auto-migrates on start)
migrate:
	cd server && go run ./cmd/server --migrate-only

## Admin TUI
admin:
	cd server && go build -ldflags="-s -w" -o bin/damask-admin ./cmd/admin

admin-run:
	cd server && go run ./cmd/admin

admin-install:
	cd server && go install ./cmd/admin
