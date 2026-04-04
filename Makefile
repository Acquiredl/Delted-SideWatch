.PHONY: dev build test lint security clean

dev:
	docker compose -f docker-compose.yml -f infra/compose/docker-compose.dev.yml up --build

build:
	cd services/manager && go build -o ../../bin/manager ./cmd/manager/
	cd services/gateway && go build -o ../../bin/gateway ./cmd/gateway/

test:
	cd services/manager && go test -race ./...
	cd services/gateway && go test -race ./...

lint:
	cd services/manager && golangci-lint run ./...
	cd services/gateway && golangci-lint run ./...

security:
	cd services/manager && govulncheck ./...
	cd services/gateway && govulncheck ./...
	cd frontend && npm audit --audit-level=high

clean:
	rm -rf bin/
	docker compose down -v

# tor-hostname:  (disabled — Tor not yet implemented)
