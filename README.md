# 🚀 High-Performance Go + Keycloak Auth Service

A production-ready, lightweight, and plug-and-play **Authentication & Authorization** microservice built with **Go (Gin)** and backed by an isolated **Keycloak** Identity Provider.

Designed for microservices architectures that need a **fast, trusted, and self-contained** auth gateway capable of handling **10,000+ RPS** for downstream token verification with zero database dependencies.

---

## ✨ Features

- **⚡ Blazing Fast**: JWT signature verification runs locally in CPU memory via OIDC JWKS (`< 0.5ms` latency, easily exceeds 10,000 RPS).
- **🔒 Zero Database Maintenance**: Keycloak runs in an isolated, persistent Docker container with embedded storage. No PostgreSQL, MySQL, or migrations required.
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
                 └───────────────────────┘       └───────────────────────┘
```

---

## 🚀 Quick Start (One Command, Zero Installs)

### Single-Command Launch (Production & Dev Ready)
Launch both Keycloak and the Go Auth microservice simultaneously with automatic realm provisioning:

```bash
docker compose up -d --build
```

That's it! 
- **Keycloak** will start on port `8080` and auto-import the hardened `auth` realm.
- **Go Auth API** will start on port `3000`, automatically wait for Keycloak to finish bootstrapping, and become healthy.

Check status:
```bash
docker compose ps
curl http://localhost:3000/health
```

---

### Local Development (Optional)
If you prefer running the Go binary locally on your host with hot reloading (`air` or `go run`):

```bash
# 1. Start Keycloak only
docker compose up -d keycloak

# 2. Run the Go Auth service locally
go run main.go
# or with live reload:
air
```

---

## 📡 API Reference

All endpoints are prefixed with `/api/auth`.

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

---

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
*(Also automatically sets `access_token` and `refresh_token` as HttpOnly cookies)*
```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_in": 300
}
```

---

#### 3. Token Verification for Downstream Microservices (`GET /api/auth/verify`)
Downstream microservices (Orders, Billing, Products) can authenticate callers by forwarding the `Bearer <token>`:

```bash
curl -X GET http://localhost:3000/api/auth/verify \
  -H "Authorization: Bearer <access_token>"
```
**Response (`200 OK`):**
```json
{
  "valid": true,
  "user_id": "c1f7a0b2-...",
  "username": "john_doe",
  "email": "john@example.com",
  "name": "John Doe",
  "first_name": "John",
  "last_name": "Doe",
  "roles": ["user", "default-roles-auth"]
}
```

---

## ⚙️ Environment Configuration (`.env`)

```ini
# HTTP Server Port
PORT=:3000

# Keycloak OIDC Provider Settings
KEYCLOAK_ISSUER_URL=http://localhost:8080/realms/auth
KEYCLOAK_BASE_URL=http://localhost:8080
KEYCLOAK_REALM=auth
KEYCLOAK_CLIENT_ID=auth-backend
KEYCLOAK_CLIENT_SECRET=Dr4G0j6vvfWMdvrKdHAYBrDNdmopUgkEF0YRZuWrWRZOv7RWnJqJ26MseNiPrXjDQDo0ZCiOGbdcHvu2cWNuk8

# Allowed CORS Origins (comma-separated)
ALLOWED_ORIGINS=http://localhost,http://localhost:3000,http://localhost:5173,http://localhost:8080
```

---

## 🗂️ Project Structure

```
.
├── Dockerfile                # Multi-stage minimal production Docker image
├── .dockerignore             # Excludes transient files from container build context
├── docker-compose.yml        # Orchestrates Keycloak + Go Auth Service stack
├── realm-export.json         # Hardened Keycloak realm with auto-provisioned client
├── auth.json                 # Postman API Collection (v2.1.0) with automated token tests
├── benchmark
│   ├── k6_benchmark.js       # Declarative k6 10,000 RPS benchmark scenario
│   ├── run.sh                # Automated benchmark execution script
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
