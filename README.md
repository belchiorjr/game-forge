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
make install && make dev   # API em :8787 (agente fake) + web em :3300
```

Abra http://localhost:3300, descreva um jogo e acompanhe o agente na timeline.

Todos os comandos: `make help`.

### Docker

```bash
make up           # sobe tudo (cria .env se faltar) e espera ficar saudável
make down         # derruba tudo, mantém banco e projetos
make down-v       # derruba e limpa tudo: volumes, imagens e artefatos de build
make docker-logs
```

Portas do host via `FORGE_WEB_PORT` / `FORGE_API_PORT` no `.env` (padrão 3300/8787, só em 127.0.0.1).
Banco e workspaces ficam nos volumes `forge-data` e `forge-workspaces`. A imagem da API inclui o
toolchain Go e git porque o agente compila e testa os jogos. O serviço de sprites roda nativo no Mac
(Docker no macOS não acessa a GPU).

### Agente real (Claude)

Abra **⚙ Configurações** (`/settings`) na web, escolha "Claude (Anthropic)", cole a chave e clique em
"Testar conexão" e "Salvar". Vale a partir do próximo run, sem reiniciar nada.

A chave vai para a API Go, que guarda criptografada (AES-256-GCM) no SQLite e nunca a devolve ao
navegador (só os 4 últimos caracteres). A chave de criptografia vem de `FORGE_SECRET_KEY` ou é gerada
em `data/secret.key` na primeira execução. Alternativa: `ANTHROPIC_API_KEY` no ambiente da API.

A web não tem login: mantenha-a acessível só na sua máquina.

| Variável | Padrão | Uso |
|---|---|---|
| `FORGE_LLM` | `fake` | provedor padrão até algo ser salvo em Configurações |
| `FORGE_SECRET_KEY` | gerada | criptografa a chave salva (trocar invalida a chave guardada) |
| `FORGE_MODEL` | `claude-opus-5-5` | modelo do agente |
| `FORGE_REFUSAL_FALLBACK` | `true` | fallback server-side quando o modelo recusa |
| `FORGE_API_TOKEN` | vazio | bearer token da API (defina igual em `apps/web/.env.local`) |
| `FORGE_MAX_STEPS` / `FORGE_RUN_TIMEOUT` | `60` / `30m` | limites por run |
| `FORGE_DATA_DIR` / `FORGE_WORKSPACES_DIR` | `data/` / `workspaces/` | SQLite e projetos |

## Testes

```bash
make test         # template + API (vet, race, archtest) + web (tsc, eslint)
make e2e          # teste global: sobe API + web isoladas e roda a suíte Playwright no navegador
make test-all     # os dois
```

O e2e usa portas livres e dados temporários, então não interfere em nada que esteja rodando.
Cobre: proxy e contrato de erros, Configurações (testar/salvar/mascarar/remover chave), criar jogo,
gate do agente, jogo rodando no preview e respondendo a toque, replay da timeline, bloqueio do
editor durante o run, 409 de run duplicado, edição manual com commit, build manual e path jail.

## Status do MVP

1. ✅ Esqueleto da API, domínio, portas, SQLite, workspace com git, `/healthz`
2. ✅ `CreateProject` / `StartRun`, agente fake, stream SSE com replay, web ligada
3. ✅ Adapter Anthropic, loop, ferramentas de arquivo e build/teste em sandbox, **verify gate** (fmt, vet, build, test, build_wasm)
4. ✅ Template Ebitengine + preview WASM servido pela API e recarregado ao vivo na web a cada build
5. ⬜ Smoke test headless (Playwright) — entra no gate quando pronto
5b. ⬜ Emulador Android ao vivo no painel (AVD local, nativo no Mac)
6. ⬜ Serviço de sprites + `generate_sprite` / `view_image`
7. ⬜ Builds mobile (Android, iOS)
8. ⬜ Polimento: Monaco, diffs, revert na UI, logs, medidor de custo

O contrato HTTP está em `apps/api/api/openapi.yaml`; os tipos TS vêm dele (`make types`).
