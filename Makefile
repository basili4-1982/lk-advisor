.PHONY: build run test fmt vet check

BIN := bin/lk-advisor

build:
	go build -trimpath -o $(BIN) ./cmd/lk-advisor

run:
	go run ./cmd/lk-advisor

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

check: fmt vet test build
