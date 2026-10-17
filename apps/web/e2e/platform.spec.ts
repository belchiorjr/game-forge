import { expect, test, type Page } from "@playwright/test";

type GameState = { scene: string; tick: number; obstacles: number };
type GameWindow = Window & { __game: { ready: boolean; tap(): void; advance(n: number): void; state(): GameState } };

async function createProject(page: Page, name: string, idea: string) {
  await page.goto("/");
  await page.getByLabel("Descreva seu jogo").fill(idea);
  await page.getByLabel("Nome").fill(name);
  await page.getByRole("button", { name: "Forjar jogo" }).click();
  await page.waitForURL(/\/projects\/[a-f0-9]+$/);
  return page.url().split("/").pop()!;
}

async function waitRunDone(page: Page) {
  await expect(page.getByText(/run concluído/i).last()).toBeVisible({ timeout: 60_000 });
}

test.describe("api through the proxy", () => {
  test("health and settings answer from the Forge API", async ({ request }) => {
    const res = await request.get("/api/forge/settings/llm");
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body).toMatchObject({ provider: expect.any(String), api_key_set: expect.any(Boolean) });
    expect(JSON.stringify(body)).not.toContain("sk-ant");
  });

  test("errors use the contract shape", async ({ request }) => {
    const missing = await request.get("/api/forge/projects/doesnotexist");
    expect(missing.status()).toBe(404);
    expect((await missing.json()).error.code).toBe("not_found");

    const invalid = await request.post("/api/forge/projects", { data: { name: "", idea: "" } });
    expect(invalid.status()).toBe(422);
    expect((await invalid.json()).error.code).toBe("invalid");
  });
});

test.describe("settings", () => {
  test("load, test, save masked key, remove, back to fake", async ({ page, request }) => {
    await page.goto("/settings");
    await expect(page.getByRole("heading", { name: "Configurações" })).toBeVisible();
    await expect(page.getByText("Não foi possível carregar")).toHaveCount(0);

    await page.getByText("Claude (Anthropic)").click();
    await page.getByLabel("Chave de API da Anthropic").fill("sk-ant-e2e-fake-key-WXYZ");
    await page.getByRole("button", { name: "Testar conexão" }).click();
    // A fake key must never pass. Offline machines get "cannot reach" instead of "invalid".
    await expect(page.getByRole("status")).toContainText(/invalid API key|cannot reach|Anthropic returned/);

    await page.getByRole("button", { name: "Salvar" }).click();
    await expect(page.getByRole("status")).toContainText("Configuração salva");
    await expect(page.getByText("Chave salva: ••••WXYZ")).toBeVisible();

    const stored = await (await request.get("/api/forge/settings/llm")).json();
    expect(stored).toMatchObject({ provider: "anthropic", api_key_set: true, api_key_masked: "••••WXYZ", api_key_source: "settings" });

    await page.reload();
    await expect(page.getByLabel("Chave de API da Anthropic")).toHaveValue(""); // never sent back to the browser

    await page.getByRole("button", { name: "Remover" }).click();
    await expect(page.getByRole("status")).toContainText("Chave removida");

    await page.getByText("Agente fake").click();
    await page.getByRole("button", { name: "Salvar" }).click();
    await expect(page.getByRole("status")).toContainText("Configuração salva");
    expect((await (await request.get("/api/forge/settings/llm")).json()).provider).toBe("fake");
  });
});

test.describe("game creation", () => {
  test("create project, watch the agent pass the gate, play the preview", async ({ page }) => {
    const id = await createProject(page, "E2E Runner", "um runner de um botão com gatos nos telhados");

    await expect(page.getByText("ao vivo")).toBeVisible();
    await expect(page.getByRole("heading", { name: "Plano", exact: true })).toBeVisible();
    await waitRunDone(page);

    for (const step of ["gofmt", "go vet", "go build", "go test", "Build do preview"]) {
      await expect(page.getByRole("button", { name: new RegExp(`^concluído ${step}`) }).first()).toBeVisible();
    }
    await expect(page.getByText(/build ok/)).toBeVisible();

    const frame = page.frameLocator('iframe[title="Preview do jogo"]');
    await expect(frame.locator("canvas")).toBeVisible();
    const game = page.frames().find((f) => f.url().includes(`/projects/${id}/preview/`))!;
    await game.waitForFunction(() => (window as unknown as Partial<GameWindow>).__game?.ready === true);

    const before = await game.evaluate(() => (window as unknown as GameWindow).__game.state());
    expect(before.scene).toBe("menu");
    await game.evaluate(() => {
      (window as unknown as GameWindow).__game.tap();
      (window as unknown as GameWindow).__game.advance(120);
    });
    const after = await game.evaluate(() => (window as unknown as GameWindow).__game.state());
    expect(after.scene).toBe("play");
    expect(after.tick).toBeGreaterThan(0);
  });

  test("project appears on home and reopens with its full timeline", async ({ page }) => {
    const id = await createProject(page, "E2E Replay", "jogo da memória");
    await waitRunDone(page);

    await page.goto("/");
    await expect(page.getByRole("link", { name: /E2E Replay/ })).toBeVisible();
    await page.getByRole("link", { name: /E2E Replay/ }).click();
    await page.waitForURL(`**/projects/${id}`);
    await waitRunDone(page); // rebuilt from the replayed event log
  });

  test("follow-up prompt, editor lock during run, manual edit commit", async ({ page, request }) => {
    const id = await createProject(page, "E2E Editor", "nave espacial");
    await waitRunDone(page);

    await page.getByLabel("Mensagem para o agente").fill("deixe o herói verde");
    await page.getByRole("button", { name: "Enviar" }).click();
    await page.getByRole("tab", { name: "Código" }).click();
    await expect(page.getByText("o editor fica somente leitura")).toBeVisible();

    const conflict = await request.post(`/api/forge/projects/${id}/runs`, { data: { prompt: "outro" } });
    expect(conflict.status()).toBe(409);
    expect((await conflict.json()).error.code).toBe("run_active");

    await waitRunDone(page);
    await expect(page.getByText("o editor fica somente leitura")).toHaveCount(0);

    await page.getByRole("button", { name: "docs/AGENT_NOTES.md" }).click();
    const editor = page.getByLabel("Conteúdo de docs/AGENT_NOTES.md");
    await expect(editor).toContainText("Fake agent run");
    await editor.fill("# Agent notes\n\n- editado no e2e\n");
    await page.getByRole("button", { name: "Salvar" }).click();
    await expect(page.getByText(/Salvo \([0-9a-f]{7}\)/)).toBeVisible();

    const saved = await request.get(`/api/forge/projects/${id}/files/content?path=docs/AGENT_NOTES.md`);
    expect(await saved.text()).toContain("editado no e2e");
  });

  test("manual preview build and path jail", async ({ page, request }) => {
    const id = await createProject(page, "E2E Build", "corrida");
    await waitRunDone(page);

    await page.getByRole("button", { name: "Build preview" }).click();
    await expect(page.getByText("compilando").first()).toBeVisible();
    await expect(page.getByText(/build ok/)).toBeVisible();

    const escape = await request.get(`/api/forge/projects/${id}/files/content?path=../../etc/passwd`);
    expect(escape.status()).toBe(422);
    const git = await request.put(`/api/forge/projects/${id}/files/content?path=.git/config`, { data: "x" });
    expect(git.status()).toBe(422);
  });
});
