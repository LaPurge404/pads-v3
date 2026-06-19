# Deployment Guide

This guide covers building, configuring, and running the PADS API in production.

---

## Prerequisites

- Go 1.22 or later
- Linux/macOS (some features such as `go test -race` are not supported on Android/arm64)
- Network access on port 8080 (or custom port)

---

## Building

### Build all binaries

```bash
go build ./...
```

This produces the following binaries:

| Binary | Purpose |
|--------|---------|
| `evolution-api` | Main HTTP API server |
| `evolution-replay` | Event replay CLI tool |
| `pads` | Main PADS CLI |
| `pads-ci` | CI integration client |

### Build specific binary

```bash
go build -o evolution-api ./cmd/evolution-api
```

---

## Configuration

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `PADS_TOKEN` | Recommended | Bearer token for API authentication. If not set, a random token is generated and stored in `token.txt`. |
| `OPENAI_API_KEY` | For GPT | OpenAI API key for `OpenAIClient` LLM integration. |
| `ANTHROPIC_API_KEY` | For Claude | Anthropic API key for `ClaudeClient` LLM integration. |
| `NVIDIA_API_KEY` | For NVIDIA | NVIDIA NIM API key for the default LLM provider. |

### Token

The API uses a Bearer token for authentication. Token resolution order:

1. `PADS_TOKEN` environment variable
2. `token.txt` file in the working directory
3. Auto-generated 128-bit random token (written to `token.txt` on first run)

**⚠️ Important**: Change the default token for any non-local deployment.

### Rotating the Token

```bash
# Via API (requires current token):
curl -X POST http://127.0.0.1:8080/rotate \
  -H "Authorization: Bearer YOUR_CURRENT_TOKEN"

# Response: {"status": "rotated", "token": "new-token-here"}
```

---

## Running the API

### Basic launch

```bash
./evolution-api
```

By default, the API listens on `127.0.0.1:8080`.

### With TLS

Generate a self-signed certificate for testing:

```bash
# Generate a private key
openssl genrsa -out server.key 2048

# Generate a self-signed certificate
openssl req -new -x509 -key server.key -out server.crt -days 365 \
  -subj "/CN=localhost"

# Launch with TLS
./evolution-api -cert server.crt -key server.key
```

For production, use certificates from Let's Encrypt or your CA.

### With a custom token file

```bash
./evolution-api -token-file /etc/pads/token.txt
```

### With a custom timeout

```bash
./evolution-api -timeout 60s
```

The default timeout is 30 seconds.

---

## Systemd Service (Linux)

Create `/etc/systemd/system/pads.service`:

```ini
[Unit]
Description=PADS Evolution API
After=network.target

[Service]
Type=simple
User=pads
Group=pads
WorkingDirectory=/opt/pads
ExecStart=/opt/pads/evolution-api
Environment="PADS_TOKEN=your-secret-token"
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

Then:

```bash
sudo systemctl daemon-reload
sudo systemctl enable pads
sudo systemctl start pads
```

---

## Health Check

```bash
curl http://127.0.0.1:8080/health
# Response: JSON, e.g. {"db":true,"wal":true,"semantic_memory":true,"worker":true}
```

No authentication required for `/health`.

---

## API Endpoints Reference

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/` | No | Dashboard HTML |
| `GET` | `/health` | No | Liveness probe |
| `GET` | `/dashboard/enriched` | No | Enhanced dashboard |
| `POST` | `/rotate` | Yes | Rotate bearer token |
| `POST` | `/evolve` | Yes | Submit evolution candidate |
| `GET` | `/state` | Yes | Full system state |
| `GET` | `/select` | Yes | Current UCB arm |
| `GET` | `/workspace` | Yes | Git + test status |
| `POST` | `/agent/evolve` | Yes | CodeAgent evolution |
| `GET` | `/agent/status` | Yes | UCB agent statistics |
| `GET` | `/agent/strategies` | Yes | List agent strategies |

All protected endpoints require: `Authorization: Bearer <token>`

---

## Files and Artifacts

The API creates the following files in its working directory:

| File | Purpose |
|------|---------|
| `token.txt` | Bearer token persistence |
| `evolution.wal` | WAL (Write-Ahead Log) — **not committed to git** |
| `evolution.log` | Event store log |
| `event_queue.log` | Event queue log |
| `worker_offset.txt` | Worker read offset for crash recovery |

These files are automatically ignored by `.gitignore`.

---

## Security Checklist

- [ ] Change the default Bearer token (`PADS_TOKEN`)
- [ ] Use TLS in production (`-cert` + `-key`)
- [ ] Restrict listening address (`127.0.0.1` is default; bind to localhost even behind a reverse proxy)
- [ ] Set `maxTokens` on the RateLimiter if expecting many distinct clients (default: 1000)
- [ ] Monitor `event_queue.log` for replay on restart
- [ ] Run the API as a non-root user

---

## CI/CD Integration

See `.github/workflows/pads-ci.yml` for the GitHub Actions pipeline. The `pads-ci` client (`cmd/pads-ci`) interacts with the API using:

```bash
go run ./cmd/pads-ci -token "YOUR_TOKEN"
```

In GitHub Actions, use secrets:

```yaml
env:
  PADS_TOKEN: ${{ secrets.PADS_TOKEN }}
```