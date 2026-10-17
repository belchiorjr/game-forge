import type { ForgeEvent, PlanItem, Usage } from "./types";

export type TimelineItem =
  | { kind: "user"; seq: number; runId: string; text: string }
  | { kind: "agent"; seq: number; runId: string; text: string }
  | { kind: "tool"; seq: number; runId: string; id: string; name: string; path: string; args: string; ok?: boolean; output?: string; durationMs?: number }
  | { kind: "finished"; seq: number; runId: string; status: string; summary: string; commit: string }
  | { kind: "error"; seq: number; runId: string; message: string };

export type BuildState = {
  status: "idle" | "building" | "ok" | "failed";
  version: number; // bumps on every successful wasm build; the preview reloads on change
  output: string;
  durationMs: number;
};

export type TimelineState = {
  build: BuildState;
  lastSeq: number;
  items: TimelineItem[];
  activeRunId: string | null;
  plan: PlanItem[];
  usage: Usage;
  changedFiles: string[];
  fileVersion: number; // bumps on every file.changed so views can refetch
};

export const initialTimeline: TimelineState = {
  build: { status: "idle", version: 0, output: "", durationMs: 0 },
  lastSeq: 0,
  items: [],
  activeRunId: null,
  plan: [],
  usage: { input_tokens: 0, output_tokens: 0 },
  changedFiles: [],
  fileVersion: 0,
};

// The UI is a pure function of the event log, so a refresh mid-run rebuilds everything by replay.
export function timelineReducer(state: TimelineState, e: ForgeEvent): TimelineState {
  if (e.seq <= state.lastSeq) return state; // duplicate after reconnect
  const s = { ...state, lastSeq: e.seq };
  const base = { seq: e.seq, runId: e.run_id };
  switch (e.type) {
    case "run.started":
      return {
        ...s,
        activeRunId: e.run_id,
        plan: [],
        usage: { input_tokens: 0, output_tokens: 0 },
        changedFiles: [],
        items: [...s.items, { kind: "user", ...base, text: e.prompt }],
      };
    case "run.finished":
      return {
        ...s,
        activeRunId: null,
        items: [...s.items, { kind: "finished", ...base, status: e.status, summary: e.summary, commit: e.commit }],
      };
    case "agent.message":
      return { ...s, items: [...s.items, { kind: "agent", ...base, text: e.text }] };
    case "agent.plan":
      return { ...s, plan: e.items };
    case "tool.started":
      return { ...s, items: [...s.items, { kind: "tool", ...base, id: e.tool_call_id, name: e.name, path: e.path ?? "", args: e.args }] };
    case "tool.finished":
      return {
        ...s,
        items: s.items.map((it) =>
          it.kind === "tool" && it.id === e.tool_call_id && it.runId === e.run_id
            ? { ...it, ok: e.ok, output: e.output, durationMs: e.duration_ms }
            : it,
        ),
      };
    case "file.changed":
      return {
        ...s,
        fileVersion: s.fileVersion + 1,
        changedFiles: s.changedFiles.includes(e.path) ? s.changedFiles : [...s.changedFiles, e.path],
      };
    case "build.started":
      return { ...s, build: { ...s.build, status: "building" } };
    case "build.finished":
      return {
        ...s,
        build: {
          status: e.ok ? "ok" : "failed",
          version: e.ok ? s.build.version + 1 : s.build.version,
          output: e.output,
          durationMs: e.duration_ms,
        },
      };
    case "usage":
      return { ...s, usage: { input_tokens: e.input_tokens, output_tokens: e.output_tokens } };
    case "error":
      return { ...s, items: [...s.items, { kind: "error", ...base, message: e.message }] };
  }
}
