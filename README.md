<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?style=for-the-badge&logo=go&logoColor=white" />
  <img src="https://img.shields.io/badge/Python-3.13-3776AB?style=for-the-badge&logo=python&logoColor=white" />
  <img src="https://img.shields.io/badge/Fiber-v2-00ACD7?style=for-the-badge&logo=go&logoColor=white" />
  <img src="https://img.shields.io/badge/PostgreSQL-16-4169E1?style=for-the-badge&logo=postgresql&logoColor=white" />
  <img src="https://img.shields.io/badge/Redis-7-DC382D?style=for-the-badge&logo=redis&logoColor=white" />
  <img src="https://img.shields.io/badge/NATS-2.10-27AAE1?style=for-the-badge&logo=natsdotio&logoColor=white" />
  <img src="https://img.shields.io/badge/Gemini_AI-2.5-8E75B2?style=for-the-badge&logo=googlegemini&logoColor=white" />
  <img src="https://img.shields.io/badge/License-MIT-green?style=for-the-badge" />
</p>

<h1 align="center">SEO Rank Guardian</h1>

<p align="center">
  <strong>Automated SEO rank tracking with AI-powered competitive intelligence</strong>
</p>

<p align="center">
  Real-time Google SERP monitoring &bull; Patchright browser scraping &bull; Gemini AI reports &bull; WebSocket live updates
</p>

---

## Overview

SEO Rank Guardian is a full-stack SEO monitoring platform that tracks your Google search rankings across keywords and regions, detects rank changes, and generates AI-powered competitive analysis reports.

**Architecture**: Go REST API (Fiber) + Python async worker (Patchright + Gemini AI), connected via NATS JetStream message queue.

### How It Works

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
│   Frontend   │────▶│  Go API      │────▶│  NATS JetStream  │
│   (React)    │◀────│  (Fiber)     │     │  Message Queue   │
└─────────────┘     └──────────────┘     └────────┬────────┘
       ▲                   │                       │
       │              PostgreSQL              ┌────▼────────────┐
       │              Redis                   │  Python Worker   │
       └──────────── WebSocket ◀──────────────│  - Patchright    │
                                              │  - BeautifulSoup │
                                              │  - Gemini AI     │
                                              └─────────────────┘
```

1. **User creates a tracking job** — domain, keywords, regions, competitors
2. **User triggers a scan** — Go API publishes scrape tasks to NATS
3. **Python worker scrapes Google** — Patchright browser with residential proxy rotation
4. **Change detection** — compares current vs previous run positions
5. **AI report generation** — Gemini analyzes SERP data and produces competitive insights
6. **Real-time updates** — WebSocket pushes live progress to the frontend

---

## Tech Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| **API Server** | Go 1.25 + Fiber v2 | REST API, WebSocket, JWT auth |
| **Worker** | Python 3.13 + Patchright | Google scraping, AI reports |
| **Database** | PostgreSQL 16 | Persistent storage (GORM) |
| **Cache** | Redis 7 | Session cache, rate limiting |
| **Message Queue** | NATS 2.10 + JetStream | Async job processing |
| **AI Engine** | Google Gemini 2.5 | SEO analysis & report generation |
| **Scraper** | Patchright (patched Playwright) | Bypasses Google SearchGuard |
| **Auth** | JWT + Casbin RBAC | Access/refresh tokens, role-based access |
| **OAuth** | Google OAuth 2.0 | Social login |

### Key Dependencies

**Go API:**
- `gofiber/fiber` — HTTP framework with WebSocket support
- `gorm.io/gorm` — ORM with PostgreSQL driver
- `nats-io/nats.go` — NATS client with JetStream
- `golang-jwt/jwt` — JWT token generation and validation
- `casbin/casbin` — Role-based access control
- `redis/go-redis` — Redis client
- `swaggo/swag` — Auto-generated Swagger docs

**Python Worker:**
- `patchright` — Patched Playwright that bypasses Google's CDP detection
- `beautifulsoup4` + `lxml` — Structural HTML/SERP parsing
- `httpx` — Async HTTP client (Gemini API, Serper.dev fallback)
- `sqlalchemy` + `psycopg2` — Database access
- `nats-py` — NATS JetStream consumer
- `pydantic-settings` — Typed configuration
- `structlog` — Structured logging

---

## Prerequisites

- **Go** 1.25+
- **Python** 3.13+
- **Docker** & Docker Compose
- **Chromium** (auto-installed by Patchright)

### External Services (required)

| Service | Purpose | How to get |
|---------|---------|-----------|
| **Gemini API Key** | AI report generation | [Google AI Studio](https://aistudio.google.com/apikey) |

### External Services (optional)

| Service | Purpose | How to get |
|---------|---------|-----------|
| **Residential Proxy** | Rotate IPs to avoid Google rate limits | [DataImpulse](https://dataimpulse.com/) or similar |
| **Serper.dev API Key** | Fallback search API (2,500 free queries/month) | [serper.dev](https://serper.dev/) |
| **Google OAuth Credentials** | Social login | [Google Cloud Console](https://console.cloud.google.com/apis/credentials) |

---

## Quick Start

```bash
# 1. Clone
git clone https://github.com/zeelrupapara/seo-rank-guardian.git
cd seo-rank-guardian

