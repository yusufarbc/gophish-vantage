#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════════
# deploy.sh — Vantage One-Click Production Deployment
# Target OS: Ubuntu 20.04 / 22.04 / 24.04 (amd64)
# Run as: sudo bash deploy.sh
# ═══════════════════════════════════════════════════════════════════════════════

set -euo pipefail

# ── Colour palette ─────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BLUE='\033[0;34m'; BOLD='\033[1m'; NC='\033[0m'

log_info()    { echo -e "${BLUE}[INFO]${NC}    $*"; }
log_ok()      { echo -e "${GREEN}[OK]${NC}      $*"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC}    $*"; }
log_error()   { echo -e "${RED}[ERROR]${NC}   $*"; exit 1; }
log_step()    { echo -e "\n${CYAN}${BOLD}══ $* ══${NC}"; }

# ── Guard: must run as root ────────────────────────────────────────────────────
[[ "${EUID}" -eq 0 ]] || log_error "Please run as root: sudo bash deploy.sh"

# ── Configurable defaults ──────────────────────────────────────────────────────
DEPLOY_DIR="${DEPLOY_DIR:-/opt/vantage}"
COMPOSE_FILE="docker-compose.prod.yml"
ENV_FILE=".env"
VANTAGE_DATA_DIR="${DEPLOY_DIR}/vantage_data"
LOG_DIR="/var/log/vantage"

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 0 — Banner
# ═══════════════════════════════════════════════════════════════════════════════
clear
cat <<'EOF'

╔═══════════════════════════════════════════════════════════════════╗
║                                                                   ║
║   ██╗   ██╗ █████╗ ███╗   ██╗████████╗ █████╗  ██████╗ ███████╗  ║
║   ██║   ██║██╔══██╗████╗  ██║╚══██╔══╝██╔══██╗██╔════╝ ██╔════╝  ║
║   ██║   ██║███████║██╔██╗ ██║   ██║   ███████║██║  ███╗█████╗    ║
║   ╚██╗ ██╔╝██╔══██║██║╚██╗██║   ██║   ██╔══██║██║   ██║██╔══╝    ║
║    ╚████╔╝ ██║  ██║██║ ╚████║   ██║   ██║  ██║╚██████╔╝███████╗  ║
║     ╚═══╝  ╚═╝  ╚═╝╚═╝  ╚═══╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝ ╚══════╝  ║
║                                                                   ║
║   Production Deployment Script v2.0                              ║
║   Unified Offensive Security Platform                            ║
╚═══════════════════════════════════════════════════════════════════╝

EOF

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 1 — Pre-flight checks
# ═══════════════════════════════════════════════════════════════════════════════
log_step "STEP 1: Pre-flight Checks"

# OS check
if ! grep -qiE "ubuntu|debian" /etc/os-release 2>/dev/null; then
    log_warn "This script is tested on Ubuntu/Debian. Proceed at your own risk."
fi

# Architecture check
ARCH=$(uname -m)
[[ "$ARCH" == "x86_64" ]] || log_error "Only amd64 is supported. Got: ${ARCH}"
log_ok "Architecture: ${ARCH}"

# Required files
[[ -f "${COMPOSE_FILE}" ]]   || log_error "Missing ${COMPOSE_FILE}. Run this script from the Vantage repo root."
[[ -f "Caddyfile.prod" ]]    || log_error "Missing Caddyfile.prod."
[[ -f "${ENV_FILE}" ]]       || {
    log_warn ".env not found — copying from .env.example"
    [[ -f ".env.example" ]] || log_error "Neither .env nor .env.example found."
    cp .env.example .env
    log_warn "Please edit .env and re-run this script."
    exit 1
}

# Validate critical .env values are set
check_env_var() {
    local var="$1"
    # shellcheck disable=SC1091
    val=$(grep -E "^${var}=" .env | cut -d= -f2- | tr -d '"' | tr -d "'")
    [[ -n "$val" ]] || log_error "Required .env variable '${var}' is empty. Please set it."
}
check_env_var "DOMAIN"
check_env_var "ADMIN_HASH"
check_env_var "ACME_EMAIL"
log_ok ".env validation passed"

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 2 — Install Docker Engine + Compose Plugin
# ═══════════════════════════════════════════════════════════════════════════════
log_step "STEP 2: Docker Installation"

install_docker() {
    log_info "Updating apt and installing prerequisites..."
    apt-get update -qq
    apt-get install -y --no-install-recommends \
        ca-certificates curl gnupg lsb-release apt-transport-https

    log_info "Adding Docker's official GPG key..."
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
        | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg

    log_info "Adding Docker apt repository..."
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
        | tee /etc/apt/sources.list.d/docker.list > /dev/null

    log_info "Installing Docker Engine and Compose plugin..."
    apt-get update -qq
    apt-get install -y docker-ce docker-ce-cli containerd.io \
        docker-buildx-plugin docker-compose-plugin

    systemctl enable --now docker
    log_ok "Docker installed and started"
}

if command -v docker &>/dev/null; then
    DOCKER_VER=$(docker --version)
    log_ok "Docker already installed: ${DOCKER_VER}"
else
    install_docker
fi

# Verify compose plugin
if ! docker compose version &>/dev/null; then
    log_error "docker compose plugin not found. Check Docker installation."
fi
log_ok "Docker Compose plugin: $(docker compose version --short)"

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 3 — Host Directory Setup
# ═══════════════════════════════════════════════════════════════════════════════
log_step "STEP 3: Directory Setup"

mkdir -p "${VANTAGE_DATA_DIR}"
mkdir -p "${LOG_DIR}"
chmod 750 "${VANTAGE_DATA_DIR}"
log_ok "Data directory: ${VANTAGE_DATA_DIR}"
log_ok "Log directory:  ${LOG_DIR}"

