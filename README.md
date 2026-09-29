# 🚀 High-Performance Go + Keycloak Auth Microservice

A production-ready, lightweight, and plug-and-play **Authentication & Authorization** microservice built with **Go (Gin)** and backed by an embedded **Keycloak** Identity Provider packaged inside a single unified container.

Designed for microservices architectures that need a **fast, trusted, and self-contained** auth gateway capable of handling **10,000+ RPS** for downstream token verification with zero external dependencies.

---

## ✨ Features

- **⚡ Blazing Fast**: JWT signature verification runs locally in CPU memory via OIDC JWKS (`< 0.5ms` latency, verified at **10,000+ RPS** with **0.00% errors**).
- **📦 All-in-One Container**: Keycloak and the Go Auth Microservice are bundled into a single image (`auth-service:latest`).
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
┌────────────────────────────────────────────────────────────────────────┐
│                   auth-service:latest (Single Container)               │
│                                                                        │
│   Client Traffic                                                       │
│         │                                                              │
│         ▼                                                              │
│   ┌──────────────────────────────────────────────────────────────┐     │
│   │               Go Auth Gateway (Port :3000)                   │     │
│   └───────────────┬───────────────────────────────┬──────────────┘     │
│                   │                               │                    │
│   [ 99% Traffic ] │                               │ [ 1% Traffic ]     │
│   Token Verify    │                               │ Login / Register   │
│                   ▼                               ▼                    │
│   ┌───────────────────────────────┐   ┌───────────────────────────┐    │
│   │    In-Memory CPU JWKS Cache   │   │  Keycloak Identity Server │    │
│   │  (RS256 Public Key Verify)    │   │       (Port :8080)        │    │
│   └───────────────────────────────┘   └─────────────┬─────────────┘    │
└─────────────────────────────────────────────────────┼──────────────────┘
                                                      │ (JDBC Connection)
                                                      ▼
                                        ┌───────────────────────────────┐
                                        │    Your Database              │
                                        │ (Postgres / Supabase / MySQL) │
                                        └───────────────────────────────┘
```

---

## 🚀 Quick Start

### ⚡ Option 1: 1-Click Smart Launcher (Easiest)
Simply run the setup script:
```bash
./start.sh
```
* **If `.env` or `docker-compose.yml` is already configured**: It starts the service immediately with **zero questions**.
* **If it's a fresh clone**: It opens an interactive wizard asking whether you want an automatic local PostgreSQL container or to connect to your existing database.

---

### 🛠️ Option 2: Docker Compose (Recommended)

1. **Copy the environment template**:
   ```bash
   cp .env.example .env
   ```

2. **Configure your database in `.env`**:
   ```env
   DB_VENDOR=postgres
   DB_URL=jdbc:postgresql://host.docker.internal:5432/my_database
   DB_USERNAME=postgres
   DB_PASSWORD=your_secure_password
   DB_SCHEMA=keycloak
   ```

3. **Launch the container**:
   ```bash
   docker compose up -d
   ```

---

### 🐳 Option 3: Docker Run (Direct CLI)

```bash
docker run -d \
  --name auth-service \
  -p 3000:3000 \
  -p 8080:8080 \
  -e DB_URL=jdbc:postgresql://host.docker.internal:5432/my_database \
  -e DB_USERNAME=postgres \
  -e DB_PASSWORD=your_secure_password \
  --add-host host.docker.internal:host-gateway \
  auth-service:latest
```

---

### 🖥️ Option 4: Docker Desktop GUI

If you are running the image via the **Docker Desktop application**:
1. Go to **Images** ➔ Find **`auth-service:latest`** ➔ Click **Run**.
2. Expand **Optional settings**:
   * **Ports**:
     * Map `:3000/tcp` to Host port `3000`
     * Map `:8080/tcp` to Host port `8080`
   * **Environment variables**:
     * `DB_URL` = `jdbc:postgresql://host.docker.internal:5432/your_database`
     * `DB_USERNAME` = `your_username`
     * `DB_PASSWORD` = `your_password`
3. Click the blue **Run** button.

*(Note: If you forget to fill in the database variables, the container will stop and display an actionable checklist in the **Logs** tab).*

---

### Check Status
```bash
curl http://localhost:3000/health
```
**Response:**
```json
{
  "environment": "production",
  "service": "auth",
  "status": "ok"
}
```

> [!NOTE]
> **First-Time Cold Start**: On a brand-new database, Keycloak automatically provisions all tables and imports the `auth` realm on first boot (~20–40s). The API Gateway on port `:3000` will be live as soon as migrations complete. Subsequent starts are instantaneous (< 5s).

---

## 🔌 Integration Examples ([`integrations/`](./integrations))

