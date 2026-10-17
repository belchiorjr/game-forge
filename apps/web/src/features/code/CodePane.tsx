"use client";

import { useEffect, useState } from "react";
import { api, ApiError, type FileInfo } from "@/lib/api/client";

type Props = { projectId: string; locked: boolean; fileVersion: number; changed: string[] };

export function CodePane({ projectId, locked, fileVersion, changed }: Props) {
  const [files, setFiles] = useState<FileInfo[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [content, setContent] = useState("");
  const [saved, setSaved] = useState("");
  const [status, setStatus] = useState<string | null>(null);

  // Refetch the tree whenever the agent changes a file.
  useEffect(() => {
    api.listFiles(projectId).then((fs) => setFiles(fs.filter((f) => !f.is_dir))).catch(() => setFiles([]));
  }, [projectId, fileVersion]);

  // Reload the open file on agent changes, unless the person has unsaved edits.
  useEffect(() => {
    if (!selected) return;
    api
      .readFile(projectId, selected)
      .then((text) => {
        setSaved(text);
        setContent((cur) => (cur !== saved && cur !== "" ? cur : text));
      })
      .catch(() => setStatus("Não foi possível abrir o arquivo"));
    // eslint-disable-next-line react-hooks/exhaustive-deps -- `saved` is read only to detect dirty state
  }, [projectId, selected, fileVersion]);

  function open(path: string) {
    setSelected(path);
    setContent("");
    setSaved("");
    setStatus(null);
  }

  async function save() {
    if (!selected) return;
    try {
      const { commit } = await api.writeFile(projectId, selected, content);
      setSaved(content);
      setStatus(`Salvo (${commit.slice(0, 7)})`);
    } catch (err) {
      setStatus(err instanceof ApiError ? err.message : "Falha ao salvar");
    }
  }

  const dirty = content !== saved;
  return (
    <div className="grid h-full min-h-[320px] grid-cols-[minmax(160px,240px)_1fr]">
      <nav aria-label="Arquivos" className="overflow-y-auto border-r border-border bg-surface py-2 text-xs">
        {files.length === 0 && <p className="px-3 text-muted">Sem arquivos.</p>}
        {files.map((f) => (
          <button
            key={f.path}
            onClick={() => open(f.path)}
            className={`block w-full truncate px-3 py-1 text-left font-mono ${selected === f.path ? "bg-accent/15" : "hover:bg-background"}`}
            title={f.path}
          >
            {changed.includes(f.path) && <span className="mr-1 text-accent" aria-label="alterado pelo agente">●</span>}
            {f.path}
          </button>
        ))}
      </nav>
      <div className="flex min-h-0 flex-col">
        {locked && (
          <div className="border-b border-border bg-accent/10 px-3 py-1.5 text-xs text-accent">
            O agente está trabalhando neste projeto; o editor fica somente leitura até o fim do run.
          </div>
        )}
        {selected ? (
          <>
            <div className="flex items-center gap-2 border-b border-border px-3 py-1.5 text-xs">
              <span className="truncate font-mono">{selected}</span>
              {status && <span className="text-muted">{status}</span>}
              <button
                onClick={save}
                disabled={locked || !dirty}
                className="ml-auto rounded bg-accent px-2 py-0.5 text-white disabled:opacity-40"
              >
                Salvar
              </button>
            </div>
            <textarea
              aria-label={`Conteúdo de ${selected}`}
              value={content}
              readOnly={locked}
              onChange={(e) => setContent(e.target.value)}
              spellCheck={false}
              className="min-h-0 flex-1 resize-none bg-background p-3 font-mono text-xs leading-relaxed outline-none"
            />
          </>
        ) : (
          <p className="p-4 text-sm text-muted">Escolha um arquivo à esquerda.</p>
        )}
      </div>
    </div>
  );
}
