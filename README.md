# PocketBase + Stripe

The all-in-one starter kit for high-performance SaaS applications. Frontend-agnostic backend with [PocketBase](https://pocketbase.io) and [Stripe](https://stripe.com) integration.

[![Deploy on Railway](https://railway.app/button.svg)](https://railway.app/template/izeSvS?referralCode=9ynQqF)

## Features

- User management and authentication with [PocketBase](https://pocketbase.io/docs/authentication)
- Powerful data access & management on top of SQLite with [PocketBase](https://pocketbase.io/docs/guides/database)
- Integration with [Stripe Checkout](https://stripe.com/docs/payments/checkout) and [Stripe Customer Portal](https://stripe.com/docs/billing/subscriptions/customer-portal)
- Automatic syncing of pricing plans and subscription statuses via [Stripe webhooks](https://stripe.com/docs/webhooks)

## Quick Start

### Prerequisites

- [Go 1.24+](https://go.dev/dl/)
- [Task](https://taskfile.dev/installation/) (task runner)
- [Stripe CLI](https://stripe.com/docs/stripe-cli) (installed via `task stripe:1-install`)

### Setup

```bash
# Clone and enter the repo ( or your fork )
git clone https://github.com/mrwyndham/pocketbase-stripe
cd pocketbase-stripe

# One-time Stripe setup
task stripe:1-install    # Install Stripe CLI
task stripe:2-login      # Authenticate with Stripe
task stripe:3-env        # Create .env from template
task stripe:4-fixtures   # Load test products

# One-time PocketBase setup
task pb:1-superuser      # Create admin account
task pb:2-schema         # Import collections schema

# Run the application
task pc:up               # Recommended: process-compose (PocketBase + Stripe listener)
# OR
task dev                 # Direct: just PocketBase (run stripe:5-listen separately)
# OR
task docker:build && task docker:run  # Docker container
```

## Development Options

See **[DEVELOPMENT.md](DEVELOPMENT.md)** for detailed development workflows including:

- **process-compose** - Runs PocketBase + Stripe listener together with TUI
- **Docker** - Containerized environment for CI/CD
- **Direct** - Simple `go run` for quick iteration

```bash
task --list              # See all available tasks
task info                # Show current configuration
task debug               # Print all Taskfile vars
```

## Configuration

All approaches use the same `.env` file. See `.env.example`:

| Variable | Purpose |
|----------|---------|
| `STRIPE_SECRET_KEY` | Stripe API key (sk_test_xxx) |
| `STRIPE_WHSEC` | Webhook signing secret (whsec_xxx) |
| `STRIPE_SUCCESS_URL` | Checkout success redirect |
| `STRIPE_CANCEL_URL` | Checkout cancel redirect |
| `PB_HOST` | Server bind address (default: 0.0.0.0) |
| `PB_PORT` | Server port (default: 8090) |

## Stripe Setup Details

### Create Products and Prices

Your webhook listens for product updates on Stripe and automatically syncs them to PocketBase. Create products in the [Stripe Dashboard](https://dashboard.stripe.com/test/products) or use our fixtures:

```bash
task stripe:4-fixtures   # Creates Hobby ($10/mo) and Freelancer ($20/mo) plans
```

### Configure Customer Portal

1. Set branding in [Stripe Settings](https://dashboard.stripe.com/settings/branding)
2. Configure [Customer Portal](https://dashboard.stripe.com/test/settings/billing/portal):
   - Enable "Allow customers to update payment methods"
   - Enable "Allow customers to update/cancel subscriptions"
   - Add your products and prices

### Production Webhooks

For production, create a webhook endpoint in [Stripe Dashboard](https://dashboard.stripe.com/webhooks):

```bash
task stripe:webhook:prod  # Shows setup instructions
```

Endpoint URL: `https://YOUR_DOMAIN/stripe`

Events to listen for:
- `product.*`, `price.*`
- `customer.*`, `customer.subscription.*`
- `checkout.session.completed`

## Going Live

1. Archive all test mode products in Stripe
2. Switch Stripe to production mode
3. Update `.env` with production API keys
4. Create production webhook endpoint
5. Rebuild and deploy

## Architecture

```
├── main.go              # PocketBase app with Stripe hooks
├── hooks/               # JSVM hooks (JavaScript)
├── pb_bootstrap/        # PocketBase schema
├── stripe_bootstrap/    # Stripe fixtures
├── Taskfile.yml         # Task runner config
├── taskfiles/           # Modular task definitions
├── process-compose.yml  # Multi-service runner
└── Dockerfile           # Container build
```

## Reliability

Stripe webhooks are retried for up to 3 days if the database is unavailable. Transactions will eventually sync when the database comes back online.

## Credits

- Based on [vercel/nextjs-subscription-payments](https://github.com/vercel/nextjs-subscription-payments)
- Original author: [Samuel Wyndham](https://twitter.com/meinbiz)
- Fork maintainer: [joeblew999](https://github.com/joeblew999)

## Sponsors

- [XAM Consulting](https://xam.com.au/)

## License

MIT License - See [LICENSE](LICENSE) for details.
