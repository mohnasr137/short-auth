# 🚀 High-Performance Go + Keycloak Auth Service

A production-ready, lightweight, and plug-and-play **Authentication & Authorization** microservice built with **Go (Gin)** and backed by an isolated **Keycloak** Identity Provider.

Designed for microservices architectures that need a **fast, trusted, and self-contained** auth gateway capable of handling **10,000+ RPS** for downstream token verification with zero external dependencies.

---

## ✨ Features

- **⚡ Blazing Fast**: JWT signature verification runs locally in CPU memory via OIDC JWKS (`< 0.5ms` latency, easily exceeds 10,000 RPS).
- **🗄️ Your Database, Zero Lock-In**: Connects directly to your existing database (PostgreSQL, MySQL, Supabase, Neon, AWS RDS). All tables live cleanly inside an isolated `keycloak` schema.
- **🛡️ Bulletproof Persistence**: Zero Docker storage volumes (`docker compose down -v` will **never** wipe your users or passwords).
- **🛡️ 64-Shard Striped Rate Limiting**: In-memory token-bucket rate limiter striped across 64 independent mutex shards to eliminate lock contention under extreme concurrency.
- **🍪 OWASP-Compliant Cookies**: Automatic `HttpOnly`, `SameSite=Lax`, and `Secure` cookie management for SPAs, mobile apps, and browser clients.
- **🌐 Turnkey Social Login**: Pre-configured Google OAuth2 federation with anti-CSRF state cookies and open-redirect protection.
- **🔁 Token Rotation**: Refresh token revocation/rotation enabled by default to prevent session hijacking.
- **🛡️ Brute-Force Shield**: Dual protection with Go IP-based rate limiting + Keycloak account lockout.

---

## 🏗️ Architecture

```
                  ┌────────────────────────────────────────────────────────┐
                  │                    Client Traffic                      │
                  └──────────────────────────┬─────────────────────────────┘
                                             │
                                             ▼
                 ┌───────────────────────────────────────────────────────┐
                 │                   Go Auth Gateway                     │
                 │                    (Port :3000)                       │
                 └───────────┬───────────────────────────────┬───────────┘
                             │                               │
        [ 99% of Traffic ]   │                               │   [ 1% of Traffic ]
        Token Verification   │                               │   Interactive Login / Register
                             ▼                               ▼
                 ┌───────────────────────┐       ┌───────────────────────┐
                 │  In-Memory CPU Cache  │       │       Keycloak        │
                 │  (JWKS Public Keys)   │       │     (Port :8080)      │
                 └───────────────────────┘       └───────────┬───────────┘
                                                             │
                                                             ▼ (JDBC)
                                                 ┌───────────────────────┐
                                                 │   Your Database       │
                                                 │ (Postgres/MySQL/Cloud)│
                                                 └───────────────────────┘
```

---

## 🚀 Quick Start

### ⚡ Option 1: 1-Click Smart Launcher (Easiest)
Simply run the setup script:
```bash
./start.sh
```
* **If `.env` or `docker-compose.yml` is already configured**: It starts the service immediately with **zero questions**.
* **If it's a fresh clone**: It opens a guided wizard asking whether you want an automatic local PostgreSQL container or to connect to your existing database.

---

### 🛠️ Option 2: Manual Setup (3 Steps)
```bash
cp .env.example .env
```

### Step 2: Configure Your Database Connection
Open `.env` and fill in your existing database details:

```env
# Database vendor: postgres (recommended) or mysql
DB_VENDOR=postgres

# Database URL:
# For local DB running on your machine:
DB_URL=jdbc:postgresql://host.docker.internal:5432/my_database
# For cloud DB (Supabase / AWS RDS / Neon):
# DB_URL=jdbc:postgresql://db.xxxx.supabase.co:5432/postgres?sslmode=require

DB_USERNAME=postgres
DB_PASSWORD=your_password
DB_SCHEMA=keycloak
```

### Step 3: Launch with One Command

#### Option A: Docker Run (Single Container)
```bash
docker run -d \
  --name auth-service \
  -p 3000:3000 \
  -p 8080:8080 \
  --env-file .env \
  --add-host host.docker.internal:host-gateway \
  auth-service:latest
```

#### Option B: Docker Compose
```bash
docker compose up -d
```

The container starts Keycloak, creates tables in your database schema, auto-imports the `auth` realm, and boots the Go Auth microservice gateway!

Check status:
```bash
docker compose ps
curl http://localhost:3000/health
```

---

## 📡 API Reference

All endpoints are prefixed with `/api/auth`. You can also import [`auth.json`](./auth.json) directly into **Postman** (v2.1.0 collection with automated token extractors).

