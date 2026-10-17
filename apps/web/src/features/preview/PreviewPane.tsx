"use client";

import { useEffect, useState } from "react";
import { api, ApiError } from "@/lib/api/client";
import type { BuildState } from "@/lib/events/reducer";

const DEVICES = [
  { id: "phone", label: "Celular 360×640", w: 360, h: 640 },
  { id: "tall", label: "Celular alto 360×780", w: 360, h: 780 },
  { id: "tablet", label: "Tablet 600×900", w: 600, h: 900 },
];

type Props = { projectId: string; build: BuildState; running: boolean };

export function PreviewPane({ projectId, build, running }: Props) {
  const [exists, setExists] = useState<boolean | null>(null);
  const [device, setDevice] = useState(DEVICES[0]);
  const [landscape, setLandscape] = useState(false);
  const [reloads, setReloads] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [showLog, setShowLog] = useState(false);

  // Check for a build from an earlier session; afterwards build events keep us up to date.
  useEffect(() => {
    api.hasPreview(projectId).then(setExists).catch(() => setExists(false));
  }, [projectId, build.version]);

  async function buildNow() {
    setError(null);
    try {
      await api.startBuild(projectId);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Falha ao iniciar o build");
    }
  }

  const w = landscape ? device.h : device.w;
  const h = landscape ? device.w : device.h;
  // The version in the URL busts caches and remounts the iframe after every successful build.
  const src = `/api/forge/projects/${projectId}/preview/index.html?v=${build.version}-${reloads}`;
  const building = build.status === "building";

  return (
    <div className="flex h-full min-h-[420px] flex-col">
      <div className="flex flex-wrap items-center gap-2 border-b border-border px-3 py-2 text-xs">
        <label className="sr-only" htmlFor="device">Dispositivo</label>
        <select
          id="device"
          value={device.id}
          onChange={(e) => setDevice(DEVICES.find((d) => d.id === e.target.value) ?? DEVICES[0])}
          className="rounded border border-border bg-background px-2 py-1"
        >
          {DEVICES.map((d) => (
            <option key={d.id} value={d.id}>{d.label}</option>
          ))}
        </select>
        <button onClick={() => setLandscape(!landscape)} className="rounded border border-border px-2 py-1" aria-pressed={landscape}>
          ⟲ Girar
        </button>
        <button onClick={() => setReloads(reloads + 1)} className="rounded border border-border px-2 py-1">
          ↻ Recarregar
        </button>
        <span className="ml-auto flex items-center gap-2">
          {build.status !== "idle" && (
            <button
              onClick={() => setShowLog(!showLog)}
              className={building ? "animate-pulse text-accent" : build.status === "ok" ? "text-ok" : "text-bad"}
            >
              {building ? "compilando…" : build.status === "ok" ? `build ok (${(build.durationMs / 1000).toFixed(1)}s)` : "build falhou (ver log)"}
            </button>
          )}
          <button
            onClick={buildNow}
            disabled={running || building}
            title={running ? "O agente compila o preview durante o run" : undefined}
            className="rounded bg-accent px-2 py-1 font-medium text-white disabled:opacity-50"
          >
            Build preview
          </button>
        </span>
      </div>
      {error && <p role="alert" className="px-3 pt-2 text-xs text-bad">{error}</p>}
      {showLog && build.output && (
        <pre className="max-h-40 overflow-auto border-b border-border bg-background p-2 font-mono text-[11px]">{build.output}</pre>
      )}

      <div className="flex min-h-0 flex-1 items-center justify-center overflow-auto p-4">
        <div
          className="relative shrink-0 overflow-hidden rounded-[2rem] border-[10px] border-neutral-800 bg-black shadow-xl"
          style={{ width: w + 20, height: h + 20, maxHeight: "100%", aspectRatio: `${w + 20} / ${h + 20}` }}
        >
          {exists ? (
            <iframe
              key={src}
              src={src}
              title="Preview do jogo"
              sandbox="allow-scripts allow-same-origin"
              className="h-full w-full border-0"
            />
          ) : (
            <div className="flex h-full flex-col items-center justify-center p-6 text-center text-sm text-neutral-400">
              {exists === null ? "Carregando…" : building ? "Compilando o primeiro build…" : "Nenhum build ainda. O agente gera um a cada run, ou clique em Build preview."}
            </div>
          )}
          {building && exists && (
            <div className="pointer-events-none absolute inset-x-0 top-0 bg-accent/90 py-1 text-center text-[11px] text-white">
              compilando nova versão…
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