# 2. Environment
cp .env.example .env
# Edit .env — at minimum set: OAUTH_ACCESS_SECRET, OAUTH_REFRESH_SECRET, AI_API_KEY

# 3. Start infrastructure
make docker-up

# 4. Start Go API server
make run
# API available at http://localhost:8081

# 5. Setup & start Python worker (separate terminal)
make worker-setup    # creates venv, installs deps, installs Chromium
make worker          # starts the async worker
```

### Development Mode (hot-reload)

```bash
# Go API with Air hot-reload
make dev

# Python worker
make worker
```

---

## Makefile Commands

| Command | Description |
|---------|-------------|
| `make run` | Start API server |
| `make dev` | Start with hot-reload (Air) |
| `make build` | Build binary to `bin/srg` |
| `make test` | Run all Go tests |
| `make tidy` | Run `go mod tidy` |
| `make swagger` | Generate Swagger docs |
| `make docker-up` | Start PostgreSQL, Redis, NATS |
| `make docker-down` | Stop Docker services |
| `make worker-setup` | Create Python venv and install deps |
| `make worker` | Run the Python worker |

---

## API Endpoints

> Full Swagger documentation available at `http://localhost:8081/swagger/`

### Authentication

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/auth/register` | Register new user |
| `POST` | `/api/v1/auth/login` | Login with email/password |
| `POST` | `/api/v1/auth/refresh` | Refresh access token |
| `DELETE` | `/api/v1/auth/logout` | Logout |
| `GET` | `/api/v1/auth/google` | Initiate Google OAuth |
| `GET` | `/api/v1/auth/google/callback` | OAuth callback |

### Users

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/users/me` | Get current user profile |
| `PUT` | `/api/v1/users/me` | Update profile |

### Jobs (SEO Tracking Projects)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/jobs` | Create tracking job |
| `GET` | `/api/v1/jobs` | List all jobs |
| `GET` | `/api/v1/jobs/:jobId` | Get job with stats |
| `PUT` | `/api/v1/jobs/:jobId` | Update job config |
| `DELETE` | `/api/v1/jobs/:jobId` | Delete job |
| `POST` | `/api/v1/jobs/:jobId/scrape` | Trigger manual scan |
| `GET` | `/api/v1/jobs/:jobId/stats` | Get job statistics |
| `GET` | `/api/v1/jobs/:jobId/rankings` | Get keyword rankings (paginated) |

