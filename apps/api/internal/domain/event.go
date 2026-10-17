package domain

import (
	"encoding/json"
	"time"
)

type EventType string

const (
	EventRunStarted   EventType = "run.started"
	EventRunFinished  EventType = "run.finished"
	EventAgentMessage EventType = "agent.message"
	EventAgentPlan    EventType = "agent.plan"
	EventToolStarted  EventType = "tool.started"
	EventToolFinished EventType = "tool.finished"
	EventFileChanged  EventType = "file.changed"
	EventUsage        EventType = "usage"
	EventError        EventType = "error"
)

// Event is one entry of a project's timeline. Seq is per project and strictly increasing,
// which lets SSE clients resume with Last-Event-ID.
type Event struct {
	Seq       int64
	ProjectID string
	RunID     string
	Type      EventType
	Data      json.RawMessage
	TS        time.Time
}
