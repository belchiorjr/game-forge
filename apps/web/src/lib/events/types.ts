import type { components } from "../api/schema";

type Envelope = { seq: number; run_id: string; ts: string };
type RunStatus = components["schemas"]["RunStatus"];
export type PlanItem = { text: string; done: boolean };
export type Usage = { input_tokens: number; output_tokens: number };

// Discriminated union mirrored from the API contract's event table.
export type ForgeEvent = Envelope &
  (
    | { type: "run.started"; status: RunStatus; prompt: string }
    | { type: "run.finished"; status: RunStatus; summary: string; commit: string; usage: Usage }
    | { type: "agent.message"; text: string }
    | { type: "agent.plan"; items: PlanItem[] }
    | { type: "tool.started"; tool_call_id: string; name: string; args: string }
    | { type: "tool.finished"; tool_call_id: string; name: string; ok: boolean; output: string; duration_ms: number }
    | { type: "file.changed"; path: string; kind: "created" | "modified" | "deleted" }
    | { type: "usage"; input_tokens: number; output_tokens: number }
    | { type: "error"; code: string; message: string; recoverable: boolean }
  );

export type EventType = ForgeEvent["type"];

export const EVENT_TYPES: EventType[] = [
  "run.started",
  "run.finished",
  "agent.message",
  "agent.plan",
  "tool.started",
  "tool.finished",
  "file.changed",
  "usage",
  "error",
];
