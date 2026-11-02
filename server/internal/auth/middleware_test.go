package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddleware_NoHeader(t *testing.T) {
	privPEM, pubPEM := generateTestKeys(t)
	mgr, _ := NewJWTManager(privPEM, pubPEM)
	called := false
	handler := AuthMiddleware(nil, mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status=%d, want 401", rec.Code)
	}
	if called {
		t.Error("handler should not be called")
	}
}

func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	privPEM, pubPEM := generateTestKeys(t)
	mgr, _ := NewJWTManager(privPEM, pubPEM)
	handler := AuthMiddleware(nil, mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Token abc123") // wrong scheme
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status=%d, want 401", rec.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	privPEM, pubPEM := generateTestKeys(t)
	mgr, _ := NewJWTManager(privPEM, pubPEM)
	handler := AuthMiddleware(nil, mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalidtoken")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status=%d, want 401", rec.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	privPEM, pubPEM := generateTestKeys(t)
	mgr, _ := NewJWTManager(privPEM, pubPEM)
	var gotUserID string
	handler := AuthMiddleware(nil, mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	token, _ := mgr.GenerateAccessToken("test-user-id")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status=%d, want 200", rec.Code)
	}
	if gotUserID != "test-user-id" {
		t.Errorf("userID=%q, want test-user-id", gotUserID)
	}
}

func TestUserIDFromContext_Empty(t *testing.T) {
	id := UserIDFromContext(context.Background())
	if id != "" {
		t.Errorf("empty context should return empty string, got %q", id)
	}
}

func TestUserIDFromContext_WithValue(t *testing.T) {
	privPEM, pubPEM := generateTestKeys(t)
	mgr, _ := NewJWTManager(privPEM, pubPEM)
	token, _ := mgr.GenerateAccessToken("stored-user")
	var extractedID string
	handler := AuthMiddleware(nil, mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		extractedID = UserIDFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if extractedID != "stored-user" {
		t.Errorf("extractedID=%q, want stored-user", extractedID)
	}
}
