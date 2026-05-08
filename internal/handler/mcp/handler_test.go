package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"notesmcp/internal/domain"
)

func TestHandlerCallsVaultOverviewTool(t *testing.T) {
	handler := NewHandler(fakeVault{})
	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"vault_overview","arguments":{}}}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if response["jsonrpc"] != "2.0" {
		t.Fatalf("jsonrpc = %v", response["jsonrpc"])
	}
	if response["result"] == nil {
		t.Fatalf("result is nil: %s", rec.Body.String())
	}
}

type fakeVault struct{}

func (fakeVault) Overview(context.Context) (domain.Overview, error) {
	return domain.Overview{TotalObjects: 1, MarkdownNotes: 1}, nil
}

func (fakeVault) ListNotes(context.Context) ([]domain.Note, error) {
	return nil, nil
}

func (fakeVault) GetNote(context.Context, string) (domain.Note, error) {
	return domain.Note{}, nil
}

func (fakeVault) SearchNotes(context.Context, string, int) ([]domain.Note, error) {
	return nil, nil
}

func (fakeVault) Backlinks(context.Context, string) ([]domain.Note, error) {
	return nil, nil
}
