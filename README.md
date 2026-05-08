# notesMCP

Read-only MCP server for an Obsidian vault stored in S3-compatible storage.

It exists so an agent can inspect notes without syncing the whole vault locally. The server understands common Obsidian markdown patterns such as wikilinks, embeds, tags, backlinks, and attachment keys. Excalidraw and Dataview blocks are returned as raw markdown.

## Run

Create an env file with five lines:

```text
s3.example.com
region
access-key
secret-key
bucket-name
```

Start the server:

```bash
export NOTES_MCP_TOKEN="$(openssl rand -hex 32)"
export NOTES_MCP_ENV_FILE=.env
export NOTES_MCP_ADDR=127.0.0.1:8080
go run ./cmd/notesmcp
```

If the S3 endpoint uses a local or self-signed certificate chain:

```bash
export NOTES_MCP_S3_INSECURE_SKIP_VERIFY=true
```

Use the MCP endpoint with bearer auth:

```bash
curl -s http://127.0.0.1:8080/mcp \
  -H "Authorization: Bearer $NOTES_MCP_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"vault_overview","arguments":{}}}'
```

## Tools

- `vault_overview`: count objects, markdown notes, folders, extensions.
- `list_notes`: list markdown notes without bodies.
- `get_note`: read one markdown note by S3 key.
- `search_notes`: search by key, title, or markdown body.
- `get_backlinks`: list notes linking to a note.

## Architecture

- `internal/handler/mcp`: JSON-RPC MCP HTTP handler.
- `internal/usecase/vault`: note listing, reading, search, backlinks.
- `internal/storage/s3store`: S3-compatible object storage adapter.
- `internal/storage/cache`: in-memory storage cache with 30 second TTL.
- `internal/obsidian`: Obsidian markdown parsing.
- `internal/domain`: shared types and interfaces.

## Test

```bash
go test ./...
```

The exam test in `internal/exam` starts a real MinIO S3-compatible container, creates a bucket, uploads sample Obsidian files through AWS SDK, calls the MCP endpoint, and verifies caching. Docker must be running.