| Method | Endpoint | Description | Rate Limit |
| :--- | :--- | :--- | :--- |
| `GET` | `/health` | Container & orchestrator health probe | Unlimited |
| `POST` | `/api/auth/register` | Register a new user | 10 req/min |
| `POST` | `/api/auth/login` | Authenticate user & issue tokens | 15 req/min |
| `POST` | `/api/auth/refresh` | Exchange refresh token for fresh tokens | 30 req/min |
| `POST` | `/api/auth/logout` | Revoke session & clear cookies | Unlimited |
| `GET` | `/api/auth/verify` | Validate token for downstream services | Unlimited |
| `GET` | `/api/auth/me` | Retrieve authenticated user profile | Unlimited |
| `PUT` | `/api/auth/change-password` | Update user password | 10 req/min |
| `POST` | `/api/auth/forgot-password` | Trigger password reset email | 5 req/min |
| `GET` | `/api/auth/google` | Initiate Google OAuth2 login | Unlimited |
| `GET/POST`| `/api/auth/callback` | OAuth2 redirect & token exchange | 20 req/min |

---

### Request & Response Examples

#### 1. User Registration (`POST /api/auth/register`)
```bash
curl -X POST http://localhost:3000/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "john_doe",
    "email": "john@example.com",
    "password": "Password123!",
    "first_name": "John",
    "last_name": "Doe"
  }'
```
**Response (`201 Created`):**
```json
{
  "message": "User registered successfully",
  "user_id": "c1f7a0b2-..."
}
```

#### 2. User Login (`POST /api/auth/login`)
```bash
curl -X POST http://localhost:3000/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "john_doe",
    "password": "Password123!"
  }'
```
**Response (`200 OK`):**
```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_in": 300,
  "refresh_expires_in": 1800,
  "token_type": "Bearer"
}
```

#### 3. Token Verification for Microservices (`GET /api/auth/verify`)
Downstream microservices can verify tokens at 10,000+ RPS:
```bash
curl -X GET http://localhost:3000/api/auth/verify \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIs..."
```
**Response (`200 OK`):**
```json
{
  "valid": true,
  "user_id": "c1f7a0b2-...",
  "username": "john_doe",
  "roles": ["user"]
}
```

---

## ⚙️ Environment Configuration Reference

| Variable | Description | Default / Example |
| :--- | :--- | :--- |
| `DB_VENDOR` | Database engine (`postgres`, `mysql`) | `postgres` |
| `DB_URL` | JDBC database connection string | `jdbc:postgresql://host.docker.internal:5432/mydb` |
| `DB_USERNAME` | Database username | `postgres` |
| `DB_PASSWORD` | Database password | `secret` |
| `DB_SCHEMA` | Isolated database schema for auth tables | `keycloak` |
| `PORT` | Go microservice listen port | `:3000` |
| `KEYCLOAK_BASE_URL` | Internal Docker URL to reach Keycloak | `http://keycloak:8080` |
| `KEYCLOAK_PUBLIC_URL` | Public-facing Keycloak URL for browser redirects | `http://localhost:8080` |
| `ALLOWED_ORIGINS` | Comma-separated allowed CORS origins | `http://localhost:3000,http://localhost:5173` |

---

## 🗂️ Project Structure

```
.
├── Dockerfile                # Multi-stage minimal production Docker image
├── .dockerignore             # Excludes build context & secrets
├── .gitignore                # Excludes secrets, binaries, logs, and data
├── .env.example              # Documented environment template with DB presets
├── docker-compose.yml        # Keycloak + Go Auth Service stack with external DB wiring
├── realm-export.json         # Hardened Keycloak realm with auto-provisioned client
├── auth.json                 # Postman API Collection (v2.1.0) with automated token tests
├── benchmark
│   ├── k6_benchmark.js       # Declarative k6 10,000 RPS benchmark scenario
│   ├── run.sh                # Automated k6 runner script
│   ├── run_hey.sh            # Automated hey runner script
│   ├── RESULTS.md            # Verified benchmark metrics and telemetry
│   └── TUNING.md             # Production Linux OS kernel socket & cloud tuning guide
├── config
│   └── config.go             # Fail-fast configuration loader
├── controllers
│   └── authController.go     # HTTP handlers (login, register, OAuth, verify)
├── internal
│   ├── depend.go             # Application dependency container
│   ├── errors.go             # Standardized error reporting with unique Error IDs
│   └── validator.go          # Password strength & payload validator
├── middlewares
│   ├── authMiddleware.go     # Token verification & role RBAC enforcement
│   ├── cors.go               # CORS headers & preflight handler
│   └── ratelimit.go          # 64-shard striped high-throughput rate limiter
├── routes
│   └── authRoute.go          # Auth route definitions
├── services
│   └── authService.go        # Keycloak REST/OIDC integration client
├── go.mod                    # Lean module dependencies
├── go.sum
└── main.go                   # HTTP server lifecycle, health checks & graceful shutdown
```
