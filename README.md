# SEO Rank Guardian

A Go backend API for SEO rank tracking, built with Fiber, PostgreSQL, Redis, NATS, JWT authentication, and Casbin RBAC.

## Prerequisites

- Go 1.25+
- Docker & Docker Compose

## Quick Start

```bash
# Clone the repository
git clone https://github.com/zeelrupapara/seo-rank-guardian.git
cd seo-rank-guardian

# Setup environment
cp .env.example .env
mkdir -p logs

# Start infrastructure (PostgreSQL, Redis, NATS)
docker compose up -d

# Run the server
go run main.go start
```

The server starts at `http://localhost:8080`.

## Makefile Commands

| Command | Description |
|---------|-------------|
| `make run` | Start the API server |
| `make worker` | Start the background worker |
| `make dev` | Start with hot-reload (air) |
| `make build` | Build binary to `bin/srg` |
| `make test` | Run all tests |
| `make tidy` | Run `go mod tidy` |
| `make swagger` | Generate Swagger docs |
| `make docker-up` | Start Docker services |
| `make docker-down` | Stop Docker services |

## API Endpoints

### Health

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/health` | Health check |

### Authentication

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/auth/register` | Register a new user |
| `POST` | `/api/v1/auth/login` | Login with email/password |
| `POST` | `/api/v1/auth/refresh` | Refresh access token |
| `DELETE` | `/api/v1/auth/logout` | Logout (requires auth) |
| `GET` | `/api/v1/auth/google` | Initiate Google OAuth login |
| `GET` | `/api/v1/auth/google/callback` | Google OAuth callback |

### Users

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/users/me` | Get current user profile (requires auth) |
| `PUT` | `/api/v1/users/me` | Update current user profile (requires auth) |

### Jobs (SEO Tracking)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/jobs` | Create a new job config (requires auth) |
| `GET` | `/api/v1/jobs` | List all jobs (requires auth) |
| `GET` | `/api/v1/jobs/:jobId` | Get job with summary stats (requires auth) |
| `PUT` | `/api/v1/jobs/:jobId` | Full replace job config (requires auth) |
| `DELETE` | `/api/v1/jobs/:jobId` | Delete a job (requires auth) |
| `POST` | `/api/v1/jobs/:jobId/scrape` | Trigger manual scrape (requires auth) |

### Runs

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/jobs/:jobId/runs` | List run history (requires auth) |
| `GET` | `/api/v1/jobs/:jobId/runs/:runId` | Get run detail with rankings, diffs, report (requires auth) |

### Request/Response Examples

**Register**
```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"myuser","email":"user@example.com","password":"password123"}'
```

**Login**
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"password123"}'
```

**Refresh Token**
```bash
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"<refresh_token>"}'
```

**Protected Endpoints** — pass `Authorization: Bearer <access_token>` header.

**Create Job**
```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <access_token>" \
  -d '{
    "name": "Fintech Georgia",
    "domain": "fintech.com",
    "schedule_time": "16:00",
    "competitors": ["legalzoom.com", "nerdwallet.com"],
    "keywords": ["financial advisor", "business loans"],
    "regions": [{"country": "US", "state": "Georgia"}]
  }'
```

**Trigger Scrape**
```bash
curl -X POST http://localhost:8080/api/v1/jobs/1/scrape \
  -H "Authorization: Bearer <access_token>"
```

**Get Run Details**
```bash
curl http://localhost:8080/api/v1/jobs/1/runs/1 \
  -H "Authorization: Bearer <access_token>"
```

**Google OAuth** — navigate to `/api/v1/auth/google` in a browser to start the OAuth flow. Requires `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` to be configured.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_NAME` | `seo-rank-guardian` | Application name |
| `APP_ENV` | `development` | Environment |
| `HTTP_HOST` | `0.0.0.0` | Server host |
| `HTTP_PORT` | `8080` | Server port |
| `POSTGRES_HOST` | `localhost` | PostgreSQL host |
| `POSTGRES_PORT` | `5432` | PostgreSQL port |
| `POSTGRES_USER` | `srg` | PostgreSQL user |
| `POSTGRES_PASSWORD` | `srg_secret` | PostgreSQL password |
| `POSTGRES_DB` | `srg_db` | PostgreSQL database |
| `POSTGRES_SSLMODE` | `disable` | PostgreSQL SSL mode |
| `REDIS_HOST` | `localhost` | Redis host |
| `REDIS_PORT` | `6379` | Redis port |
| `REDIS_PASSWORD` | | Redis password |
| `REDIS_DB` | `0` | Redis database |
| `NATS_URL` | `nats://localhost:4222` | NATS URL |
| `LOG_LEVEL` | `debug` | Log level |
| `LOG_FILE` | `logs/srg.log` | Log file path |
| `OAUTH_ACCESS_SECRET` | | JWT access token secret |
| `OAUTH_REFRESH_SECRET` | | JWT refresh token secret |
| `OAUTH_ACCESS_EXPIRY` | `15m` | Access token TTL |
| `OAUTH_REFRESH_EXPIRY` | `168h` | Refresh token TTL |
| `GOOGLE_CLIENT_ID` | | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | | Google OAuth client secret |
| `GOOGLE_REDIRECT_URL` | `http://localhost:8080/api/v1/auth/google/callback` | Google OAuth redirect URL |
| `AI_PROVIDER` | `gemini` | AI provider for reports |
| `AI_API_KEY` | | AI provider API key |
| `SCRAPE_RESULT_LIMIT` | `10` | Max Google results per query |
| `SCRAPE_MIN_DELAY_MS` | `3000` | Min delay between scrapes (ms) |
| `SCRAPE_MAX_DELAY_MS` | `7000` | Max delay between scrapes (ms) |
| `SCRAPE_MAX_RETRIES` | `3` | Max retries per scrape |

## Project Structure

```
.
├── app/            # Application bootstrap
├── cmd/            # CLI commands (start, worker)
├── config/         # Configuration loading
├── internal/
│   ├── middleware/  # Auth, RBAC, logging middleware
│   └── server/     # HTTP server and route handlers
├── model/          # GORM models
├── pkg/
│   ├── ai/         # AI client (Gemini)
│   ├── authz/      # Casbin authorization
│   ├── cache/      # Redis cache wrapper
│   ├── db/         # PostgreSQL connection
│   ├── errors/     # Application errors
│   ├── http/       # Fiber app setup and response helpers
│   ├── logger/     # Zap logger
│   ├── nats/       # NATS client
│   ├── oauth2/     # JWT token management and Google OAuth
│   ├── redis/      # Redis client
│   ├── scraper/    # Google SERP scraper (Colly)
│   └── seed/       # Database seeding
├── utils/          # Utility functions
├── worker/         # Background workers (scrape, detect, report)
├── compose.yml     # Docker Compose (PostgreSQL, Redis, NATS)
├── Dockerfile      # Container build
└── Makefile        # Build commands
```
