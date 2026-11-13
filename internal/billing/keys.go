// Package billing sells higher API rate limits: Stripe Checkout subscription -> API key.
// There are no user accounts; the Stripe Checkout session id is the customer's handle.
package billing

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	_ "modernc.org/sqlite"
)

// KeyPrefix marks API keys so they are recognisable in logs and at the nginx edge.
const KeyPrefix = "cjp_"

// Keys stores API keys in their own writable SQLite file (the dictionary database is read-only
// and gets rebuilt by the ETL). One key per Stripe subscription; only the SHA-256 is stored.
type Keys struct{ db *sql.DB }

func OpenKeys(path string) (*Keys, error) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS api_keys (
		subscription_id TEXT PRIMARY KEY,
		key_hash        TEXT NOT NULL UNIQUE,
		active          INTEGER NOT NULL,
		created_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Keys{db: db}, nil
}

func (k *Keys) Close() error { return k.db.Close() }

func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// Issue creates the subscription's key or replaces the existing one, and returns the new key.
func (k *Keys) Issue(ctx context.Context, subscriptionID string) (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	key := KeyPrefix + base64.RawURLEncoding.EncodeToString(raw)
	_, err := k.db.ExecContext(ctx, `INSERT INTO api_keys (subscription_id, key_hash, active) VALUES (?, ?, 1)
		ON CONFLICT(subscription_id) DO UPDATE SET key_hash = excluded.key_hash, active = 1, created_at = CURRENT_TIMESTAMP`,
		subscriptionID, hashKey(key))
	return key, err
}

// Exists reports whether a key was already issued for the subscription.
func (k *Keys) Exists(ctx context.Context, subscriptionID string) (bool, error) {
	var n int
	err := k.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_keys WHERE subscription_id = ?`, subscriptionID).Scan(&n)
	return n > 0, err
}

// SetActive follows the subscription's state (webhook). Unknown subscriptions are a no-op.
func (k *Keys) SetActive(ctx context.Context, subscriptionID string, active bool) error {
	_, err := k.db.ExecContext(ctx, `UPDATE api_keys SET active = ? WHERE subscription_id = ?`, active, subscriptionID)
	return err
}

// Lookup returns a stable id for rate limiting and whether the key is valid and paid for.
func (k *Keys) Lookup(ctx context.Context, key string) (id string, ok bool) {
	h := hashKey(key)
	var active bool
	if err := k.db.QueryRowContext(ctx, `SELECT active FROM api_keys WHERE key_hash = ?`, h).Scan(&active); err != nil {
		return "", false
	}
	return h, active
}
