"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api, type Project } from "@/lib/api/client";
import { useProjectEvents } from "@/lib/events/useProjectEvents";
import { CodePane } from "@/features/code/CodePane";
import { PreviewPane } from "@/features/preview/PreviewPane";
import { AgentPane } from "./AgentPane";

type CenterTab = "preview" | "code";

export function Workspace({ projectId }: { projectId: string }) {
  const { state, connection } = useProjectEvents(projectId);
  const [project, setProject] = useState<Project | null>(null);
  const [notFound, setNotFound] = useState(false);
  const [tab, setTab] = useState<CenterTab>("preview");

  useEffect(() => {
    api.getProject(projectId).then(setProject).catch(() => setNotFound(true));
  }, [projectId]);

  if (notFound) {
    return (
      <main className="p-8">
        <p>Projeto não encontrado.</p>
        <Link href="/" className="text-accent underline">Voltar</Link>
      </main>
    );
  }

  const running = state.activeRunId !== null;
  return (
    <div className="flex h-dvh flex-col">
      <header className="flex items-center gap-3 border-b border-border bg-surface px-4 py-2 text-sm">
        <Link href="/" className="font-semibold">
          Game <span className="text-accent">Forge</span>
        </Link>
        <span className="text-muted">/</span>
        <span className="truncate font-medium">{project?.name ?? "…"}</span>
        <span
          className={`ml-auto rounded-full px-2 py-0.5 text-xs ${running ? "bg-accent/15 text-accent" : "bg-border text-muted"}`}
        >
          {running ? "agente trabalhando" : "ocioso"}
        </span>
        <span className="flex items-center gap-1 text-xs text-muted" title="Conexão com o stream de eventos">
          <span
            className={`size-2 rounded-full ${connection === "open" ? "bg-ok" : "animate-pulse bg-accent"}`}
            aria-hidden
          />
          {connection === "open" ? "ao vivo" : connection === "connecting" ? "conectando" : "reconectando"}
        </span>
        <Link href="/settings" className="text-muted hover:text-foreground" aria-label="Configurações">
          ⚙
        </Link>
      </header>

      <div className="grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[minmax(320px,400px)_1fr]">
        <AgentPane projectId={projectId} state={state} />
        <section className="flex min-h-0 flex-col border-t border-border lg:border-t-0 lg:border-l">
          <div role="tablist" className="flex gap-1 border-b border-border bg-surface px-2 pt-2 text-sm">
            {(["preview", "code"] as const).map((t) => (
              <button
                key={t}
                role="tab"
                aria-selected={tab === t}
                onClick={() => setTab(t)}
                className={`rounded-t-md px-3 py-1.5 ${tab === t ? "bg-background font-medium" : "text-muted"}`}
              >
                {t === "preview" ? "Preview" : "Código"}
              </button>
            ))}
          </div>
          <div className="min-h-0 flex-1">
            {tab === "preview" ? (
              <PreviewPane projectId={projectId} build={state.build} running={running} />
            ) : (
              <CodePane
                projectId={projectId}
                locked={running}
                fileVersion={state.fileVersion}
                changed={state.changedFiles}
              />
            )}
          </div>
        </section>
      </div>
    </div>
  );
}