Ready-to-use Docker Compose integration examples are provided in the [`integrations/`](./integrations) folder (see [integrations documentation](./integrations/README.md)). These show external developers how to run the published `auth-service:latest` image directly alongside a database and the **Adminer** web database manager (`http://localhost:8081`):

### 🐘 PostgreSQL 18 Integration
Runs an isolated PostgreSQL 18 stack with automatic `keycloak` schema setup:
```bash
docker compose -f integrations/docker-compose.postgres.yml up -d
```
* **Auth API Gateway**: `http://localhost:3000`
* **Keycloak Admin**: `http://localhost:8080` (admin / admin)
* **Adminer Web GUI**: `http://localhost:8081`
  * **System**: `PostgreSQL` | **Server**: `db` | **User**: `keycloak` | **Pass**: `123456` | **DB**: `keycloak`

### 🐬 MySQL Integration
Runs an isolated MySQL stack (`mysql:latest`) tuned for fast initial migration:
```bash
docker compose -f integrations/docker-compose.mysql.yml up -d
```
* **Auth API Gateway**: `http://localhost:3000`
* **Keycloak Admin**: `http://localhost:8080` (admin / admin)
* **Adminer Web GUI**: `http://localhost:8081`
  * **System**: `MySQL` | **Server**: `db` | **User**: `keycloak` | **Pass**: `123456` | **DB**: `keycloak`

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

## 📊 Benchmark Verification (10,000 RPS Target)

The token verification architecture was benchmarked under extreme concurrency using **Grafana `k6`** and **`hey`**. Full logs and methodology are saved in [`benchmark/RESULTS.md`](./benchmark/RESULTS.md) and [`benchmark/TUNING.md`](./benchmark/TUNING.md).

| Metric | Measured Value | SLA Target | Status |
| :--- | :---: | :---: | :---: |
| **Total Requests Handled** | **212,822 requests** | N/A | ✅ |
| **HTTP Error Rate** | **0.00%** (0 errors) | < 1.0% | 🏆 **Zero Errors** |
| **Peak Throughput / Arrival Rate** | **10,000.00 req/sec** | 10,000 RPS | 🎯 **Target Achieved** |
| **Fastest Cryptographic Verification** | **533 µs** (0.53 ms) | < 1.0 ms | ⚡ **Sub-Millisecond** |
| **Integrity Checks** | **100.00% passed** | 100.00% | 🏆 **100% Cryptographically Valid** |

---

## ⚙️ Environment Configuration Reference

| Variable | Description | Default / Example |
| :--- | :--- | :--- |
| `DB_VENDOR` | Database engine (`postgres`, `mysql`) | `postgres` |
| `DB_URL` | JDBC database connection string | `jdbc:postgresql://host.docker.internal:5432/mydb` |
| `DB_USERNAME` | Database username | `postgres` |
| `DB_PASSWORD` | Database password | *(required)* |
| `DB_SCHEMA` | Isolated database schema for auth tables | `keycloak` |
| `PORT` | Go microservice listen port | `:3000` |
| `ENV` | Environment mode (`production` or `development`) | `production` |
| `GIN_MODE` | Gin framework mode (`release` or `debug`) | `release` |
| `ALLOWED_ORIGINS` | Comma-separated allowed CORS origins | `http://localhost:3000,http://localhost:5173` |

---

## 🗂️ Project Structure

```
.
├── Dockerfile                # Multi-stage production All-in-One image (Go + Keycloak)
├── entrypoint.sh             # Process supervisor (validates DB, boots Keycloak, then starts Go)
├── start.sh                  # Smart 1-click launcher & interactive setup wizard
├── .dockerignore             # Excludes build context, secrets, and temp files
├── .gitignore                # Excludes secrets (.env), keys, binaries, logs, and data
├── .env.example              # Documented environment template with DB presets
├── docker-compose.yml        # All-in-One stack definition with optional with-db profile
├── realm-export.json         # Hardened Keycloak realm with auto-provisioned client
├── auth.json                 # Postman API Collection (v2.1.0) with automated token tests
├── benchmark
│   ├── k6_benchmark.js       # Declarative k6 10,000 RPS benchmark scenario
│   ├── run.sh                # Automated k6 runner script
│   ├── run_hey.sh            # Automated hey runner script
│   ├── RESULTS.md            # Verified benchmark metrics, percentiles, and raw logs
│   └── TUNING.md             # Production Linux OS kernel socket & cloud tuning guide
├── integrations
│   ├── README.md                 # Dedicated guide for standalone database integration templates
│   ├── docker-compose.postgres.yml # Integration example: PostgreSQL 18 + Adminer UI
│   └── docker-compose.mysql.yml    # Integration example: MySQL Latest + Adminer UI
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
