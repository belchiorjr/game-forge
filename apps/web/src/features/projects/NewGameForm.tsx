"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { api, ApiError } from "@/lib/api/client";

export function NewGameForm() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [idea, setIdea] = useState("");
  const [orientation, setOrientation] = useState<"portrait" | "landscape">("portrait");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const project = await api.createProject({ name, idea, orientation });
      await api.startRun(project.id, idea, crypto.randomUUID());
      router.push(`/projects/${project.id}`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Falha ao criar o projeto");
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="rounded-xl border border-border bg-surface p-4 sm:p-5">
      <label htmlFor="idea" className="block text-sm font-medium">
        Descreva seu jogo
      </label>
      <textarea
        id="idea"
        required
        rows={4}
        value={idea}
        onChange={(e) => setIdea(e.target.value)}
        placeholder="Um endless runner de um botão: um gato pula telhados à noite, coleta peixes e evita chaminés."
        className="mt-2 w-full resize-y rounded-lg border border-border bg-background p-3 text-sm outline-none focus:border-accent"
      />
      <div className="mt-3 flex flex-col gap-3 sm:flex-row sm:items-end">
        <div className="flex-1">
          <label htmlFor="name" className="block text-sm font-medium">
            Nome
          </label>
          <input
            id="name"
            required
            maxLength={80}
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Roof Cat"
            className="mt-1 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm outline-none focus:border-accent"
          />
        </div>
        <fieldset className="flex gap-1 rounded-lg border border-border p-1 text-sm">
          <legend className="sr-only">Orientação</legend>
          {(["portrait", "landscape"] as const).map((o) => (
            <button
              key={o}
              type="button"
              aria-pressed={orientation === o}
              onClick={() => setOrientation(o)}
              className={`rounded-md px-3 py-1.5 ${orientation === o ? "bg-accent text-white" : "text-muted"}`}
            >
              {o === "portrait" ? "Retrato" : "Paisagem"}
            </button>
          ))}
        </fieldset>
        <button
          type="submit"
          disabled={busy}
          className="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
        >
          {busy ? "Criando…" : "Forjar jogo"}
        </button>
      </div>
      {error && <p role="alert" className="mt-3 text-sm text-bad">{error}</p>}
    </form>
  );
}
