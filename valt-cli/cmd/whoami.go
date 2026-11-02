package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/ldphuong-vn/valtag/valt-cli/internal/keychain"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show currently authenticated user info",
	RunE:  runWhoami,
}

func init() { rootCmd.AddCommand(whoamiCmd) }

func runWhoami(cmd *cobra.Command, args []string) error {
	token, err := keychain.GetToken()
	if err != nil || token == "" {
		fmt.Println("Not logged in. Run 'valtag setup' to authenticate.")
		return nil
	}

	claims, err := decodeJWTClaims(token)
	if err != nil {
		fmt.Println("Stored token is malformed. Run 'valtag setup' to re-authenticate.")
		return nil
	}

	userID, _ := claims["sub"].(string)
	typ, _ := claims["typ"].(string)

	var expStr string
	if expRaw, ok := claims["exp"].(float64); ok {
		expTime := time.Unix(int64(expRaw), 0)
		if time.Now().After(expTime) {
			expStr = fmt.Sprintf("%s (EXPIRED)", expTime.Format(time.RFC3339))
		} else {
			expStr = expTime.Format(time.RFC3339)
		}
	}

	fmt.Printf("User ID:    %s\n", userID)
	fmt.Printf("Token type: %s\n", typ)
	fmt.Printf("Expires:    %s\n", expStr)
	return nil
}

// decodeJWTClaims base64-decodes the JWT payload section without verifying signature.
func decodeJWTClaims(token string) (map[string]interface{}, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("not a JWT")
	}
	// JWT uses base64url without padding.
	payload := parts[1]
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}
	data, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(data, &claims); err != nil {
		return nil, fmt.Errorf("unmarshal claims: %w", err)
	}
	return claims, nil
}
