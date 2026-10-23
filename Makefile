# be-bumn-academy — perintah umum pengembangan
SHELL := /bin/bash

API         ?= http://127.0.0.1:8080/api/v1
USERS       ?= 200
CONCURRENCY ?= 50

BINARY      := bin/api
MIGRATE_BIN := bin/migrate
SEED_BIN    := bin/seed

.PHONY: help tidy fmt vet build run migrate-up migrate-down migrate-status seed test smoke loadtest ai-test docker-up docker-down docker-logs clean

help: ## Tampilkan daftar perintah
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

tidy: ## Rapikan dependensi
	go mod tidy

fmt: ## Format kode
	gofmt -s -w .

vet: ## Analisis statis
	go vet ./...

build: ## Build semua binary
	@mkdir -p bin
	go build -trimpath -ldflags "-s -w" -o $(BINARY) ./cmd/api
	go build -trimpath -ldflags "-s -w" -o $(MIGRATE_BIN) ./cmd/migrate
	go build -trimpath -ldflags "-s -w" -o $(SEED_BIN) ./cmd/seed

run: ## Jalankan API (migrasi + seed otomatis sesuai .env)
	go run ./cmd/api

migrate-up: ## Jalankan semua migrasi
	go run ./cmd/migrate up

migrate-down: ## Rollback 1 migrasi terakhir
	go run ./cmd/migrate down

migrate-status: ## Lihat status migrasi
	go run ./cmd/migrate status

seed: ## Isi data demo (idempotent)
	go run ./cmd/seed

test: ## Jalankan unit test
	go test ./... -count=1

docker-up: ## Nyalakan postgres + redis + api
	docker compose up -d --build

docker-down: ## Matikan semua container
	docker compose down

docker-logs: ## Lihat log api
	docker compose logs -f api

smoke: ## Smoke test end-to-end API (butuh API hidup)
	./scripts/smoke-test.sh $(API)

loadtest: ## Uji tryout serentak (butuh API hidup)
	go run ./scripts/loadtest.go -users $(USERS) -concurrency $(CONCURRENCY)

ai-test: ## Uji mentor AI pada ulasan hasil tryout (butuh API hidup)
	./scripts/ai-mentor-test.sh $(API)

clean: ## Bersihkan artefak build
	rm -rf bin coverage
