#!/bin/bash

# ═════════════════════════════════════════════════════════════════════════════════
# VANTAGE Docker Entrypoint Script
# Handles initialization, configuration, and graceful startup
# ═════════════════════════════════════════════════════════════════════════════════

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ─────────────────────────────────────────────────────────────────────────────
# Logging Functions
# ─────────────────────────────────────────────────────────────────────────────
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# ─────────────────────────────────────────────────────────────────────────────
# Initialize Configuration
# ─────────────────────────────────────────────────────────────────────────────
initialize_config() {
# Prepare working config (handles read-only bind mounts)
    VANTAGE_CONFIG_PATH="/tmp/config.json"
    cp /opt/vantage/config.json "$VANTAGE_CONFIG_PATH"
    
    # Expand environment variables in config
    sed -i "s|\${DB_PATH:-/opt/vantage/db/vantage.db}|${DB_PATH:-/opt/vantage/db/vantage.db}|g" "$VANTAGE_CONFIG_PATH"
    sed -i "s|\${SMTP_HOST:-postfix}|${SMTP_HOST:-postfix}|g" "$VANTAGE_CONFIG_PATH"
    sed -i "s|\${SMTP_PORT:-25}|${SMTP_PORT:-25}|g" "$VANTAGE_CONFIG_PATH"
    # Fix migrations_prefix to absolute path inside container
    # Local dev uses relative "db/db_", Docker needs the absolute location of copied migrations
    sed -i 's|"migrations_prefix": "db/db_"|"migrations_prefix": "/opt/vantage/migrations/db_"|g' "$VANTAGE_CONFIG_PATH"
    
    log_success "Configuration initialized (at $VANTAGE_CONFIG_PATH)"
}

# ─────────────────────────────────────────────────────────────────────────────
# Verify Capabilities
# ─────────────────────────────────────────────────────────────────────────────
verify_capabilities() {
    log_info "Verifying Linux capabilities..."
    
    # Use capsh to check capabilities if available, otherwise fallback to status grep
    if command -v capsh &> /dev/null; then
        CAPS=$(capsh --print)
        if echo "$CAPS" | grep -q "cap_net_admin"; then
            log_success "NET_ADMIN capability verified"
        else
            log_warn "NET_ADMIN capability not available - TUN/TAP tunneling may not work"
        fi
        
        if echo "$CAPS" | grep -q "cap_net_raw"; then
            log_success "NET_RAW capability verified"
        else
            log_warn "NET_RAW capability not available - naabu/httpx port scanning may not work"
        fi
    else
        # Fallback to /proc/self/status bits check (simplified)
        log_info "capsh not found, skipping advanced capability verification"
    fi
    
    # Check for /dev/net/tun device
    if [ ! -c /dev/net/tun ]; then
        log_warn "/dev/net/tun device not available - TUN/TAP not available"
    fi
    
    log_success "Capability verification complete"
}


# ─────────────────────────────────────────────────────────────────────────────
# Initialize Database
# ─────────────────────────────────────────────────────────────────────────────
initialize_database() {
    log_info "Initializing database..."
    
    DB_DIR=$(dirname "${DB_PATH:-/opt/vantage/db/vantage.db}")
    mkdir -p "$DB_DIR"
    
    if [ -f "$DB_DIR/vantage.db" ]; then
        log_info "Database file already exists, skipping initialization"
    else
        log_info "Creating new database..."
        # Database will be auto-created by Gophish/Vantage on first run
        # This is just ensuring the directory exists
        touch "$DB_DIR/vantage.db"
    fi
    
    log_success "Database initialized"
}

# ─────────────────────────────────────────────────────────────────────────────
# Initialize Nuclei Templates
# ─────────────────────────────────────────────────────────────────────────────
initialize_nuclei() {
    log_info "Checking Nuclei templates..."
    
    NUCLEI_TEMPLATES_DIR="/home/vantage/.nuclei-templates"
    mkdir -p "$NUCLEI_TEMPLATES_DIR"
    
    # Check if templates already cached
    if [ -d "$NUCLEI_TEMPLATES_DIR/nuclei-templates" ] && [ -f "$NUCLEI_TEMPLATES_DIR/nuclei-templates/.gitkeep" ]; then
        TEMPLATE_COUNT=$(find "$NUCLEI_TEMPLATES_DIR" -name "*.yaml" 2>/dev/null | wc -l)
        log_info "Found $TEMPLATE_COUNT cached Nuclei templates"
    else
        log_warn "Nuclei templates not cached - will download on first scan"
        log_info "This may take 5-10 minutes. Templates will be cached for future runs."
    fi
    
    log_success "Nuclei initialization complete"
}

