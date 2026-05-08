package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBearerAuthAcceptsMatchingToken(t *testing.T) {
	handler := BearerAuth("secret-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestBearerAuthRejectsMissingToken(t *testing.T) {
	handler := BearerAuth("secret-token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("protected handler was called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
