package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"

	"github.com/belchiorjr/game-forge/apps/api/internal/usecase/port"
)

// Anthropic implements port.LLM with the official SDK. Credentials come from the
// environment (ANTHROPIC_API_KEY or an `ant auth login` profile), never from the web app.
type Anthropic struct {
	client   anthropic.Client
	model    string
	fallback bool
}

func NewAnthropic(model string, fallback bool, opts ...option.RequestOption) *Anthropic {
	return &Anthropic{client: anthropic.NewClient(opts...), model: model, fallback: fallback}
}

func (a *Anthropic) Complete(ctx context.Context, system string, msgs []port.Message, tools []port.ToolDef) (port.LLMResponse, error) {
	params := anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(a.model),
		MaxTokens: 32000,
		System:    []anthropic.BetaTextBlockParam{{Text: system}},
		Messages:  toParams(msgs),
		Tools:     toTools(tools),
		// Agentic coding benefits from more depth than the model's default effort.
		OutputConfig: anthropic.BetaOutputConfigParam{Effort: anthropic.BetaOutputConfigEffortHigh},
		// Tools + system are stable across a run, so caching the prefix makes each step cheaper.
		CacheControl: anthropic.NewBetaCacheControlEphemeralParam(),
	}
	if a.fallback {
		// A policy refusal is re-served inside the same call by a model the API picks per category.
		params.Betas = []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01}
		params.Fallbacks = anthropic.BetaFallbacksParamUnion{OfDefault: constant.ValueOf[constant.Default]()}
	}
	// Streaming avoids HTTP timeouts on long agent turns.
	stream := a.client.Beta.Messages.NewStreaming(ctx, params)
	var msg anthropic.BetaMessage
	for stream.Next() {
		if err := msg.Accumulate(stream.Current()); err != nil {
			return port.LLMResponse{}, err
		}
	}
	if err := stream.Err(); err != nil {
		return port.LLMResponse{}, err
	}

	out := port.LLMResponse{
		Usage:  port.Usage{InputTokens: int(msg.Usage.InputTokens + msg.Usage.CacheReadInputTokens + msg.Usage.CacheCreationInputTokens), OutputTokens: int(msg.Usage.OutputTokens)},
		Opaque: msg.ToParam(),
	}
	switch msg.StopReason {
	case anthropic.BetaStopReasonRefusal:
		return out, fmt.Errorf("model declined the request")
	case anthropic.BetaStopReasonMaxTokens:
		return out, fmt.Errorf("model response hit max_tokens")
	}
	for _, block := range msg.Content {
		switch b := block.AsAny().(type) {
		case anthropic.BetaTextBlock:
			out.Text += b.Text
		case anthropic.BetaToolUseBlock:
			out.ToolCalls = append(out.ToolCalls, port.ToolCall{ID: b.ID, Name: b.Name, Input: []byte(b.JSON.Input.Raw())})
		}
	}
	return out, nil
}

// toParams maps the neutral history to API messages. Assistant turns replay their opaque
// copy unchanged; consecutive tool results are grouped into one user message.
func toParams(msgs []port.Message) []anthropic.BetaMessageParam {
	var out []anthropic.BetaMessageParam
	var results []anthropic.BetaContentBlockParamUnion
	flush := func() {
		if len(results) > 0 {
			out = append(out, anthropic.NewBetaUserMessage(results...))
			results = nil
		}
	}
	for _, m := range msgs {
		switch m.Role {
		case "tool":
			results = append(results, anthropic.NewBetaToolResultBlock(m.ToolID, m.Text, m.IsError))
		case "assistant":
			flush()
			if p, ok := m.Opaque.(anthropic.BetaMessageParam); ok {
				out = append(out, p)
				continue
			}
			blocks := []anthropic.BetaContentBlockParamUnion{}
			if m.Text != "" {
				blocks = append(blocks, anthropic.NewBetaTextBlock(m.Text))
			}
			for _, c := range m.ToolCalls {
				var input any
				_ = json.Unmarshal(c.Input, &input)
				blocks = append(blocks, anthropic.NewBetaToolUseBlock(c.ID, input, c.Name))
			}
			out = append(out, anthropic.BetaMessageParam{Role: anthropic.BetaMessageParamRoleAssistant, Content: blocks})
		default:
			flush()
			out = append(out, anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(m.Text)))
		}
	}
	flush()
	return out
}

func toTools(defs []port.ToolDef) []anthropic.BetaToolUnionParam {
	out := make([]anthropic.BetaToolUnionParam, 0, len(defs))
	for _, d := range defs {
		var schema struct {
			Properties map[string]any `json:"properties"`
			Required   []string       `json:"required"`
		}
		_ = json.Unmarshal(d.Schema, &schema)
		tool := anthropic.BetaToolParam{
			Name:        d.Name,
			Description: anthropic.String(d.Description),
			InputSchema: anthropic.BetaToolInputSchemaParam{Properties: schema.Properties, Required: schema.Required},
		}
		out = append(out, anthropic.BetaToolUnionParam{OfTool: &tool})
	}
	return out
}
