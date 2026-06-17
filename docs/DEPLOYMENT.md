# Deployment

CaptureQuest deploys as a Vite static frontend plus a Go server. The runtime
database target is Postgres.

## Services

| Service | Purpose | Notes |
| --- | --- | --- |
| CaptureQuest frontend | Static Vite build | Serve `dist/` through the edge/reverse proxy. |
| CaptureQuest server | Go binary | Handles HTTP API, WebSocket fallback, and WebTransport. |
| Postgres | Runtime database | Set through `DATABASE_URL`. |

## Required Environment

| Variable | Purpose |
| --- | --- |
| `DATABASE_URL` | Postgres connection string. |
| `HTTP_PORT` | HTTP API port. Defaults to the configured value. |
| `HASH_PORT` | WebTransport certificate-hash HTTP port. |
| `ADMIN_KEY` | Admin API authentication secret. |
| `VITE_ASSET_URL` | Optional CDN/base URL for remote assets. |

## GitHub Actions Deploy

The deploy workflow builds the frontend and server, then uploads them to a
configured host over SSH. Configure these values in the GitHub repository
settings before enabling the workflow:

| Name | Type | Purpose |
| --- | --- | --- |
| `DEPLOY_SSH_KEY` | Secret | Private SSH key used by the action. |
| `DEPLOY_HOST` | Secret | SSH destination, for example `ubuntu@example.com`. |
| `DEPLOY_APP_DIR` | Repository variable | Absolute path to the app directory on the host. |

## Build

```bash
npm ci
npm run build

cd server
go test ./...
go build -o /tmp/capturequest-server ./cmd/server
```

## Database Bootstrap

On a fresh database, generate assets and apply the runtime schema through the
top-level bootstrap path:

```bash
DATABASE_URL="$DATABASE_URL" npm run bootstrap:fresh
```

The importer is deterministic for extractor-generated static data and
scripted-event JSON. It should be safe to rerun after source data changes.

## WebTransport

Route normal HTTPS traffic to the server's HTTP API port. WebTransport uses HTTP/3
over UDP and connects to `/cq`; make sure the configured UDP port is open and
reaches the Go server directly.

The frontend fetches the certificate hash through the HTTP API and pins the
WebTransport connection in the browser.

## Operational Checks

- `npm run build` succeeds before uploading frontend assets.
- `cd server && go test ./...` succeeds before replacing the server binary.
- The server starts with `DATABASE_URL` set and logs a successful world-manager
  initialization.
- The bootstrap/importer has been rerun after extractor pipeline changes or
  scripted-event generation changes.
