VERSION ?= dev
BIN     := bin/scopex
LDFLAGS := -s -w -X github.com/expvintl/scopex/internal/version.Version=$(VERSION)

.PHONY: all frontend backend build run test vet tidy clean

all: build

frontend:
	cd frontend && npm install && npm run build

backend:
	cd backend && go build -trimpath -ldflags "$(LDFLAGS)" -o ../$(BIN) ./cmd/server

build: frontend backend

run: build
	SCOPEX_ADMIN_PASS=$${SCOPEX_ADMIN_PASS:-admin12345} ./$(BIN)

test:
	cd backend && go test ./...

vet:
	cd backend && go vet ./...

tidy:
	cd backend && go mod tidy

clean:
	rm -rf bin dist backend/web/dist/assets backend/web/dist/index.html