### Scan History & Reports

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/jobs/:jobId/runs` | List scan runs |
| `GET` | `/api/v1/jobs/:jobId/runs/:runId` | Get run detail with diffs |
| `GET` | `/api/v1/jobs/:jobId/runs/:runId/report` | Get AI report |
| `GET` | `/api/v1/jobs/:jobId/reports` | List all reports |
| `GET` | `/api/v1/jobs/:jobId/trends` | Position trends over time |

### Keyword Pair Analysis

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/jobs/:jobId/pairs/:keyword/:state/summary` | Keyword pair summary |
| `GET` | `/api/v1/jobs/:jobId/pairs/:keyword/:state/scans` | Scan history for pair |
| `GET` | `/api/v1/jobs/:jobId/pairs/:keyword/:state/competitors` | Competitor analysis |

### WebSocket

| Path | Description |
|------|-------------|
| `/api/v1/ws` | Real-time scan progress, log events, report updates |

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| **App** | | |
| `APP_NAME` | `seo-rank-guardian` | Application name |
| `APP_ENV` | `development` | Environment |
| `HTTP_HOST` | `0.0.0.0` | Server bind host |
| `HTTP_PORT` | `8081` | Server port |
| **Database** | | |
| `POSTGRES_HOST` | `localhost` | PostgreSQL host |
| `POSTGRES_PORT` | `5433` | PostgreSQL port |
| `POSTGRES_USER` | `srg` | Database user |
| `POSTGRES_PASSWORD` | `srg_secret` | Database password |
| `POSTGRES_DB` | `srg_db` | Database name |
| `POSTGRES_SSLMODE` | `disable` | SSL mode |
| **Cache** | | |
| `REDIS_HOST` | `localhost` | Redis host |
| `REDIS_PORT` | `6380` | Redis port |
| **Message Queue** | | |
| `NATS_URL` | `nats://localhost:4223` | NATS connection URL |
| **Auth** | | |
| `OAUTH_ACCESS_SECRET` | — | JWT access token secret (required) |
| `OAUTH_REFRESH_SECRET` | — | JWT refresh token secret (required) |
| `OAUTH_ACCESS_EXPIRY` | `15m` | Access token TTL |
| `OAUTH_REFRESH_EXPIRY` | `168h` | Refresh token TTL |
| **Google OAuth** | | |
| `GOOGLE_CLIENT_ID` | — | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | — | Google OAuth client secret |
| `GOOGLE_REDIRECT_URL` | `http://localhost:8081/api/v1/auth/google/callback` | OAuth redirect |
| **AI** | | |
| `AI_PROVIDER` | `gemini` | AI provider |
| `AI_API_KEY` | — | Gemini API key (required for reports) |
| `AI_MODEL` | `gemini-2.0-flash` | Gemini model |
| `AI_SEARCH_GROUNDING` | `true` | Enable Gemini web search for reports |
| **Scraper** | | |
| `SCRAPE_RESULT_LIMIT` | `10` | Google results per query |
| `PROXY_URL` | — | Residential proxy URL (`http://user:pass@host:port`) |
| `SERPER_API_KEY` | — | Serper.dev fallback API key |

---

## Project Structure

