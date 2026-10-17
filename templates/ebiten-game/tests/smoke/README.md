# Smoke tests

Playwright scripts that load the WASM preview, wait for `window.__game.ready`, then drive the game
through `tap()`, `advance(n)` and `state()`. The default boot script arrives with MVP step 5.