# ─────────────────────────────────────────────────────────────────────────────
# Verify Tool Installation
# ─────────────────────────────────────────────────────────────────────────────
verify_tools() {
    log_info "Verifying ProjectDiscovery tools..."
    
    TOOLS=(
        "subfinder"
        "httpx"
        "nuclei"
        "naabu"
        "dnsx"
        "katana"
        "tlsx"
        "asnmap"
        "uncover"
        "interactsh-client"
        "chisel"
    )
    
    MISSING=0
    for TOOL in "${TOOLS[@]}"; do
        if command -v "$TOOL" &> /dev/null; then
            VERSION=$("$TOOL" -version 2>&1 | head -1 || echo "version unknown")
            log_info "✓ $TOOL ($VERSION)"
        else
            log_warn "✗ $TOOL NOT FOUND"
            MISSING=$((MISSING + 1))
        fi
    done
    
    if [ $MISSING -gt 0 ]; then
        log_warn "Missing $MISSING tools - some scanning features may not work"
    else
        log_success "All tools verified"
    fi
}

# ─────────────────────────────────────────────────────────────────────────────
# Setup Signal Handlers for Graceful Shutdown
# ─────────────────────────────────────────────────────────────────────────────
setup_signal_handlers() {
    # Trap SIGTERM and SIGINT for graceful shutdown
    trap 'log_warn "Received shutdown signal, stopping gracefully..."; kill -TERM $SERVER_PID 2>/dev/null; exit 130' SIGTERM SIGINT
}

# ─────────────────────────────────────────────────────────────────────────────
# Health Check
# ─────────────────────────────────────────────────────────────────────────────
wait_for_service() {
    log_info "Waiting for Vantage to become ready..."
    
    TIMEOUT=120
    ELAPSED=0
    INTERVAL=5
    
    while [ $ELAPSED -lt $TIMEOUT ]; do
        if curl -sf http://localhost:3333/ > /dev/null 2>&1; then
            log_success "Vantage is ready!"
            return 0
        fi
        
        log_info "Waiting... ($((ELAPSED))s)"
        sleep $INTERVAL
        ELAPSED=$((ELAPSED + INTERVAL))
    done
    
    log_error "Vantage failed to start within ${TIMEOUT}s"
}

# ─────────────────────────────────────────────────────────────────────────────
# Banner
# ─────────────────────────────────────────────────────────────────────────────
show_banner() {
    cat <<EOF

${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}
${BLUE}║                                                                ║${NC}
${BLUE}║  VANTAGE - Unified Security Operations Platform               ║${NC}
${BLUE}║  Gophish + ProjectDiscovery Tools + Reverse Tunneling         ║${NC}
${BLUE}║                                                                ║${NC}
${BLUE}║  Starting Docker Container...                                 ║${NC}
${BLUE}║                                                                ║${NC}
${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}

EOF
}

# ─────────────────────────────────────────────────────────────────────────────
# Main Startup
# ─────────────────────────────────────────────────────────────────────────────
main() {
    show_banner
    
    log_info "Starting Vantage initialization sequence..."
    
    # Run initialization steps
    initialize_config
    verify_capabilities
    initialize_database
    initialize_nuclei
    verify_tools
    
    log_success "Initialization complete!"
    log_info ""
    log_info "Starting Vantage Server..."
    log_info ""
    log_info "Available ports:"
    log_info "  - Admin Dashboard: http://localhost:3333"
    log_info "  - HTTP Listener: http://localhost:80"
    log_info "  - HTTPS Listener: https://localhost:443"
    log_info "  - Chisel Tunnel: :8080 (for reverse tunnel agents)"
    log_info ""
    
    # Setup signal handlers
    setup_signal_handlers
    
    # Execute the command passed to entrypoint with the working config
    exec "$@" --config "$VANTAGE_CONFIG_PATH"
}

# Run main
main "$@"
