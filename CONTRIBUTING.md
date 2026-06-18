# Contributing to PADS

Thank you for your interest in contributing to PADS. This document outlines the guidelines and workflow for contributors.

---

## Language

- **Code**: English only (Go, comments, variable names).
- **Commit messages**: English, imperative mood, 72 chars max per line.
- **Documentation**: English (README, DESIGN.md, DEPLOY.md).

---

## Commit Conventions

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>: <short description>

[optional body]
```

### Types

| Type | Use for |
|------|---------|
| `feat` | New feature |
| `fix` | Bug fix |
| `security` | Security fix |
| `perf` | Performance improvement |
| `refactor` | Code restructure without behavior change |
| `docs` | Documentation only |
| `test` | Adding or fixing tests |
| `chore` | Build, CI, dependencies |

### Rules

- One logical change per commit (atomic commits).
- Subject line: imperative, no period, 72 chars max.
- Body: explain *what* and *why*, not *how*.
- Reference issues: `Fixes #123` or `See #456`.

### Examples

```
security: replace deterministic readRand with crypto/rand

The readRand() function was using a predictable LCG (linear congruential
generator) to generate agent IDs. This is a security risk as IDs could
be predicted by an attacker. Replaced with crypto/rand.Read().
```

```
fix: add background cleanup to RateLimiter to prevent unbounded map growth

The requests map in RateLimiter was never cleaned, causing unbounded
memory growth with many distinct tokens. Added a 5-minute cleanup loop
that evicts inactive tokens and enforces a maxTokens limit (1000).
```

```
feat: add /rotate endpoint for token rotation

Allows authenticated users to rotate the bearer token via POST /rotate.
The new token is written to token.txt and returned in the response.
```

---

## Running Tests

```bash
# All tests
go test ./... -count=1

# With race detection (Linux/macOS only)
go test ./... -race -count=1

# Specific package
go test ./internal/policy/evolution -v -count=1

# Coverage report
go test ./... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

**Note**: Race detection is not supported on Android/arm64. Tests on this platform skip the `-race` flag automatically.

---

## Building

```bash
# Build all binaries
go build ./...

# Build specific binary
go build -o evolution-api ./cmd/evolution-api
go build -o evolution-replay ./cmd/evolution-replay
go build -o pads ./cmd/pads
go build -o pads-ci ./cmd/pads-ci
```

---

## Code Style

- Run `go fmt ./...` before committing.
- Run `go vet ./...` — no warnings.
- Prefer `slog` over `log` for structured logging.
- Error handling: always check and propagate errors (no silent `_, _ = ...` unless intentionally ignored).

---

## Environment Variables

Do **not** hardcode API keys or tokens. Use environment variables:

| Variable | Purpose |
|----------|---------|
| `PADS_TOKEN` | Bearer token for API authentication |
| `OPENAI_API_KEY` | OpenAI GPT integration |
| `ANTHROPIC_API_KEY` | Anthropic Claude integration |
| `NVIDIA_API_KEY` | NVIDIA NIM integration (default LLM provider) |

---

## Security Rules

1. **Never commit secrets** — use `.gitignore`, environment variables, or GitHub Secrets.
2. **Sanitize logs** — never log tokens, API keys, or passwords.
3. **Validate input** — all HTTP request bodies must be validated before use.
4. **Use parameterized queries** — never interpolate user input into SQL strings.
5. **Check `git diff --staged`** before every commit to ensure no secrets are included.

---

## Pull Request Workflow

1. Fork the repository.
2. Create a feature branch: `git checkout -b feat/my-feature`.
3. Make your changes and commit (follow the commit conventions above).
4. Ensure `go build ./...` and `go test ./... -count=1` pass.
5. Open a PR with a clear description of what changed and why.
6. Address review feedback.
7. Squash commits before merging if needed.

---

## Reporting Issues

- Use the GitHub issue tracker.
- Security vulnerabilities: **do not** open a public issue. Report privately.
- Include: Go version, OS, steps to reproduce, and minimal reproduction case if possible.