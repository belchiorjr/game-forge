# game-forge

Plataforma onde você descreve um jogo mobile e um agente de IA projeta, programa (Go + Ebitengine),
testa e mostra o resultado num cockpit web.

```
apps/web (Next.js)  --REST + SSE-->  apps/api (Go, Clean Architecture)
                                        |- use cases: projects, runs, files
                                        |- agent loop (LLM + typed tools)
                                        |- adapters: SQLite, workspace FS + git, event bus, LLM
                                        v
                         workspaces/<projectId>/   (cada jogo é um repo git; cada run é um commit)

templates/ebiten-game   (jogo inicial copiado para todo projeto novo)
apps/sprites            (serviço Python de sprites, nativo no Mac — ainda não implementado)
```

## Rodando

Requisitos: Go 1.25+, Node 20+, git.

```bash
cd apps/web && npm install && cp .env.example .env.local && cd ../..
make dev          # API em :8080 (agente fake) + web em :3000
```

Abra http://localhost:3000, descreva um jogo e acompanhe o agente na timeline.

### Agente real (Claude)

```bash
export ANTHROPIC_API_KEY=...        # ou `ant auth login`
FORGE_LLM=anthropic make api
```

| Variável | Padrão | Uso |
|---|---|---|
| `FORGE_LLM` | `fake` | `fake` (roteirizado, sem custo) ou `anthropic` |
| `FORGE_MODEL` | `claude-opus-5-5` | modelo do agente |
| `FORGE_REFUSAL_FALLBACK` | `true` | fallback server-side quando o modelo recusa |
| `FORGE_API_TOKEN` | vazio | bearer token da API (defina igual em `apps/web/.env.local`) |
| `FORGE_MAX_STEPS` / `FORGE_RUN_TIMEOUT` | `60` / `30m` | limites por run |
| `FORGE_DATA_DIR` / `FORGE_WORKSPACES_DIR` | `data/` / `workspaces/` | SQLite e projetos |

## Testes

```bash
make test         # template + API (vet, race, archtest) + web (tsc, eslint)
```

## Status do MVP

1. ✅ Esqueleto da API, domínio, portas, SQLite, workspace com git, `/healthz`
2. ✅ `CreateProject` / `StartRun`, agente fake, stream SSE com replay, web ligada
3. 🟡 Adapter Anthropic + loop + ferramentas de arquivo prontos; **falta o verify gate** (go_build/go_test/build_wasm/smoke_test via sandbox)
4. 🟡 Template Ebitengine compila p/ desktop e WASM; **falta servir o preview** pela API
5. ⬜ Smoke test headless (Playwright)
6. ⬜ Serviço de sprites + `generate_sprite` / `view_image`
7. ⬜ Builds mobile (Android, iOS)
8. ⬜ Polimento: Monaco, diffs, revert na UI, logs, medidor de custo

O contrato HTTP está em `apps/api/api/openapi.yaml`; os tipos TS vêm dele (`make types`).
