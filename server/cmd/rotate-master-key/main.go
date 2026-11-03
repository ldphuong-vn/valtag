// Command rotate-master-key re-encrypts every master-key-wrapped column from
// an old valtag master key to a new one, in a single transaction: either
// every row is re-wrapped or nothing changes. Run it with the server STOPPED
// (or accept that rows written mid-rotation with the old key need a second
// run).
//
// Env:
//
//	DATABASE_URL    — Postgres connection string (required)
//	OLD_MASTER_KEY  — current VAULT_MASTER_KEY, base64 (required)
//	NEW_MASTER_KEY  — the replacement, base64 32 bytes (required)
//
// Consequences (by design): HMAC-derived API keys (valtag_dk_*) are derived
// from the master key, so every outstanding derived key stops validating —
// agents re-request credentials, which the lease model expects anyway.
// TOTP secrets, provider configs, integration tokens and secret DEKs survive
// transparently.
package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ldphuong-vn/valtag/server/internal/audit"
	"github.com/ldphuong-vn/valtag/server/pkg/crypto"
)

// rewrapSpec describes one master-key-wrapped column: how to read every
// (rowKey, ciphertext) pair and how to write a re-wrapped value back.
type rewrapSpec struct {
	name   string
	query  string
	update string
}

var specs = []rewrapSpec{
	{
		name:   "secrets.encrypted_dek",
		query:  `SELECT id, encrypted_dek FROM secrets`,
		update: `UPDATE secrets SET encrypted_dek = $1 WHERE id = $2`,
	},
	{
		name:   "users.totp_secret_enc",
		query:  `SELECT id, totp_secret_enc FROM users WHERE totp_secret_enc IS NOT NULL`,
		update: `UPDATE users SET totp_secret_enc = $1 WHERE id = $2`,
	},
	{
		name:   "dynamic_providers.config_enc",
		query:  `SELECT id, config_enc FROM dynamic_providers`,
		update: `UPDATE dynamic_providers SET config_enc = $1 WHERE id = $2`,
	},
	{
		name:   "dynamic_leases.secret_data_enc",
		query:  `SELECT id, secret_data_enc FROM dynamic_leases`,
		update: `UPDATE dynamic_leases SET secret_data_enc = $1 WHERE id = $2`,
	},
	{
		name:   "org_integrations.access_token_enc",
		query:  `SELECT id, access_token_enc FROM org_integrations`,
		update: `UPDATE org_integrations SET access_token_enc = $1 WHERE id = $2`,
	},
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	dbURL := os.Getenv("DATABASE_URL")
	oldB64 := os.Getenv("OLD_MASTER_KEY")
	newB64 := os.Getenv("NEW_MASTER_KEY")
	if dbURL == "" || oldB64 == "" || newB64 == "" {
		log.Fatal("DATABASE_URL, OLD_MASTER_KEY and NEW_MASTER_KEY are all required")
	}

	oldKey, err := decodeKey("OLD_MASTER_KEY", oldB64)
	if err != nil {
		log.Fatal(err)
	}
	newKey, err := decodeKey("NEW_MASTER_KEY", newB64)
	if err != nil {
		log.Fatal(err)
	}
	if string(oldKey) == string(newKey) {
		log.Fatal("OLD_MASTER_KEY and NEW_MASTER_KEY are identical — nothing to rotate")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	if err := runRotation(ctx, pool, oldKey, newKey); err != nil {
		log.Fatalf("rotation aborted, nothing changed: %v", err)
	}
}

// runRotation re-wraps every master-key-wrapped column in a single
// transaction and appends an audit entry. Any error aborts with a rollback —
// the DB is left exactly as before.
func runRotation(ctx context.Context, pool *pgxpool.Pool, oldKey, newKey []byte) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	total := 0
	for _, spec := range specs {
		rows, err := tx.Query(ctx, spec.query)
		if err != nil {
			return fmt.Errorf("%s: query: %w", spec.name, err)
		}
		var updates [][]interface{}
		for rows.Next() {
			var rowKey string
			var blob []byte
			if err := rows.Scan(&rowKey, &blob); err != nil {
				rows.Close()
				return fmt.Errorf("%s: scan: %w", spec.name, err)
			}
			plaintext, err := crypto.DecryptAES256GCM(oldKey, blob)
			if err != nil {
				rows.Close()
				return fmt.Errorf("%s row %s: decrypt with OLD key failed — wrong key or corrupt row: %w", spec.name, rowKey, err)
			}
			rewrapped, err := crypto.EncryptAES256GCM(newKey, plaintext)
			if err != nil {
				rows.Close()
				return fmt.Errorf("%s row %s: re-encrypt: %w", spec.name, rowKey, err)
			}
			updates = append(updates, []interface{}{rewrapped, rowKey})
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("%s: iterate: %w", spec.name, err)
		}
		for _, u := range updates {
			if _, err := tx.Exec(ctx, spec.update, u...); err != nil {
				return fmt.Errorf("%s row %s: update: %w", spec.name, u[1], err)
			}
		}
		total += len(updates)
		log.Printf("%s: %d row(s) re-wrapped", spec.name, len(updates))
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	// Audit the rotation (outside the tx — the chain logger manages its own).
	// audit_logs.user_id is a UUID column: the nil UUID marks system events,
	// the tool name travels in metadata.
	if _, err := audit.NewLogger(pool).Log(ctx, audit.Entry{
		UserID:       "00000000-0000-0000-0000-000000000000",
		Action:       "master_key.rotate",
		ResourceType: "system",
		EventType:    "action",
		Status:       "success",
		Metadata:     `{"tool":"rotate-master-key"}`,
	}); err != nil {
		log.Printf("audit log error (rotation already committed): %v", err)
	}

	log.Printf("Rotation complete: %d row(s) re-wrapped across %d tables. Restart the server with NEW_MASTER_KEY as VAULT_MASTER_KEY. Outstanding derived API keys (valtag_dk_*) no longer validate — agents re-request credentials.", total, len(specs))
	return nil
}

func decodeKey(name, b64 string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("%s: base64 decode: %w", name, err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("%s: must be 32 bytes when decoded, got %d", name, len(key))
	}
	return key, nil
}
