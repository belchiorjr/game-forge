package port

import (
	"context"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
)

type Message struct {
	Role      string // "user" | "assistant" | "tool"
	Text      string
	ToolCalls []ToolCall
	ToolID    string // set when Role == "tool"
	IsError   bool   // set when Role == "tool" and the tool failed
	// Opaque is the provider's own copy of an assistant turn (thinking blocks included).
	// Adapters replay it unchanged, because some providers reject edited history.
	Opaque any
}

type ToolCall struct {
	ID    string
	Name  string
	Input []byte // raw JSON
}

type ToolDef struct {
	Name        string
	Description string
	Schema      []byte // JSON Schema
}

type Usage struct{ InputTokens, OutputTokens int }

type LLMResponse struct {
	Text      string
	ToolCalls []ToolCall
	Usage     Usage
	Opaque    any // copy into the assistant Message appended to history
}

// LLM is provider-neutral. Adapters translate to a specific provider's API.
type LLM interface {
	Complete(ctx context.Context, system string, msgs []Message, tools []ToolDef) (LLMResponse, error)
}

// SettingsRepository stores the LLM configuration (API key encrypted at rest).
type SettingsRepository interface {
	// GetLLM returns nil, nil when nothing was saved yet.
	GetLLM(ctx context.Context) (*domain.LLMSettings, error)
	SaveLLM(ctx context.Context, s *domain.LLMSettings) error
}

// LLMTester checks that a configuration can reach the provider without running an agent.
type LLMTester interface {
	Test(ctx context.Context, s domain.LLMSettings) error
}
