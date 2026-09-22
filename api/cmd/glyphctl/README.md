# glyphctl

A command-line client for the [Glyph API](../../README.md). It talks to a
running Glyph server over HTTP, authenticating with an OAuth bearer token, and
exposes the pages, tasks, lanes, and templates resources — plus a helper for
minting a token via the `client_credentials` grant.

## Build

```bash
# from the repo root
make build-cli          # builds ./api/bin/glyphctl

# or directly
cd api && go build -o bin/glyphctl ./cmd/glyphctl
```

## Configuration

Every setting can come from a flag or an environment variable. Global flags must
appear **before** the command.

| Flag | Environment | Default | Description |
|---|---|---|---|
| `--url` | `GLYPH_API_URL` | `http://localhost:8080` | Base URL of the server |
| `--token` | `GLYPH_TOKEN` | — | OAuth bearer access token |
| `--json` | — | `false` | Emit raw JSON instead of formatted tables |

List views print a compact table by default; pass `--json` for the full payload.
Single-object results (`get`, `create`, `update`) always print JSON.

## Authentication

Most commands need a bearer token. Obtain one from the server's OAuth token
endpoint with the `client_credentials` grant (see the API's OAuth client admin
routes for how to create a client and scope it to an org):

```bash
export GLYPH_TOKEN=$(glyphctl auth token \
  --client-id "$CLIENT_ID" \
  --client-secret "$CLIENT_SECRET" \
  --subject alice@example.com \
  --org "$ORG_UUID" \
  --scope "task:read task:write page:read")
```

`auth token` prints just the access token on stdout (so it is easy to capture),
with token metadata on stderr. Use `--json` for the full response.

`--client-id` / `--client-secret` also read from `GLYPH_CLIENT_ID` /
`GLYPH_CLIENT_SECRET`.

## Commands

```
glyphctl health                          # liveness check (no auth)
glyphctl auth token [flags]              # mint a bearer token

glyphctl pages list
glyphctl pages get <id>
glyphctl pages create --title "Notes" [--type page|folder] [--parent <id>] [--org <id>] [--tags a,b] [--priority high]
glyphctl pages delete <id>
glyphctl pages content-get <id>
glyphctl pages content-set <id> --file doc.json     # or --content '{...}' or --file -

glyphctl tasks list
glyphctl tasks get <id>
glyphctl tasks create --title "Do it" [--status todo] [--priority high] [--due 2026-01-31] [--description ...] [--tags a,b] [--org <id>]
glyphctl tasks update <id> [--title ...] [--status done] [--priority low] [--due ...] [--description ...] [--tags a,b]
glyphctl tasks delete <id>

glyphctl lanes list
glyphctl lanes get <id>
glyphctl lanes create --title "In Progress" [--order 1]
glyphctl lanes delete <id>

glyphctl templates list
glyphctl templates get <id>
glyphctl templates create --name "Standup" [--content ...] [--title-template ...] [--default] [--org <id>]
glyphctl templates delete <id>
```

## Examples

```bash
# Check the server is up
glyphctl --url https://glyph.example.com health

# List your tasks as a table
glyphctl --token "$GLYPH_TOKEN" tasks list

# Create a task and get the JSON back
glyphctl --token "$GLYPH_TOKEN" tasks create --title "Ship the CLI" --priority high

# Mark it done
glyphctl --token "$GLYPH_TOKEN" tasks update <task-id> --status done

# Pipe a page's ProseMirror document in from a file
glyphctl --token "$GLYPH_TOKEN" pages content-set <page-id> --file page.json
```
