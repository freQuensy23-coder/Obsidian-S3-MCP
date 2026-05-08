package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"notesmcp/internal/domain"
)

type Handler struct {
	vault domain.VaultUseCase
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id,omitempty"`
	Result  any            `json:"result,omitempty"`
	Error   *responseError `json:"error,omitempty"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func NewHandler(v domain.VaultUseCase) http.Handler {
	return &Handler{vault: v}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, response{JSONRPC: "2.0", Error: rpcError(-32700, "parse error")})
		return
	}

	result, err := h.dispatch(r.Context(), req)
	if err != nil {
		writeJSON(w, response{JSONRPC: "2.0", ID: req.ID, Error: rpcError(-32603, err.Error())})
		return
	}
	writeJSON(w, response{JSONRPC: "2.0", ID: req.ID, Result: result})
}

func (h *Handler) dispatch(ctx context.Context, req request) (any, error) {
	switch req.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]any{"name": "notesmcp", "version": "0.1.0"},
			"capabilities":    map[string]any{"tools": map[string]any{}},
		}, nil
	case "tools/list":
		return map[string]any{"tools": tools()}, nil
	case "tools/call":
		var params toolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, err
		}
		return h.callTool(ctx, params)
	default:
		return nil, errors.New("unknown method")
	}
}

func (h *Handler) callTool(ctx context.Context, params toolCallParams) (any, error) {
	switch params.Name {
	case "vault_overview":
		return h.vault.Overview(ctx)
	case "list_notes":
		return h.vault.ListNotes(ctx)
	case "get_note":
		var args struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return nil, err
		}
		return h.vault.GetNote(ctx, args.Key)
	case "search_notes":
		var args struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return nil, err
		}
		return h.vault.SearchNotes(ctx, args.Query, args.Limit)
	case "get_backlinks":
		var args struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return nil, err
		}
		return h.vault.Backlinks(ctx, args.Key)
	case "write_note":
		var args struct {
			Key  string `json:"key"`
			Body string `json:"body"`
		}
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return nil, err
		}
		return h.vault.WriteNote(ctx, args.Key, args.Body)
	case "append_note":
		var args struct {
			Key  string `json:"key"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return nil, err
		}
		return h.vault.AppendNote(ctx, args.Key, args.Text)
	case "replace_in_note":
		var args struct {
			Key        string `json:"key"`
			OldText    string `json:"old_text"`
			NewText    string `json:"new_text"`
			ReplaceAll bool   `json:"replace_all"`
		}
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return nil, err
		}
		return h.vault.ReplaceInNote(ctx, args.Key, args.OldText, args.NewText, args.ReplaceAll)
	case "add_note_tags":
		var args struct {
			Key  string   `json:"key"`
			Tags []string `json:"tags"`
		}
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return nil, err
		}
		return h.vault.AddNoteTags(ctx, args.Key, args.Tags)
	default:
		return nil, errors.New("unknown tool")
	}
}

func tools() []map[string]any {
	return []map[string]any{
		{"name": "vault_overview", "description": "Summarize the Obsidian vault stored in S3."},
		{"name": "list_notes", "description": "List markdown notes without bodies."},
		{"name": "get_note", "description": "Read one markdown note by S3 key."},
		{"name": "search_notes", "description": "Search notes by key, title, or markdown body."},
		{"name": "get_backlinks", "description": "List notes linking to a markdown note."},
		{"name": "write_note", "description": "Replace the full markdown body of a note."},
		{"name": "append_note", "description": "Append markdown text to a note."},
		{"name": "replace_in_note", "description": "Replace one or all exact text fragments inside a note."},
		{"name": "add_note_tags", "description": "Add Obsidian tag links such as [[0000.Life]] to a note."},
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func rpcError(code int, message string) *responseError {
	return &responseError{Code: code, Message: message}
}
