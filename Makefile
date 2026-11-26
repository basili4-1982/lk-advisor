.PHONY: build run test fmt vet check db-up db-down migrate seed eval demo data-check

BIN := bin/lk-advisor
DSN ?= postgres://lk:lk@localhost:55432/lk?sslmode=disable

build:
	go build -trimpath -o $(BIN) ./cmd/lk-advisor

run:
	go run ./cmd/lk-advisor serve

test:
	go test -p 1 ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

check: fmt vet test build

db-up:
	docker compose up -d postgres

db-down:
	docker compose down

migrate:
	DATABASE_URL='$(DSN)' go run ./cmd/lk-advisor migrate

seed:
	DATABASE_URL='$(DSN)' go run ./cmd/lk-advisor seed --seed 42

eval:
	DATABASE_URL='$(DSN)' go run ./cmd/lk-advisor eval

demo:
	./docs/demo.sh

data-check:
	./analysis/check_data_hygiene.sh
