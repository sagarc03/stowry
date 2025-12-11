.PHONY: build test lint fmt check clean tidy docker-build docker-test

build:
	go build -o bin/stowry ./cmd/stowry

test:
	go test ./...

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

check: fmt lint test

clean:
	rm -rf bin/

tidy:
	go mod tidy

docker-build:
	docker build -t stowry:test .

docker-test: docker-build
	docker run --rm -p 5708:5708 -e STOWRY_DATABASE_TYPE=sqlite -e STOWRY_STORAGE_PATH=/data stowry:test serve --db-dsn :memory:
