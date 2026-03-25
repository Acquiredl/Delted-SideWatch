.PHONY: dev build test lint clean tor-hostname

dev:
	docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build

build:
	cd services/manager && go build -o ../../bin/manager ./cmd/manager/
	cd services/gateway && go build -o ../../bin/gateway ./cmd/gateway/

test:
	cd services/manager && go test -race ./...
	cd services/gateway && go test -race ./...

lint:
	cd services/manager && go vet ./...
	cd services/gateway && go vet ./...

clean:
	rm -rf bin/
	docker compose down -v

tor-hostname:  ## Show the .onion address
	docker compose exec tor cat /var/lib/tor/hidden_service/hostname
