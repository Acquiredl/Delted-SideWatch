.PHONY: dev build test test-integration test-e2e lint clean

dev:
	docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build

build:
	cd services/manager && go build -o ../../bin/manager ./cmd/manager/
	cd services/gateway && go build -o ../../bin/gateway ./cmd/gateway/

test:
	cd services/manager && go test -race ./...
	cd services/gateway && go test -race ./...

test-integration:
	cd services/manager && go test -race -tags integration -timeout 60s ./...

test-e2e:
	docker compose -f docker-compose.yml -f docker-compose.dev.yml -f docker-compose.test.yml up --build -d
	@echo "Waiting for services to initialize..."
	sleep 20
	cd tests/e2e && go test -v -tags e2e -timeout 120s ./...
	docker compose -f docker-compose.yml -f docker-compose.dev.yml -f docker-compose.test.yml down -v

lint:
	cd services/manager && go vet ./...
	cd services/gateway && go vet ./...

clean:
	rm -rf bin/
	docker compose down -v
