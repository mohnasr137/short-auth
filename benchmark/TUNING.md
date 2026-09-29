# 🚀 Production Tuning Guide for 10,000+ RPS

This guide outlines the system architecture, OS kernel settings, and deployment recommendations required to reliably serve **10,000 to 50,000+ Requests Per Second** in production environments.

---

## 1. Workload Reality in Production

In a distributed microservice system:
- **`POST /api/auth/login`**: Intentionally CPU-intensive (Keycloak cryptographic password hashing protects against brute force attacks; ~50–150 logins/sec per CPU core). Users only log in once every few hours.
- **`GET /api/auth/verify` & Middleware**: High throughput (10,000+ RPS). Every single downstream API call (e-commerce orders, cart, catalog, checkout) requires instantaneous token signature validation.

Our Go Auth service performs **in-memory cryptographic RSA public-key verification (`RS256`)** against cached Keycloak JWKS keys without making any external HTTP or database calls. This in-memory operation takes **micro-seconds** (~0.05ms to 0.2ms), allowing multi-core instances to handle tens of thousands of requests per second.

---

## 2. Linux OS & Network Kernel Tuning (`/etc/sysctl.conf`)

Default Linux kernel settings are designed for general-purpose desktop/server workloads and will exhaust ephemeral sockets and connection backlogs within seconds at 10,000 RPS.

Apply these settings on both the **Auth Service hosts** and the **Load Generator hosts**:

```ini
# /etc/sysctl.conf

# Maximum connection backlog queue (default is usually 128 or 4096)
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 65535

# Expand the range of ephemeral ports available for outgoing/incoming connections
net.ipv4.ip_local_port_range = 1024 65535

# Enable fast reuse of TIME_WAIT sockets for outgoing connections
net.ipv4.tcp_tw_reuse = 1

# Reduce FIN timeout to free up dead TCP connections rapidly
net.ipv4.tcp_fin_timeout = 15

# Increase socket buffer sizes
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
```

Reload settings:
```bash
sudo sysctl -p
```

### File Descriptor Limits (`/etc/security/limits.conf`)
```ini
* soft nofile 65535
* hard nofile 65535
```
Apply in current shell:
```bash
ulimit -n 65535
```

---

## 3. Production Deployment Architecture

```
                       ┌────────────────────────────┐
                       │     Cloud Load Balancer    │
                       │   (AWS ALB / NGINX / Cloud)│
                       └─────────────┬──────────────┘
                                     │
                 ┌───────────────────┼───────────────────┐
                 │ (Round-Robin)     │                   │
                 ▼                   ▼                   ▼
        ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
        │  Go Auth Pod 1  │ │  Go Auth Pod 2  │ │  Go Auth Pod 3  │
        │   2 vCPU, 2GB   │ │   2 vCPU, 2GB   │ │   2 vCPU, 2GB   │
        └────────┬────────┘ └────────┬────────┘ └────────┬────────┘
                 │                   │                   │
                 └───────────────────┼───────────────────┘
                                     │ (Initial JWKS public key fetch only)
                                     ▼
                          ┌─────────────────────┐
                          │    Keycloak IAM     │
                          │   (Embedded DB)     │
                          └─────────────────────┘
```

### Production Sizing:
- **Go Auth Service**: 2 to 3 replicas with 2–4 vCPUs and 2 GB RAM easily handle 10,000–30,000 RPS with flat memory usage (~30MB per container).
- **Keycloak**: 1 to 2 replicas for user authentication and JWKS public key distribution. Keycloak CPU remains near 0% during downstream token verification because tokens are validated in-memory in Go.
- **Keep-Alive**: Downstream services must enable HTTP persistent keep-alive connections to reuse TCP handshakes.
