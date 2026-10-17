"use client";

import { useEffect, useReducer, useState } from "react";
import { initialTimeline, timelineReducer } from "./reducer";
import { EVENT_TYPES, type ForgeEvent } from "./types";

export type ConnectionState = "connecting" | "open" | "reconnecting";

// One EventSource per open project. The browser resends Last-Event-ID on reconnect and the
// API replays what was missed, so the timeline survives refreshes and API restarts.
export function useProjectEvents(projectId: string) {
  const [state, dispatch] = useReducer(timelineReducer, initialTimeline);
  const [connection, setConnection] = useState<ConnectionState>("connecting");

  useEffect(() => {
    const es = new EventSource(`/api/forge/projects/${projectId}/events`);
    es.onopen = () => setConnection("open");
    es.onerror = () => setConnection("reconnecting");
    const onEvent = (msg: MessageEvent<string>) => {
      try {
        dispatch({ ...JSON.parse(msg.data), type: msg.type } as ForgeEvent);
      } catch {
        // ignore malformed payloads rather than breaking the whole timeline
      }
    };
    for (const t of EVENT_TYPES) es.addEventListener(t, onEvent);
    return () => es.close();
  }, [projectId]);

  return { state, connection };
}
