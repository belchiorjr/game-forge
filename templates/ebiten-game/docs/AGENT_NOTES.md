# Agent notes

Decisions, gotchas, how to run tests, known issues. Keep it short and current.

- Tests: `go vet ./... && go test ./...`. WASM preview: `make wasm`.
- `internal/logic` must stay free of Ebitengine imports; `TestStepDoesNotAllocate` guards the game loop.
