package domain

import (
	"errors"
	"testing"
)

func TestLLMSettingsValidate(t *testing.T) {
	cases := []struct {
		name    string
		s       LLMSettings
		wantErr bool
	}{
		{"fake needs nothing", LLMSettings{Provider: ProviderFake}, false},
		{"anthropic ok", LLMSettings{Provider: ProviderAnthropic, Model: "claude-opus-5-5", APIKey: "sk-ant-x"}, false},
		{"anthropic needs model", LLMSettings{Provider: ProviderAnthropic}, true},
		{"unknown provider", LLMSettings{Provider: "x"}, true},
		{"key with spaces", LLMSettings{Provider: ProviderFake, APIKey: "a b"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.s.Validate()
			if c.wantErr != errors.Is(err, ErrInvalid) {
				t.Fatalf("err = %v", err)
			}
		})
	}
	if got := MaskSecret("sk-ant-api03-abcdWXYZ"); got != "••••WXYZ" {
		t.Fatalf("mask = %q", got)
	}
}
