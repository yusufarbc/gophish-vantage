# ========================================================================================
# VANTAGE: Enterprise Multi-Stage Dockerfile
# Gophish + ProjectDiscovery Tools + Reverse Tunnel Infrastructure
# Final production image: Minimal Debian with compiled binaries only
# ========================================================================================

# ========================================================================================
# STAGE 1: Asset Minification (JavaScript/CSS)
# ========================================================================================
FROM node:20-alpine AS asset-builder
RUN npm install -g gulp gulp-cli
WORKDIR /build
COPY package.json ./
RUN npm install --only=dev
COPY . .
RUN gulp

# ========================================================================================
# STAGE 2: ProjectDiscovery Tools Builder
# ========================================================================================
FROM golang:latest AS pd-tools-builder
RUN apt-get update && apt-get install -y --no-install-recommends \
    libpcap-dev libdumbnet-dev gcc g++ make && \
    rm -rf /var/lib/apt/lists/*

ENV GOPROXY=https://proxy.golang.org,direct CGO_ENABLED=1
RUN set -eux; \
    # Minimal required tools - stable set for reliable container startup.
    go install -v github.com/projectdiscovery/subfinder/v2/cmd/subfinder@v2.12.0; \
    go install -v github.com/projectdiscovery/httpx/cmd/httpx@v1.6.0; \
    go install -v github.com/projectdiscovery/nuclei/v3/cmd/nuclei@v3.1.0; \
    go install -v github.com/projectdiscovery/interactsh/cmd/interactsh-client@v1.2.0; \
    true

# ========================================================================================
# STAGE 3: Gophish/Vantage Backend Builder
# ========================================================================================
FROM golang:latest AS app-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o vantage-server .

# ========================================================================================
# STAGE 4: Production Runtime
# ========================================================================================
FROM debian:bullseye-slim

LABEL maintainer="Vantage Security Platform" \
      description="Unified Gophish + ProjectDiscovery Security Operations Hub v2.0"

# Install minimal runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    libpcap0.8 ca-certificates iproute2 iptables libcap2-bin \
    libgomp1 libdumbnet1 curl wget jq dnsmasq && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Create application user
RUN groupadd -r vantage && useradd -r -g vantage vantage

WORKDIR /opt/vantage

# Copy ProjectDiscovery tools
COPY --from=pd-tools-builder /go/bin/* /usr/local/bin/

# Copy Vantage application
COPY --from=app-builder /app/vantage-server /opt/vantage/
COPY --from=app-builder /app/templates /opt/vantage/templates
COPY --from=app-builder /app/static /opt/vantage/static
COPY --from=asset-builder /build/static/js/dist /opt/vantage/static/js/dist
COPY --from=asset-builder /build/static/css/dist /opt/vantage/static/css/dist
COPY --from=app-builder /app/config.json /opt/vantage/config.json.example
COPY docker/docker-entrypoint.sh /opt/vantage/docker-entrypoint.sh

# Prepare directories
RUN mkdir -p /opt/vantage/db /root/.nuclei-templates /root/.config/nuclei && \
    chown -R vantage:vantage /opt/vantage /root/.nuclei-templates /root/.config/nuclei && \
    chmod +x /opt/vantage/docker-entrypoint.sh

# Set Linux capabilities for network operations (only for binaries that exist)
RUN set -eux; \
    [ -f /usr/local/bin/naabu ] && setcap cap_net_raw,cap_net_admin=ep /usr/local/bin/naabu || true; \
    [ -f /usr/local/bin/httpx ] && setcap cap_net_raw=ep /usr/local/bin/httpx || true; \
    [ -f /usr/local/bin/chisel ] && setcap cap_net_admin=ep /usr/local/bin/chisel || true; \
    [ -f /opt/vantage/vantage-server ] && setcap cap_net_bind_service=ep /opt/vantage/vantage-server || true

# Verify capabilities (non-fatal)
RUN set -eux; \
    [ -f /usr/local/bin/naabu ] && getcap /usr/local/bin/naabu || true; \
    [ -f /usr/local/bin/httpx ] && getcap /usr/local/bin/httpx || true; \
    [ -f /usr/local/bin/chisel ] && getcap /usr/local/bin/chisel || true; \
    [ -f /opt/vantage/vantage-server ] && getcap /opt/vantage/vantage-server || true

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
    CMD curl -f http://localhost:3333/ || exit 1

# Expose ports
# 3333: Gophish Admin & Vantage Dashboard
# 80/443: HTTP(S) Phishing Redirector
# 8080: Chisel Reverse Tunnel Listener
# 9090: Reverse TUN Server
EXPOSE 3333 80 443 8080 9090

# Security context
USER vantage
ENTRYPOINT ["/opt/vantage/docker-entrypoint.sh"]
CMD ["./vantage-server"]

