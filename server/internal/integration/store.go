package integration

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ldphuong-vn/valtag/server/pkg/crypto"
)

// OrgIntegration represents a connected third-party integration for an org.
type OrgIntegration struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	Provider    string    `json:"provider"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
	TeamName    string    `json:"team_name,omitempty"`
	BotUserID   string    `json:"bot_user_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Store manages org_integrations with encrypted token storage.
type Store struct {
	pool      *pgxpool.Pool
	masterKey []byte
}

// NewStore creates an integration Store.
func NewStore(pool *pgxpool.Pool, masterKey []byte) *Store {
	return &Store{pool: pool, masterKey: masterKey}
}

// Upsert inserts or updates an org integration with an encrypted access token.
func (s *Store) Upsert(ctx context.Context, orgID, provider, accessToken, workspaceID, teamName, botUserID string) error {
	enc, err := crypto.EncryptAES256GCM(s.masterKey, []byte(accessToken))
	if err != nil {
		return fmt.Errorf("encrypting token: %w", err)
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO org_integrations (org_id, provider, access_token_enc, workspace_id, team_name, bot_user_id)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (org_id, provider) DO UPDATE
		 SET access_token_enc = EXCLUDED.access_token_enc,
		     workspace_id = EXCLUDED.workspace_id,
		     team_name = EXCLUDED.team_name,
		     bot_user_id = EXCLUDED.bot_user_id,
		     updated_at = NOW()`,
		orgID, provider, enc, workspaceID, teamName, botUserID,
	)
	return err
}

// Get returns an org integration (without decrypted token — use GetToken for that).
func (s *Store) Get(ctx context.Context, orgID, provider string) (*OrgIntegration, error) {
	var i OrgIntegration
	err := s.pool.QueryRow(ctx,
		`SELECT id, org_id, provider, COALESCE(workspace_id,''), COALESCE(team_name,''), COALESCE(bot_user_id,''), created_at, updated_at
		 FROM org_integrations WHERE org_id = $1 AND provider = $2`,
		orgID, provider,
	).Scan(&i.ID, &i.OrgID, &i.Provider, &i.WorkspaceID, &i.TeamName, &i.BotUserID, &i.CreatedAt, &i.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying integration: %w", err)
	}
	return &i, nil
}

// GetToken returns the decrypted access token for an org+provider.
func (s *Store) GetToken(ctx context.Context, orgID, provider string) (string, error) {
	var enc []byte
	err := s.pool.QueryRow(ctx,
		`SELECT access_token_enc FROM org_integrations WHERE org_id = $1 AND provider = $2`,
		orgID, provider,
	).Scan(&enc)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("querying token: %w", err)
	}
	plain, err := crypto.DecryptAES256GCM(s.masterKey, enc)
	if err != nil {
		return "", fmt.Errorf("decrypting token: %w", err)
	}
	return string(plain), nil
}

// Delete removes an org integration.
func (s *Store) Delete(ctx context.Context, orgID, provider string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM org_integrations WHERE org_id = $1 AND provider = $2`,
		orgID, provider,
	)
	return err
}

// ListByOrg returns all integrations for an org (no tokens).
func (s *Store) ListByOrg(ctx context.Context, orgID string) ([]OrgIntegration, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, org_id, provider, COALESCE(workspace_id,''), COALESCE(team_name,''), COALESCE(bot_user_id,''), created_at, updated_at
		 FROM org_integrations WHERE org_id = $1 ORDER BY created_at`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing integrations: %w", err)
	}
	defer rows.Close()

	var result []OrgIntegration
	for rows.Next() {
		var i OrgIntegration
		if err := rows.Scan(&i.ID, &i.OrgID, &i.Provider, &i.WorkspaceID, &i.TeamName, &i.BotUserID, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning integration: %w", err)
		}
		result = append(result, i)
	}
	if result == nil {
		result = []OrgIntegration{}
	}
	return result, nil
}