# Ensure TUN kernel module is available
if ! lsmod | grep -q tun 2>/dev/null; then
    log_info "Loading TUN kernel module..."
    modprobe tun || log_warn "Could not load TUN module. Chisel tunneling may not work."
fi

# Persist TUN module across reboots
if ! grep -q "^tun$" /etc/modules 2>/dev/null; then
    echo "tun" >> /etc/modules
    log_ok "TUN module added to /etc/modules"
fi

# Ensure /dev/net/tun exists
if [[ ! -c /dev/net/tun ]]; then
    mkdir -p /dev/net
    mknod /dev/net/tun c 10 200
    chmod 0666 /dev/net/tun
    log_ok "Created /dev/net/tun"
else
    log_ok "/dev/net/tun already exists"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 4 — Kernel Hardening (sysctl)
# ═══════════════════════════════════════════════════════════════════════════════
log_step "STEP 4: Kernel Hardening"

cat > /etc/sysctl.d/99-vantage.conf <<'SYSCTL'
# Vantage — Production kernel hardening
net.ipv4.ip_forward = 1
net.ipv4.conf.all.rp_filter = 1
net.ipv4.conf.default.rp_filter = 1
net.ipv4.tcp_syncookies = 1
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.all.send_redirects = 0
net.ipv6.conf.all.accept_redirects = 0
# Increase ephemeral port range for high-volume scanning
net.ipv4.ip_local_port_range = 1024 65535
# Bigger socket buffers for Naabu/httpx mass-scanning
net.core.rmem_max = 134217728
net.core.wmem_max = 134217728
SYSCTL
sysctl -p /etc/sysctl.d/99-vantage.conf &>/dev/null
log_ok "Kernel parameters applied"

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 5 — Build & Deploy
# ═══════════════════════════════════════════════════════════════════════════════
log_step "STEP 5: Build & Deploy"

log_info "Pulling latest base images..."
docker compose -f "${COMPOSE_FILE}" pull --ignore-buildable 2>/dev/null || true

log_info "Building Vantage image (this may take 10–20 minutes on first run)..."
docker compose -f "${COMPOSE_FILE}" build --progress=plain

log_info "Starting all services in detached mode..."
docker compose -f "${COMPOSE_FILE}" up -d --remove-orphans

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 6 — Post-deployment Health Check
# ═══════════════════════════════════════════════════════════════════════════════
log_step "STEP 6: Health Check"

log_info "Waiting for vantage-core to become healthy (up to 120s)..."
TIMEOUT=120
ELAPSED=0
INTERVAL=5

until docker compose -f "${COMPOSE_FILE}" ps vantage-core \
        | grep -q "(healthy)"; do
    sleep "${INTERVAL}"
    ELAPSED=$((ELAPSED + INTERVAL))
    if [[ "${ELAPSED}" -ge "${TIMEOUT}" ]]; then
        log_warn "Service did not become healthy in ${TIMEOUT}s. Showing logs:"
        docker compose -f "${COMPOSE_FILE}" logs --tail=50 vantage-core
        log_error "Deployment health check failed."
    fi
    log_info "Waiting... (${ELAPSED}s / ${TIMEOUT}s)"
done

log_ok "vantage-core is healthy"

# Extract the initial admin password from container logs
INIT_PASSWORD=$(docker compose -f "${COMPOSE_FILE}" logs vantage-core 2>&1 \
    | grep -oP "(?<=the password )\S+" | tail -1 || true)

# Read domain from .env for display
# shellcheck disable=SC1091
DOMAIN_VAL=$(grep -E "^DOMAIN=" .env | cut -d= -f2- | tr -d '"' | tr -d "'")
ADMIN_SUB=$(grep -E "^ADMIN_SUBDOMAIN=" .env | cut -d= -f2- | tr -d '"' | tr -d "'" || echo "admin.${DOMAIN_VAL}")
ADMIN_USER_VAL=$(grep -E "^ADMIN_USER=" .env | cut -d= -f2- | tr -d '"' | tr -d "'" || echo "vantage")

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 7 — Success Banner
# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo -e "${GREEN}${BOLD}"
cat <<EOF
╔═══════════════════════════════════════════════════════════════════╗
║                                                                   ║
║   ✅  VANTAGE DEPLOYED SUCCESSFULLY                              ║
║                                                                   ║
╠═══════════════════════════════════════════════════════════════════╣
║                                                                   ║
║   🔐  Admin Dashboard                                            ║
║       URL:  https://${ADMIN_SUB}/
║       User: ${ADMIN_USER_VAL}
║       Pass: <your .env ADMIN_HASH password>
║                                                                   ║
║   🎣  Phishing Listener                                          ║
║       URL:  https://${DOMAIN_VAL}/
║                                                                   ║
║   🐳  Gophish Admin API                                         ║
EOF
if [[ -n "${INIT_PASSWORD}" ]]; then
echo -e "║       First-boot password: ${YELLOW}${INIT_PASSWORD}${GREEN}${BOLD}"
echo    "║       ⚠️  Change this immediately via the Settings page!"
fi
cat <<EOF
║                                                                   ║
║   📋  Useful Commands                                            ║
║   • Logs:    docker compose -f docker-compose.prod.yml logs -f   ║
║   • Status:  docker compose -f docker-compose.prod.yml ps        ║
║   • Restart: docker compose -f docker-compose.prod.yml restart   ║
║   • Stop:    docker compose -f docker-compose.prod.yml down      ║
║                                                                   ║
╚═══════════════════════════════════════════════════════════════════╝
EOF
echo -e "${NC}"
