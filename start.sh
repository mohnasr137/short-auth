#!/bin/bash
set -e

ENV_FILE=".env"

echo "================================================================================"
echo "⚡ Checking Docker Compose & Environment Configuration..."
echo "================================================================================"

# ==============================================================================
# 1. Zero-Prompt Check: Does Docker Compose already have all required data?
# ==============================================================================
# If the developer already provided environment variables in .env, in their shell,
# or directly in docker-compose.yml, 'docker compose config -q' succeeds with code 0.
if docker compose config -q 2>/dev/null; then
  echo "✅ Complete configuration detected (from docker-compose / .env / environment)!"
  echo "🚀 Starting Auth Microservice immediately without asking any questions..."
  echo "================================================================================"
  docker compose up -d
  echo ""
  echo "🎉 Containers started successfully!"
  echo "🔗 Auth Microservice API Gateway: http://localhost:3000"
  echo "🔑 Keycloak Admin Console:        http://localhost:8080"
  exit 0
fi

# ==============================================================================
# 2. Interactive Setup Wizard (Only triggers if required data is missing)
# ==============================================================================
echo "⚠️ Required database configuration is missing."
echo ""
echo "================================================================================"
echo "🧙 Interactive Setup Wizard"
echo "================================================================================"
echo "How would you like to set up your database?"
echo "  1) Automatic Local Database (Spins up a lightweight PostgreSQL container)"
echo "  2) Connect to My Existing Database (PostgreSQL, Supabase, Neon, AWS RDS)"
echo ""
read -r -p "Enter choice [1 or 2] (Default: 1): " DB_CHOICE
DB_CHOICE=${DB_CHOICE:-1}

if [ "$DB_CHOICE" = "1" ]; then
  echo ""
  echo "📦 Configuring automatic local PostgreSQL container..."
  
  cat <<EOF > "$ENV_FILE"
# ==============================================================================
# 🗄️ DATABASE CONFIGURATION (Local Container)
# ==============================================================================
DB_VENDOR=postgres
DB_URL=jdbc:postgresql://host.docker.internal:5432/postgres
DB_USERNAME=postgres
DB_PASSWORD=postgres
DB_SCHEMA=keycloak

# ==============================================================================
# 🔐 AUTH MICROSERVICE SETTINGS
# ==============================================================================
PORT=:3000
ENV=production
GIN_MODE=release
ALLOWED_ORIGINS=http://localhost,http://localhost:3000,http://localhost:5173,http://localhost:8080
EOF

  echo "✅ Created .env with local database settings!"
  echo "🚀 Starting PostgreSQL + Auth Service..."
  docker compose --profile with-db up -d

else
  echo ""
  echo "🌐 Please enter your database connection details:"
  echo ""
  read -r -p "👉 Database Host [host.docker.internal]: " INPUT_HOST
  INPUT_HOST=${INPUT_HOST:-host.docker.internal}

  read -r -p "👉 Database Port [5432]: " INPUT_PORT
  INPUT_PORT=${INPUT_PORT:-5432}

  read -r -p "👉 Database Name [postgres]: " INPUT_NAME
  INPUT_NAME=${INPUT_NAME:-postgres}

  read -r -p "👉 Database Username [postgres]: " INPUT_USER
  INPUT_USER=${INPUT_USER:-postgres}

  read -r -s -p "👉 Database Password: " INPUT_PASS
  echo ""

  cat <<EOF > "$ENV_FILE"
# ==============================================================================
# 🗄️ DATABASE CONFIGURATION
# ==============================================================================
DB_VENDOR=postgres
DB_URL=jdbc:postgresql://${INPUT_HOST}:${INPUT_PORT}/${INPUT_NAME}
DB_USERNAME=${INPUT_USER}
DB_PASSWORD=${INPUT_PASS}
DB_SCHEMA=keycloak

# ==============================================================================
# 🔐 AUTH MICROSERVICE SETTINGS
# ==============================================================================
PORT=:3000
ENV=production
GIN_MODE=release
ALLOWED_ORIGINS=http://localhost,http://localhost:3000,http://localhost:5173,http://localhost:8080
EOF

  echo "✅ Created .env with your database connection!"
  echo "🚀 Starting Auth Service..."
  docker compose up -d
fi

echo ""
echo "================================================================================"
echo "🎉 Setup complete! Checking service health..."
sleep 3
curl -s http://localhost:3000/health || echo "Service starting up..."
echo ""
echo "🔗 Auth Microservice API Gateway: http://localhost:3000"
echo "🔑 Keycloak Admin Console:        http://localhost:8080"
echo "================================================================================"

