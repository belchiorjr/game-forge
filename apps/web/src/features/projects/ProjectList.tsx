"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api, type Project } from "@/lib/api/client";

export function ProjectList() {
  const [projects, setProjects] = useState<Project[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.listProjects().then(setProjects).catch((e: Error) => setError(e.message));
  }, []);

  if (error) return <p role="alert" className="text-sm text-bad">Não foi possível carregar: {error}</p>;
  if (!projects) return <p className="text-sm text-muted">Carregando…</p>;
  if (projects.length === 0) return <p className="text-sm text-muted">Nenhum projeto ainda. Descreva o primeiro acima.</p>;

  return (
    <ul className="divide-y divide-border overflow-hidden rounded-xl border border-border bg-surface">
      {projects.map((p) => (
        <li key={p.id}>
          <Link href={`/projects/${p.id}`} className="block px-4 py-3 hover:bg-background focus-visible:bg-background">
            <div className="font-medium">{p.name}</div>
            <div className="truncate text-sm text-muted">{p.idea}</div>
          </Link>
        </li>
      ))}
    </ul>
  );
}
