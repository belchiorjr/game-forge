"use client";

import { useEffect, useRef, useState } from "react";
import { api, ApiError } from "@/lib/api/client";
import type { TimelineItem, TimelineState } from "@/lib/events/reducer";

const TOOL_LABEL: Record<string, string> = {
  list_files: "Listou arquivos",
  read_file: "Leu",
  write_file: "Escreveu",
  edit_file: "Editou",
  update_plan: "Atualizou o plano",
  note: "Anotou em AGENT_NOTES.md",
  finish: "Concluiu",
  go_fmt: "gofmt",
  go_vet: "go vet",
  go_build: "go build",
  go_test: "go test",
  build_wasm: "Build do preview",
};

function toolLine(name: string, path: string) {
  return `${TOOL_LABEL[name] ?? name}${path ? ` ${path}` : ""}`;
}

function Item({ it }: { it: TimelineItem }) {
  const [open, setOpen] = useState(false);
  switch (it.kind) {
    case "user":
      return <div className="ml-8 rounded-lg bg-accent/10 px-3 py-2 text-sm">{it.text}</div>;
    case "agent":
      return <div className="whitespace-pre-wrap text-sm">{it.text}</div>;
    case "tool":
      return (
        <div className="text-xs">
          <button
            onClick={() => setOpen(!open)}
            aria-expanded={open}
            className="flex w-full items-center gap-2 text-left text-muted hover:text-foreground"
          >
            <span className={it.ok === undefined ? "animate-pulse" : it.ok ? "text-ok" : "text-bad"} aria-hidden>
              {it.ok === undefined ? "●" : it.ok ? "✓" : "✗"}
            </span>
            <span className="sr-only">{it.ok === undefined ? "rodando" : it.ok ? "concluído" : "falhou"}</span>
            <span className="truncate font-mono">{toolLine(it.name, it.path)}</span>
            {it.durationMs !== undefined && <span className="ml-auto shrink-0">{it.durationMs} ms</span>}
          </button>
          {open && it.output && (
            <pre className="mt-1 max-h-48 overflow-auto rounded bg-background p-2 font-mono text-[11px]">{it.output}</pre>
          )}
        </div>
      );
    case "finished":
      return (
        <div
          className={`rounded-lg border px-3 py-2 text-sm ${it.status === "succeeded" ? "border-ok/40" : "border-bad/40"}`}
        >
          <div className="text-xs font-medium uppercase tracking-wide text-muted">
            run {it.status === "succeeded" ? "concluído" : it.status === "cancelled" ? "cancelado" : "falhou"}
            {it.commit && <span className="ml-2 font-mono normal-case">{it.commit.slice(0, 7)}</span>}
          </div>
          <div className="mt-1 whitespace-pre-wrap">{it.summary}</div>
        </div>
      );
    case "error":
      return <div className="text-sm text-bad">⚠ {it.message}</div>;
  }
}

export function AgentPane({ projectId, state }: { projectId: string; state: TimelineState }) {
  const [prompt, setPrompt] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [sending, setSending] = useState(false);
  const bottom = useRef<HTMLDivElement>(null);
  const running = state.activeRunId !== null;

  useEffect(() => {
    bottom.current?.scrollIntoView({ block: "end" });
  }, [state.items.length]);

  async function send(e: React.FormEvent) {
    e.preventDefault();
    setSending(true);
    setError(null);
    try {
      await api.startRun(projectId, prompt, crypto.randomUUID());
      setPrompt("");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Falha ao iniciar o run");
    } finally {
      setSending(false);
    }
  }

  async function stop() {
    if (!state.activeRunId) return;
    try {
      await api.cancelRun(projectId, state.activeRunId);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Falha ao cancelar");
    }
  }

  const tokens = state.usage.input_tokens + state.usage.output_tokens;
  return (
    <aside className="flex min-h-0 flex-col bg-surface">
      {state.plan.length > 0 && (
        <div className="border-b border-border px-4 py-3">
          <h2 className="mb-1 text-xs font-medium uppercase tracking-wide text-muted">Plano</h2>
          <ul className="space-y-0.5 text-sm">
            {state.plan.map((p, i) => (
              <li key={i} className={p.done ? "text-muted line-through" : ""}>
                {p.done ? "☑" : "☐"} {p.text}
              </li>
            ))}
          </ul>
        </div>
      )}

      <div className="min-h-0 flex-1 space-y-3 overflow-y-auto px-4 py-4" aria-live="polite">
        {state.items.length === 0 ? (
          <p className="text-sm text-muted">Nenhuma atividade ainda. Peça algo ao agente abaixo.</p>
        ) : (
          state.items.map((it) => <Item key={it.seq} it={it} />)
        )}
        <div ref={bottom} />
      </div>

      <form onSubmit={send} className="border-t border-border p-3">
        {running && (
          <div className="mb-2 flex items-center justify-between text-xs text-muted">
            <span>{tokens.toLocaleString("pt-BR")} tokens neste run</span>
            <button type="button" onClick={stop} className="rounded border border-bad/50 px-2 py-0.5 text-bad">
              Parar
            </button>
          </div>
        )}
        <label htmlFor="prompt" className="sr-only">
          Mensagem para o agente
        </label>
        <textarea
          id="prompt"
          rows={3}
          value={prompt}
          disabled={running}
          onChange={(e) => setPrompt(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) e.currentTarget.form?.requestSubmit();
          }}
          placeholder={running ? "O agente está trabalhando; aguarde o fim do run." : "Ex.: adicione um pulo duplo"}
          className="w-full resize-none rounded-lg border border-border bg-background p-2 text-sm outline-none focus:border-accent disabled:opacity-60"
        />
        <div className="mt-2 flex items-center justify-between">
          {error ? <p role="alert" className="text-xs text-bad">{error}</p> : <span className="text-xs text-muted">⌘/Ctrl + Enter</span>}
          <button
            type="submit"
            disabled={running || sending || !prompt.trim()}
            className="rounded-lg bg-accent px-3 py-1.5 text-sm font-medium text-white disabled:opacity-50"
          >
            Enviar
          </button>
        </div>
      </form>
    </aside>
  );
}
