import type { components } from "./schema";

export type Project = components["schemas"]["Project"];
export type Run = components["schemas"]["Run"];
export type RunStatus = components["schemas"]["RunStatus"];
export type FileInfo = components["schemas"]["FileInfo"];
export type CreateProjectRequest = components["schemas"]["CreateProjectRequest"];
export type LLMSettings = components["schemas"]["LLMSettings"];
export type LLMSettingsInput = components["schemas"]["LLMSettingsInput"];

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
  }
}

const BASE = "/api/forge";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(BASE + path, { cache: "no-store", ...init });
  const type = res.headers.get("content-type") ?? "";
  if (!res.ok) {
    const body = type.includes("json") ? await res.json().catch(() => null) : null;
    throw new ApiError(res.status, body?.error?.code ?? "http_error", body?.error?.message ?? res.statusText);
  }
  return (type.includes("json") ? res.json() : res.text()) as Promise<T>;
}

const json = (method: string, body: unknown, headers: Record<string, string> = {}): RequestInit => ({
  method,
  headers: { "content-type": "application/json", ...headers },
  body: JSON.stringify(body),
});

export const api = {
  getLLMSettings: () => request<LLMSettings>("/settings/llm"),
  saveLLMSettings: (body: LLMSettingsInput) => request<LLMSettings>("/settings/llm", json("PUT", body)),
  testLLMSettings: (body: LLMSettingsInput) =>
    request<{ ok: boolean; message: string }>("/settings/llm/test", json("POST", body)),
  listProjects: () => request<{ projects: Project[] }>("/projects").then((r) => r.projects),
  getProject: (id: string) => request<Project>(`/projects/${id}`),
  createProject: (body: CreateProjectRequest) => request<Project>("/projects", json("POST", body)),
  startRun: (id: string, prompt: string, idempotencyKey: string) =>
    request<{ run_id: string }>(`/projects/${id}/runs`, json("POST", { prompt }, { "idempotency-key": idempotencyKey })),
  cancelRun: (id: string, runId: string) => request<unknown>(`/projects/${id}/runs/${runId}/cancel`, { method: "POST" }),
  startBuild: (id: string) => request<unknown>(`/projects/${id}/builds`, json("POST", { target: "wasm" })),
  hasPreview: (id: string) =>
    fetch(`${BASE}/projects/${id}/preview/game.wasm`, { method: "HEAD", cache: "no-store" }).then((r) => r.ok),
  listFiles: (id: string) => request<{ files: FileInfo[] }>(`/projects/${id}/files`).then((r) => r.files),
  readFile: (id: string, path: string) =>
    request<string>(`/projects/${id}/files/content?path=${encodeURIComponent(path)}`),
  writeFile: (id: string, path: string, content: string) =>
    request<{ commit: string }>(`/projects/${id}/files/content?path=${encodeURIComponent(path)}`, {
      method: "PUT",
      headers: { "content-type": "text/plain" },
      body: content,
    }),
};
