package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTManager struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

func NewJWTManager(privatePEM, publicPEM []byte) (*JWTManager, error) {
	privBlock, _ := pem.Decode(privatePEM)
	if privBlock == nil {
		return nil, fmt.Errorf("failed to decode private key PEM")
	}
	privKey, err := x509.ParsePKCS1PrivateKey(privBlock.Bytes)
	if err != nil {
		// Try PKCS8
		key, err2 := x509.ParsePKCS8PrivateKey(privBlock.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("parsing private key: %w", err)
		}
		var ok bool
		privKey, ok = key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
	}

	pubBlock, _ := pem.Decode(publicPEM)
	if pubBlock == nil {
		return nil, fmt.Errorf("failed to decode public key PEM")
	}
	pubIface, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing public key: %w", err)
	}
	pubKey, ok := pubIface.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not RSA")
	}

	return &JWTManager{privateKey: privKey, publicKey: pubKey}, nil
}

func (m *JWTManager) GenerateAccessToken(userID string) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(15 * time.Minute).Unix(),
		"typ": "access",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(m.privateKey)
}

// GenerateCLIToken issues a long-lived JWT for the CLI and MCP server (30-day expiry, typ: "cli").
// The ver claim carries the user's token_version at issue time: revoke-all
// bumps it, so outstanding CLI tokens stop validating (see ValidateUserToken).
func (m *JWTManager) GenerateCLIToken(userID string, tokenVersion int) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(30 * 24 * time.Hour).Unix(),
		"typ": "cli",
		"ver": tokenVersion,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(m.privateKey)
}

func (m *JWTManager) GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating refresh token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// GenerateTOTPToken issues a short-lived JWT for TOTP challenge (5min, purpose=totp).
func (m *JWTManager) GenerateTOTPToken(userID string) (string, error) {
	claims := jwt.MapClaims{
		"sub":     userID,
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(5 * time.Minute).Unix(),
		"typ":     "totp",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(m.privateKey)
}

// ValidateTOTPToken validates a TOTP challenge token and returns the userID.
func (m *JWTManager) ValidateTOTPToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.publicKey, nil
	})
	if err != nil {
		return "", fmt.Errorf("invalid totp token: %w", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("invalid token claims")
	}
	typ, _ := claims["typ"].(string)
	if typ != "totp" {
		return "", fmt.Errorf("not a totp token")
	}
	userID, ok := claims["sub"].(string)
	if !ok || userID == "" {
		return "", fmt.Errorf("missing subject claim")
	}
	return userID, nil
}

// ValidateAccessToken validates a JWT and returns the userID.
// Accepts both typ="access" (15-min dashboard tokens) and typ="cli" (30-day CLI/MCP tokens).
func (m *JWTManager) ValidateAccessToken(tokenString string) (string, error) {
	userID, _, _, err := m.validateAccessTokenDetailed(tokenString)
	return userID, err
}

// validateAccessTokenDetailed parses an access/CLI JWT and surfaces the
// claims callers need for further checks (typ, ver).
func (m *JWTManager) validateAccessTokenDetailed(tokenString string) (userID, typ string, version int, err error) {
	token, parseErr := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.publicKey, nil
	})
	if parseErr != nil {
		return "", "", 0, fmt.Errorf("invalid token: %w", parseErr)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", "", 0, fmt.Errorf("invalid token claims")
	}

	typ, _ = claims["typ"].(string)
	if typ != "access" && typ != "cli" {
		return "", "", 0, fmt.Errorf("not an access token")
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", "", 0, fmt.Errorf("missing subject claim")
	}
	userID = sub

	if verFloat, ok := claims["ver"].(float64); ok {
		version = int(verFloat)
	}
	return userID, typ, version, nil
}
