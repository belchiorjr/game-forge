.PHONY: dev api web test arch fmt types template-test

# Run API (fake agent unless FORGE_LLM=anthropic) and web together.
dev:
	$(MAKE) -j2 api web

api:
	cd apps/api && go run ./cmd/api

web:
	cd apps/web && npm run dev

test: template-test
	cd apps/api && go vet ./... && go test -race ./...
	cd apps/web && npm run typecheck && npm run lint

arch:
	cd apps/api && go test ./internal/archtest/...

fmt:
	cd apps/api && gofmt -l -w .
	cd templates/ebiten-game && gofmt -l -w .

# Regenerate web types from the OpenAPI contract.
types:
	cd apps/web && npm run types

# The template must always build and pass untouched: every project starts from it.
template-test:
	cd templates/ebiten-game && go vet ./... && go test ./... && GOOS=js GOARCH=wasm go build -o /dev/null ./cmd/wasm
