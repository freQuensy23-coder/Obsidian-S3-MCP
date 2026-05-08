package exam

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"notesmcp/internal/auth"
	"notesmcp/internal/config"
	"notesmcp/internal/handler/mcp"
	"notesmcp/internal/storage/cache"
	"notesmcp/internal/storage/s3store"
	"notesmcp/internal/usecase/vault"
)

func TestMCPReadsObsidianVaultFromLocalS3CompatibleStorage(t *testing.T) {
	s3 := newFakeS3(map[string]string{
		"Home.md":             "[[Haskell]]\n![[Materials/image.png]]",
		"Haskell.md":          "#guide\nFunctional programming note",
		"Materials/image.png": "png-bytes",
		"Excalidraw/Board.md": "```compressed-json\nignored\n```",
		"Tasks/Read later.md": "todo",
	})
	s3Server := httptest.NewServer(s3)
	t.Cleanup(s3Server.Close)

	store, err := s3store.New(context.Background(), config.S3Config{
		Endpoint:        s3Server.URL,
		Region:          "test-region",
		AccessKeyID:     "test-access",
		SecretAccessKey: "test-secret",
		Bucket:          "notes",
	})
	if err != nil {
		t.Fatalf("create s3 store: %v", err)
	}

	service := vault.NewService(cache.New(store, 30*time.Second, time.Now))
	mcpServer := httptest.NewServer(auth.BearerAuth("test-token", mcp.NewHandler(service)))
	t.Cleanup(mcpServer.Close)

	overview := callTool(t, mcpServer.URL, "test-token", "vault_overview", map[string]any{})
	result, ok := overview["result"].(map[string]any)
	if !ok {
		t.Fatalf("overview result has unexpected shape: %#v", overview)
	}
	if got := result["markdown_notes"]; got != float64(4) {
		t.Fatalf("markdown_notes = %#v, want 4", got)
	}

	first := callTool(t, mcpServer.URL, "test-token", "get_note", map[string]any{"key": "Home.md"})
	second := callTool(t, mcpServer.URL, "test-token", "get_note", map[string]any{"key": "Home.md"})
	firstResult := first["result"].(map[string]any)
	secondResult := second["result"].(map[string]any)
	if firstResult["body"] != "[[Haskell]]\n![[Materials/image.png]]" || secondResult["body"] != firstResult["body"] {
		t.Fatalf("unexpected get_note bodies: %#v %#v", firstResult["body"], secondResult["body"])
	}
	if got := s3.getObjectCalls("Home.md"); got != 1 {
		t.Fatalf("local S3 GetObject calls for Home.md = %d, want 1 due to cache", got)
	}

	backlinks := callTool(t, mcpServer.URL, "test-token", "get_backlinks", map[string]any{"key": "Haskell.md"})
	backlinkResult := backlinks["result"].([]any)
	if len(backlinkResult) != 1 || backlinkResult[0].(map[string]any)["key"] != "Home.md" {
		t.Fatalf("backlinks = %#v, want Home.md", backlinkResult)
	}
}

func callTool(t *testing.T, url, token, name string, args map[string]any) map[string]any {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out["error"] != nil {
		t.Fatalf("rpc error: %#v", out["error"])
	}
	return out
}

type fakeS3 struct {
	mu       sync.Mutex
	objects  map[string]string
	getCalls map[string]int
}

func newFakeS3(objects map[string]string) *fakeS3 {
	return &fakeS3{objects: objects, getCalls: map[string]int{}}
}

func (s *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/notes")
	key = strings.TrimPrefix(key, "/")
	if r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2" {
		s.serveList(w)
		return
	}
	if r.Method == http.MethodGet && key != "" {
		s.serveGet(w, key)
		return
	}
	http.NotFound(w, r)
}

func (s *fakeS3) serveList(w http.ResponseWriter) {
	keys := make([]string, 0, len(s.objects))
	for key := range s.objects {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	w.Header().Set("Content-Type", "application/xml")
	fmt.Fprint(w, `<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">`)
	fmt.Fprint(w, `<Name>notes</Name><KeyCount>`, len(keys), `</KeyCount><IsTruncated>false</IsTruncated>`)
	for _, key := range keys {
		fmt.Fprintf(w, `<Contents><Key>%s</Key><LastModified>2026-05-08T00:00:00Z</LastModified><Size>%d</Size></Contents>`, key, len(s.objects[key]))
	}
	fmt.Fprint(w, `</ListBucketResult>`)
}

func (s *fakeS3) serveGet(w http.ResponseWriter, key string) {
	value, ok := s.objects[key]
	if !ok {
		http.NotFound(w, nil)
		return
	}
	s.mu.Lock()
	s.getCalls[key]++
	s.mu.Unlock()
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(value))
}

func (s *fakeS3) getObjectCalls(key string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getCalls[key]
}
