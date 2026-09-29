# 🔌 Auth Service Database Integration Examples

This folder provides standalone **Docker Compose** templates demonstrating how to deploy the pre-built `auth-service:latest` image alongside popular relational databases (**PostgreSQL 18** and **MySQL Latest**), complete with an **Adminer** web database management interface.

These compose files are designed as turnkey reference architectures for developers integrating the Auth Microservice with their own infrastructure.

---

## 📁 Available Stacks

| Stack | Compose File | Database Engine | Adminer Port | Gateway Port |
| :--- | :--- | :--- | :--- | :--- |
| **PostgreSQL 18** | [`docker-compose.postgres.yml`](./docker-compose.postgres.yml) | PostgreSQL 18 | `http://localhost:8081` | `http://localhost:3000` |
| **MySQL Latest** | [`docker-compose.mysql.yml`](./docker-compose.mysql.yml) | MySQL Latest | `http://localhost:8081` | `http://localhost:3000` |

---

## 🐘 Option 1: PostgreSQL 18 Integration

Runs an isolated PostgreSQL 18 container with an automatic `keycloak` database and schema.

### Quick Start
```bash
docker compose -f integrations/docker-compose.postgres.yml up -d
```

### Endpoints & Access
* **Auth Microservice API Gateway**: `http://localhost:3000`
  * Health Probe: `curl http://localhost:3000/health`
* **Keycloak Admin Console**: `http://localhost:8080` (admin / admin)
* **Adminer Web GUI**: `http://localhost:8081`
  * **System**: `PostgreSQL`
  * **Server**: `db`
  * **Username**: `keycloak`
  * **Password**: `123456`
  * **Database**: `keycloak`

### Stop & Tear Down
```bash
docker compose -f integrations/docker-compose.postgres.yml down -v
```

---

## 🐬 Option 2: MySQL Latest Integration

Runs an isolated MySQL Latest container tuned for fast initial migration.

### Performance Tuning Included
During initial cold starts, Keycloak applies extensive Liquibase schema migrations. To eliminate per-transaction disk sync bottlenecks without compromising runtime stability, this template includes:
* `--innodb-flush-log-at-trx-commit=2`: Caches log writes to memory before flushing, dramatically speeding up table creation.
* `--disable-log-bin`: Disables binary logging for fast local migration.
* `--innodb-buffer-pool-size=256M`: Allocates sufficient buffer pool for table indexes.
* `--character-set-server=utf8mb4` & `--collation-server=utf8mb4_unicode_ci`: Full Unicode support.

### Quick Start
```bash
docker compose -f integrations/docker-compose.mysql.yml up -d
```

### Endpoints & Access
* **Auth Microservice API Gateway**: `http://localhost:3000`
  * Health Probe: `curl http://localhost:3000/health`
* **Keycloak Admin Console**: `http://localhost:8080` (admin / admin)
* **Adminer Web GUI**: `http://localhost:8081`
  * **System**: `MySQL`
  * **Server**: `db`
  * **Username**: `keycloak`
  * **Password**: `123456`
  * **Database**: `keycloak`

### Stop & Tear Down
```bash
docker compose -f integrations/docker-compose.mysql.yml down -v
```

---

## 🔍 Healthcheck & Startup Lifecycle

Both integration templates use native dependency ordering:
1. **Database Container (`db`)**: Boots first. It defines a native healthcheck (`pg_isready` for PostgreSQL, `mysqladmin ping` for MySQL).
2. **Auth Service Container (`auth-service`)**: Waits for `db` to reach `healthy` state before starting.
   - Keycloak initializes, runs schema migrations, and registers the pre-configured `auth` realm.
   - The embedded entrypoint supervisor displays a progress heartbeat every 10 seconds during migrations.
   - Once Keycloak port `8080` responds, the Go API Gateway launches on port `3000`.
   - The container healthcheck uses bash TCP probing (`/dev/tcp/127.0.0.1/3000`), ensuring compatibility with Keycloak's minimal UBI base image.
3. **Adminer UI (`adminer`)**: Starts alongside the database for visual table inspection.

---

## 🧪 Testing the Integration

Once the container is reported as `healthy`:

```bash
# 1. Verify health
curl -s http://localhost:3000/health

# 2. Register a new user
curl -s -X POST http://localhost:3000/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"dev_user","email":"dev@example.com","password":"Password123!","first_name":"Dev","last_name":"User"}'

# 3. Log in to obtain tokens
curl -s -X POST http://localhost:3000/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"dev_user","password":"Password123!"}'
```

---

## 📊 Verified Benchmark Performance

Both integration stacks have been verified under extreme concurrency using `k6` (up to 10,000 RPS arrival rate) and `hey` (50,000 requests, 100 concurrent workers):

| Metric | PostgreSQL 18 Stack | MySQL Latest Stack |
| :--- | :---: | :---: |
| **Initial Cold-Start Migration** | **21.6 seconds** | **~115 seconds** |
| **Warm-Restart Startup** | **< 10 seconds** | **< 10 seconds** |
| **Sustained Throughput (`hey`)** | **4,057 req/sec** | **4,504 req/sec** |
| **Fastest Verification Latency** | **489 µs** (0.49 ms) | **508 µs** (0.50 ms) |
| **HTTP Error Rate** | **0.00%** (0 errors) | **0.00%** (0 errors) |
| **Cryptographic Checks** | **100.00% valid** | **100.00% valid** |

