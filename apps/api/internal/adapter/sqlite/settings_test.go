package sqlite

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
)

func TestSettingsEncryptedRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo, err := NewSettingsRepo(db, []byte("0123456789abcdef-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if s, err := repo.GetLLM(ctx); s != nil || err != nil {
		t.Fatalf("empty repo: %v %v", s, err)
	}
	in := &domain.LLMSettings{Provider: domain.ProviderAnthropic, Model: "claude-opus-5-5", APIKey: "sk-ant-very-secret", RefusalFallback: true, UpdatedAt: time.Unix(5, 0)}
	if err := repo.SaveLLM(ctx, in); err != nil {
		t.Fatal(err)
	}
	var raw string
	_ = db.QueryRow(`SELECT value FROM settings WHERE key = 'llm'`).Scan(&raw)
	if strings.Contains(raw, "very-secret") {
		t.Fatal("api key stored in clear text")
	}
	got, err := repo.GetLLM(ctx)
	if err != nil || got.APIKey != in.APIKey || got.Model != in.Model || !got.RefusalFallback {
		t.Fatalf("got %+v, %v", got, err)
	}
	other, _ := NewSettingsRepo(db, []byte("another-secret-0000"))
	if _, err := other.GetLLM(ctx); err == nil {
		t.Fatal("wrong secret must fail to decrypt")
	}
}
