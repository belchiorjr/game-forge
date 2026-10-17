.PHONY: run test wasm serve

run:
	go run ./cmd/desktop

test:
	go vet ./... && go test ./...

wasm:
	GOOS=js GOARCH=wasm go build -o web/game.wasm ./cmd/wasm
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" web/ 2>/dev/null || cp "$$(go env GOROOT)/misc/wasm/wasm_exec.js" web/

serve: wasm
	cd web && python3 -m http.server 8090
