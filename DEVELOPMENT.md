# Development Options

This project supports multiple approaches. Use whichever works best for you.

## One-Time Setup (All Options)

```bash
# Stripe setup
task stripe:1-install
task stripe:2-login
task stripe:3-env
task stripe:4-fixtures

# PocketBase setup
task pb:1-superuser
task pb:2-schema
```

## Option 1: process-compose (Recommended)

Runs PocketBase + Stripe listener together with health checks and auto-restart:

```bash
task pc:up              # Foreground with TUI
task pc:up:detached     # Background
task pc:logs
task pc:down
```

## Option 2: Docker (Containerized)

Isolated environment, useful for CI/CD or when you need full container isolation:

```bash
task docker:build
task docker:run           # Foreground
task docker:run:detached  # Background
task docker:logs
task docker:stop
```

## Option 3: Direct (Simple)

Run PocketBase directly without process management:

```bash
task dev                  # Run with go run
task build && task serve  # Build and run binary
```

Note: Run `task stripe:5-listen` separately in another terminal.

## Quick Reference

```bash
task --list          # See all available tasks
task info            # Show current configuration
task docker:info     # Show Docker status
```

## Environment Variables

All approaches share the same `.env` file. See `.env.example`:

| Variable | Purpose |
|----------|---------|
| `STRIPE_SECRET_KEY` | Stripe API key (sk_test_xxx) |
| `STRIPE_WHSEC` | Webhook signing secret (whsec_xxx) |
| `STRIPE_SUCCESS_URL` | Checkout success redirect |
| `STRIPE_CANCEL_URL` | Checkout cancel redirect |
| `PB_HOST` | Server bind address (default: 0.0.0.0) |
| `PB_PORT` | Server port (default: 8090) |

## Why Taskfile?

- **Reproducible setup**: New developers onboard with numbered steps
- **Easier upgrades**: Version numbers in one place, documented migration
- **Process management**: Health checks, auto-restart, logs
- **Unified config**: Same `.env` works for all approaches
