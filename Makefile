.DEFAULT_GOAL := help

API      := apps/api
WEB      := apps/web
TEMPLATE := templates/ebiten-game
BIN      := bin

.PHONY: help install dev api api-anthropic web build build-api build-web start \
        test test-api test-web template-test e2e arch lint fmt types \
        template-run template-wasm template-serve clean reset-data \
        test-all docker-build docker-up docker-down docker-logs docker-ps docker-restart docker-clean

##@ Ajuda

help: ## Lista todos os comandos
	@awk 'BEGIN {FS = ":.*##"; printf "\nUso: make \033[36m<alvo>\033[0m\n"} \
		/^[a-zA-Z0-9_-]+:.*##/ { printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)
	@echo

##@ Setup

install: ## Instala dependências (Go da API e do template, npm da web, .env.local)
	cd $(API) && go mod download
	cd $(TEMPLATE) && go mod download
	cd $(WEB) && npm install
	@test -f $(WEB)/.env.local || cp $(WEB)/.env.example $(WEB)/.env.local

##@ Desenvolvimento

dev: ## Sobe API (agente fake) + web juntos
	$(MAKE) -j2 api web

api: ## Sobe só a API em :8787 (FORGE_LLM=fake por padrão)
	cd $(API) && go run ./cmd/api

api-anthropic: ## Sobe a API com o agente real (Claude)
	cd $(API) && FORGE_LLM=anthropic go run ./cmd/api

web: ## Sobe só a web em :3300
	cd $(WEB) && npm run dev

##@ Build

build: build-api build-web ## Build de produção (API + web)

build-api: ## Compila a API em bin/forge-api
	cd $(API) && go build -o ../../$(BIN)/forge-api ./cmd/api

build-web: ## Build de produção do Next.js
	cd $(WEB) && npm run build

start: build ## Roda os builds de produção (API + web)
	$(MAKE) -j2 _start-api _start-web

_start-api:
	cd $(API) && ../../$(BIN)/forge-api

_start-web:
	cd $(WEB) && npm run start

##@ Qualidade

test: template-test test-api test-web ## Roda todos os testes e checagens

e2e: ## Teste global: sobe API + web isoladas e roda a suíte Playwright (KEEP=1 mantém logs)
	@cd $(WEB) && npx playwright install chromium >/dev/null
	./scripts/e2e.sh

test-all: test e2e ## Tudo: testes unitários + e2e

test-api: ## API: go vet + testes com -race (inclui archtest)
	cd $(API) && go vet ./... && go test -race ./...

test-web: ## Web: typecheck + lint
	cd $(WEB) && npm run typecheck && npm run lint

template-test: ## Template: vet, testes e build WASM (precisa ficar sempre verde)
	cd $(TEMPLATE) && go vet ./... && go test ./... && GOOS=js GOARCH=wasm go build -o /dev/null ./cmd/wasm

arch: ## Só o teste de arquitetura (regra de dependência)
	cd $(API) && go test ./internal/archtest/...

lint: ## Lint da web + gofmt check do Go
	@test -z "$$(cd $(API) && gofmt -l .)" || (echo "gofmt pendente em $(API)"; exit 1)
	@test -z "$$(cd $(TEMPLATE) && gofmt -l .)" || (echo "gofmt pendente em $(TEMPLATE)"; exit 1)
	cd $(WEB) && npm run lint

fmt: ## Formata o código Go (API + template)
	cd $(API) && gofmt -l -w .
	cd $(TEMPLATE) && gofmt -l -w .

types: ## Regenera os tipos TS a partir do openapi.yaml
	cd $(WEB) && npm run types

##@ Template do jogo

template-run: ## Roda o jogo template no desktop
	cd $(TEMPLATE) && go run ./cmd/desktop

template-wasm: ## Gera o build WASM do template em web/
	$(MAKE) -C $(TEMPLATE) wasm

template-serve: ## Serve o template WASM em http://localhost:8090
	$(MAKE) -C $(TEMPLATE) serve

##@ Docker

docker-build: ## Build das imagens (api + web)
	docker compose build

docker-up: ## Sobe a stack em background (portas: FORGE_WEB_PORT/FORGE_API_PORT)
	@test -f .env || cp .env.example .env
	docker compose up -d --build

docker-down: ## Para a stack (mantém volumes)
	docker compose down

docker-logs: ## Segue os logs da stack
	docker compose logs -f

docker-ps: ## Status dos containers
	docker compose ps

docker-restart: ## Reinicia a stack
	docker compose restart

docker-clean: ## APAGA containers, imagens e volumes da stack (pede confirmação)
	@read -p "Apagar volumes (banco e workspaces) do Docker? [y/N] " ok && [ "$$ok" = "y" ]
	docker compose down -v --rmi local

##@ Limpeza

clean: ## Remove artefatos de build
	rm -rf $(BIN) $(WEB)/.next $(TEMPLATE)/web/game.wasm $(TEMPLATE)/web/wasm_exec.js

reset-data: ## APAGA banco e workspaces locais (pede confirmação)
	@read -p "Apagar data/ e workspaces/? [y/N] " ok && [ "$$ok" = "y" ]
	rm -rf data workspaces/*
