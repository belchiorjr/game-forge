"use client";

import { useEffect, useState } from "react";
import { api, ApiError, type LLMSettings, type LLMSettingsInput } from "@/lib/api/client";

const MODELS = [
  { id: "claude-opus-5-5", label: "Claude Opus 5.5 (recomendado)" },
  { id: "claude-sonnet-5-5", label: "Claude Sonnet 5.5" },
  { id: "claude-haiku-5-5", label: "Claude Haiku 5.5" },
  { id: "claude-fable-5-1", label: "Claude Fable 5.1" },
];

type Provider = LLMSettingsInput["provider"];
type Notice = { ok: boolean; text: string } | null;

export function LLMSettingsForm() {
  const [current, setCurrent] = useState<LLMSettings | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [provider, setProvider] = useState<Provider>("fake");
  const [model, setModel] = useState("claude-opus-5-5");
  const [fallback, setFallback] = useState(true);
  const [apiKey, setApiKey] = useState(""); // only what the person typed now; the stored key never comes back
  const [busy, setBusy] = useState<"save" | "test" | "remove" | null>(null);
  const [notice, setNotice] = useState<Notice>(null);

  function load(s: LLMSettings) {
    setCurrent(s);
    setProvider(s.provider);
    setModel(s.model || "claude-opus-5-5");
    setFallback(s.refusal_fallback);
  }

  useEffect(() => {
    api.getLLMSettings().then(load).catch((e: Error) => setLoadError(e.message));
  }, []);

  const input = (key?: string): LLMSettingsInput => ({
    provider,
    model,
    refusal_fallback: fallback,
    ...(key !== undefined ? { api_key: key } : apiKey ? { api_key: apiKey } : {}),
  });

  async function run(kind: "save" | "test" | "remove", fn: () => Promise<Notice>) {
    setBusy(kind);
    setNotice(null);
    try {
      setNotice(await fn());
    } catch (err) {
      setNotice({ ok: false, text: err instanceof ApiError ? err.message : "Falha na requisição" });
    } finally {
      setBusy(null);
    }
  }

  const save = (e: React.FormEvent) => {
    e.preventDefault();
    run("save", async () => {
      load(await api.saveLLMSettings(input()));
      setApiKey("");
      return { ok: true, text: "Configuração salva. Vale a partir do próximo run." };
    });
  };
  const test = () =>
    run("test", async () => {
      const r = await api.testLLMSettings(input());
      return { ok: r.ok, text: r.ok ? "Conexão OK: chave e modelo válidos." : r.message };
    });
  const remove = () =>
    run("remove", async () => {
      load(await api.saveLLMSettings(input("")));
      setApiKey("");
      return { ok: true, text: "Chave removida." };
    });

  if (loadError) return <p role="alert" className="mt-6 text-sm text-bad">Não foi possível carregar: {loadError}</p>;
  if (!current) return <p className="mt-6 text-sm text-muted">Carregando…</p>;

  const anthropic = provider === "anthropic";
  return (
    <form onSubmit={save} className="mt-6 space-y-6 rounded-xl border border-border bg-surface p-5">
      <fieldset>
        <legend className="text-sm font-medium">Provedor</legend>
        <div className="mt-2 grid gap-2 sm:grid-cols-2">
          {(
            [
              ["fake", "Agente fake", "Roteirizado, sem custo. Bom para testar a plataforma."],
              ["anthropic", "Claude (Anthropic)", "Agente real: projeta, programa e testa o jogo."],
            ] as const
          ).map(([id, title, desc]) => (
            <label
              key={id}
              className={`cursor-pointer rounded-lg border p-3 text-sm ${provider === id ? "border-accent bg-accent/5" : "border-border"}`}
            >
              <input
                type="radio"
                name="provider"
                value={id}
                checked={provider === id}
                onChange={() => setProvider(id)}
                className="sr-only"
              />
              <span className="font-medium">{title}</span>
              <span className="mt-0.5 block text-xs text-muted">{desc}</span>
            </label>
          ))}
        </div>
      </fieldset>

      <div className={anthropic ? "" : "pointer-events-none opacity-50"} aria-disabled={!anthropic}>
        <label htmlFor="api-key" className="block text-sm font-medium">
          Chave de API da Anthropic
        </label>
        <p className="mt-0.5 text-xs text-muted">
          {current.api_key_set
            ? current.api_key_source === "env"
              ? `Usando ANTHROPIC_API_KEY do servidor (${current.api_key_masked}). Salve uma aqui para substituir.`
              : `Chave salva: ${current.api_key_masked}. Deixe em branco para manter.`
            : "Nenhuma chave configurada. Crie uma em console.anthropic.com."}
        </p>
        <div className="mt-2 flex gap-2">
          <input
            id="api-key"
            type="password"
            autoComplete="off"
            spellCheck={false}
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            placeholder={current.api_key_set ? "•••••••• (mantida)" : "sk-ant-…"}
            className="min-w-0 flex-1 rounded-lg border border-border bg-background px-3 py-2 font-mono text-sm outline-none focus:border-accent"
          />
          {current.api_key_source === "settings" && (
            <button
              type="button"
              onClick={remove}
              disabled={busy !== null}
              className="rounded-lg border border-bad/50 px-3 text-sm text-bad disabled:opacity-50"
            >
              Remover
            </button>
          )}
        </div>
        <p className="mt-1 text-xs text-muted">
          A chave vai para a API Go, que guarda criptografada (AES-GCM) e nunca a devolve ao navegador.
        </p>

        <label htmlFor="model" className="mt-5 block text-sm font-medium">
          Modelo
        </label>
        <select
          id="model"
          value={MODELS.some((m) => m.id === model) ? model : "custom"}
          onChange={(e) => e.target.value !== "custom" && setModel(e.target.value)}
          className="mt-2 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm outline-none focus:border-accent"
        >
          {MODELS.map((m) => (
            <option key={m.id} value={m.id}>
              {m.label}
            </option>
          ))}
          {!MODELS.some((m) => m.id === model) && <option value="custom">{model}</option>}
        </select>

        <label className="mt-5 flex items-start gap-2 text-sm">
          <input type="checkbox" checked={fallback} onChange={(e) => setFallback(e.target.checked)} className="mt-1" />
          <span>
            Fallback em recusas
            <span className="block text-xs text-muted">
              Se o modelo recusar por política, a própria API refaz a chamada com outro modelo.
            </span>
          </span>
        </label>
      </div>

      {notice && (
        <p role="status" className={`text-sm ${notice.ok ? "text-ok" : "text-bad"}`}>
          {notice.text}
        </p>
      )}

      <div className="flex flex-wrap gap-2 border-t border-border pt-4">
        <button
          type="submit"
          disabled={busy !== null}
          className="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
        >
          {busy === "save" ? "Salvando…" : "Salvar"}
        </button>
        {anthropic && (
          <button
            type="button"
            onClick={test}
            disabled={busy !== null}
            className="rounded-lg border border-border px-4 py-2 text-sm disabled:opacity-60"
          >
            {busy === "test" ? "Testando…" : "Testar conexão"}
          </button>
        )}
      </div>
    </form>
  );
}
