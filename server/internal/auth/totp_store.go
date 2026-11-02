package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ldphuong-vn/valtag/server/pkg/crypto"
)

// TOTPStore manages TOTP secrets and backup codes in the database.
type TOTPStore struct {
	pool      *pgxpool.Pool
	masterKey []byte
}

// NewTOTPStore creates a TOTPStore.
func NewTOTPStore(pool *pgxpool.Pool, masterKey []byte) *TOTPStore {
	return &TOTPStore{pool: pool, masterKey: masterKey}
}

// SetSetupSecret stores the pending TOTP secret (not yet verified).
func (s *TOTPStore) SetSetupSecret(ctx context.Context, userID, secret string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET totp_setup_secret = $1 WHERE id = $2`,
		secret, userID,
	)
	return err
}

// GetSetupSecret returns the pending TOTP setup secret.
func (s *TOTPStore) GetSetupSecret(ctx context.Context, userID string) (string, error) {
	var secret *string
	err := s.pool.QueryRow(ctx,
		`SELECT totp_setup_secret FROM users WHERE id = $1`, userID,
	).Scan(&secret)
	if err != nil {
		return "", err
	}
	if secret == nil || *secret == "" {
		return "", fmt.Errorf("no pending TOTP setup")
	}
	return *secret, nil
}

// EnableTOTP encrypts the secret, stores it, and enables 2FA.
func (s *TOTPStore) EnableTOTP(ctx context.Context, userID, secret string) error {
	enc, err := crypto.EncryptAES256GCM(s.masterKey, []byte(secret))
	if err != nil {
		return fmt.Errorf("encrypting totp secret: %w", err)
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE users SET totp_secret_enc = $1, totp_enabled = true, totp_setup_secret = NULL WHERE id = $2`,
		enc, userID,
	)
	return err
}

// DisableTOTP removes TOTP configuration and backup codes.
func (s *TOTPStore) DisableTOTP(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET totp_secret_enc = NULL, totp_enabled = false, totp_setup_secret = NULL WHERE id = $1`,
		userID,
	)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM backup_codes WHERE user_id = $1`, userID)
	return err
}

// GetTOTPSecret returns the decrypted TOTP secret and enabled flag.
func (s *TOTPStore) GetTOTPSecret(ctx context.Context, userID string) (string, bool, error) {
	var enc []byte
	var enabled bool
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(totp_secret_enc, '\x'), totp_enabled FROM users WHERE id = $1`, userID,
	).Scan(&enc, &enabled)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}
	if !enabled || len(enc) == 0 {
		return "", enabled, nil
	}
	plain, err := crypto.DecryptAES256GCM(s.masterKey, enc)
	if err != nil {
		return "", enabled, fmt.Errorf("decrypting totp secret: %w", err)
	}
	return string(plain), enabled, nil
}

// StoreBackupCodes deletes old codes and inserts new hashed codes.
func (s *TOTPStore) StoreBackupCodes(ctx context.Context, userID string, codes []string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM backup_codes WHERE user_id = $1`, userID)
	if err != nil {
		return err
	}
	for _, code := range codes {
		hash := hashCode(code)
		if _, err := s.pool.Exec(ctx,
			`INSERT INTO backup_codes (user_id, code_hash) VALUES ($1, $2)`,
			userID, hash,
		); err != nil {
			return err
		}
	}
	return nil
}

// UseBackupCode attempts to use a backup code. Returns true if valid and consumed.
func (s *TOTPStore) UseBackupCode(ctx context.Context, userID, code string) (bool, error) {
	hash := hashCode(code)
	tag, err := s.pool.Exec(ctx,
		`UPDATE backup_codes SET used = true WHERE user_id = $1 AND code_hash = $2 AND used = false`,
		userID, hash,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func hashCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}