```
seo-rank-guardian/
├── main.go                  # Application entry point
├── Makefile                 # Build & dev commands
├── compose.yml              # Docker Compose (PostgreSQL, Redis, NATS)
├── .env.example             # Environment template
│
├── app/                     # Application bootstrap
├── cmd/                     # CLI commands (cobra)
├── config/                  # Configuration loading (envconfig)
│
├── internal/
│   ├── middleware/           # Auth, RBAC, logging middleware
│   └── server/v1/           # HTTP handlers & route registration
│       ├── auth.go          # Auth endpoints (register, login, OAuth)
│       ├── job.go           # Job CRUD & scan trigger
│       ├── history.go       # Runs, reports, trends, pair analysis
│       ├── user.go          # User profile endpoints
│       ├── dashboard.go     # Dashboard statistics
│       ├── run.go           # WebSocket handler
│       ├── routes.go        # Route registration
│       └── response.go      # Response types
│
├── model/                   # GORM database models
│   ├── job.go               # Job, JobKeyword
│   ├── job_run.go           # JobRun (scan execution)
│   ├── search_pair.go       # SearchPair, SearchResult
│   └── ...                  # RankDiff, Report, User, RunEvent
│
├── pkg/                     # Shared packages
│   ├── authz/               # Casbin RBAC policies
│   ├── cache/               # Redis cache layer
│   ├── db/                  # PostgreSQL connection (GORM)
│   ├── http/                # Fiber app setup & helpers
│   ├── logger/              # Zap structured logging
│   ├── nats/                # NATS JetStream client
│   ├── oauth2/              # JWT & Google OAuth
│   └── redis/               # Redis client wrapper
│
├── py-worker/               # Python async worker
│   ├── main.py              # Worker entry point
│   ├── worker.py            # NATS consumer & job dispatcher
│   ├── config.py            # Pydantic settings
│   ├── models.py            # SQLAlchemy models
│   ├── db.py                # Database session factory
│   ├── requirements.txt     # Python dependencies
│   ├── scraper/
│   │   ├── google_search.py # Patchright Google scraper with retry
│   │   └── parser.py        # Structural SERP parser (a > h3)
│   ├── handlers/
│   │   ├── scrape.py        # Google search & result storage
│   │   ├── detect.py        # Rank change detection
│   │   └── report.py        # AI report generation
│   └── ai/
│       ├── client.py        # AI client protocol & factory
│       ├── gemini_api.py    # Gemini API integration
│       └── prompt.py        # SEO analysis prompt & schema
│
├── docs/                    # Auto-generated Swagger docs
├── utils/                   # Helper utilities
└── logs/                    # Runtime log files
```

---

## Scraping Strategy

The Python worker uses a multi-layer approach to reliably scrape Google:

1. **Patchright Browser** (primary) — Patched Chromium that fixes the CDP `Runtime.Enable` leak and `navigator.webdriver` detection. Bypasses Google SearchGuard (deployed Jan 2025).

2. **Residential Proxy Rotation** — Each browser launch gets a fresh IP from the proxy pool. On CAPTCHA or 429, the worker retries with a new browser instance (up to 5 attempts with increasing backoff).

3. **Serper.dev API** (fallback) — If all browser attempts fail, falls back to the Serper.dev search API.

4. **Structural SERP Parsing** — Uses `a > h3` structural patterns instead of class names (`div.g`, `div.MjjYud`) for future-proof parsing as Google updates its markup.

---

## AI Reports

Reports are generated by Gemini with web search grounding enabled, producing:

- **Health Score** (0-100) based on ranking positions and trends
- **Critical Alerts** — significant drops, lost rankings, new competitor threats
- **Keyword Rankings** — current positions with change indicators
- **Content Gap Analysis** — what competitors cover that you don't
- **Prioritized Recommendations** — specific, actionable SEO improvements
- **Competitor Insights** — who's beating you and their trend direction

---

## Development

### Generate Swagger Docs

```bash
make swagger
# Docs available at http://localhost:8081/swagger/
```

### Run Tests

```bash
make test
```

### Build for Production

```bash
make build
# Binary at bin/srg
```

---

## Ports Reference

| Service | Port | Notes |
|---------|------|-------|
| Go API | `8081` | REST + WebSocket |
| PostgreSQL | `5433` | Non-default to avoid conflicts |
| Redis | `6380` | Non-default to avoid conflicts |
| NATS Client | `4223` | Non-default to avoid conflicts |
| NATS Monitor | `8223` | JetStream dashboard |

---

## Contributors

<table>
  <tr>
    <td align="center">
      <strong>Jeel Rupapara</strong><br />
      <a href="mailto:zeelrupapara@gmail.com">zeelrupapara@gmail.com</a><br />
      <sub>Creator & Lead Developer</sub>
    </td>
  </tr>
</table>

---

## License

This project is licensed under the MIT License.
