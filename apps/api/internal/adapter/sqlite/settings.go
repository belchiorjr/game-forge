package sqlite

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/belchiorjr/game-forge/apps/api/internal/domain"
)

// SettingsRepo stores settings as JSON. Secrets are sealed with AES-256-GCM using a key
// derived from the server secret, so a copied database file does not leak the API key.
type SettingsRepo struct {
	db   *sql.DB
	aead cipher.AEAD
}

func NewSettingsRepo(db *sql.DB, secret []byte) (*SettingsRepo, error) {
	if len(secret) < 16 {
		return nil, errors.New("settings secret must be at least 16 bytes")
	}
	key := sha256.Sum256(secret)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &SettingsRepo{db: db, aead: aead}, nil
}

type llmRow struct {
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	APIKeySealed    string `json:"api_key_sealed,omitempty"`
	RefusalFallback bool   `json:"refusal_fallback"`
}

const llmKey = "llm"

func (r *SettingsRepo) GetLLM(ctx context.Context) (*domain.LLMSettings, error) {
	var value, updated string
	err := r.db.QueryRowContext(ctx, `SELECT value, updated_at FROM settings WHERE key = ?`, llmKey).Scan(&value, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var row llmRow
	if err := json.Unmarshal([]byte(value), &row); err != nil {
		return nil, err
	}
	key, err := r.open(row.APIKeySealed)
	if err != nil {
		// Usually the server secret changed: the stored key is unrecoverable and must be re-entered.
		return nil, fmt.Errorf("decrypt stored api key (was FORGE_SECRET_KEY changed?): %w", err)
	}
	return &domain.LLMSettings{Provider: domain.LLMProvider(row.Provider), Model: row.Model, APIKey: key, RefusalFallback: row.RefusalFallback, UpdatedAt: parseTime(updated)}, nil
}

func (r *SettingsRepo) SaveLLM(ctx context.Context, s *domain.LLMSettings) error {
	sealed, err := r.seal(s.APIKey)
	if err != nil {
		return err
	}
	value, _ := json.Marshal(llmRow{Provider: string(s.Provider), Model: s.Model, APIKeySealed: sealed, RefusalFallback: s.RefusalFallback})
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		llmKey, string(value), fmtTime(s.UpdatedAt))
	return err
}

func (r *SettingsRepo) seal(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	nonce := make([]byte, r.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(r.aead.Seal(nonce, nonce, []byte(plain), []byte(llmKey))), nil
}

func (r *SettingsRepo) open(sealed string) (string, error) {
	if sealed == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil || len(raw) < r.aead.NonceSize() {
		return "", errors.New("corrupt value")
	}
	n := r.aead.NonceSize()
	plain, err := r.aead.Open(nil, raw[:n], raw[n:], []byte(llmKey))
	return string(plain), err
}
