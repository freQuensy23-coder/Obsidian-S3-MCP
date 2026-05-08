# Obsidian S3 MCP

Read/write MCP server for an Obsidian vault stored in S3-compatible storage.

It exists so an agent can inspect and edit notes without syncing the whole vault locally. This project is built for an Obsidian setup that uses the Remotely Save/RemotelySafe plugin to automatically sync the vault into an S3 bucket.

The server understands common Obsidian markdown patterns such as wikilinks, embeds, tags, backlinks, and attachment keys. Excalidraw and Dataview blocks are returned as raw markdown.

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
- `write_note`: replace the full markdown body of a note.
- `append_note`: append markdown text to a note.
- `replace_in_note`: edit like Claude Code's file edit tool. `old_text` must match exactly and must be unique unless `replace_all` is true.
- `add_note_tags`: add existing Obsidian tag-note links such as `[[0000.Life]]` or `[[0001.ML]]`. The tool resolves inputs like `Life`, `0000.Life`, or `[[0000.Life]]` against existing `0000.*.md` / `0001.*.md` notes before editing.

## Coolify

The repository includes `Dockerfile` and `docker-compose.yml` for Coolify.

Required environment variables:

- `NOTES_MCP_TOKEN`
- `NOTES_MCP_S3_ENDPOINT`
- `NOTES_MCP_S3_REGION`
- `NOTES_MCP_S3_ACCESS_KEY_ID`
- `NOTES_MCP_S3_SECRET_ACCESS_KEY`
- `NOTES_MCP_S3_BUCKET`
- `NOTES_MCP_S3_INSECURE_SKIP_VERIFY`

The compose file exposes the MCP server on port `8080` and maps Traefik to `/mcp`.

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
