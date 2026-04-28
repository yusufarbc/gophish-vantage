#!/bin/bash

# ==============================================================================
# Vantage Production Deployment Script
# Mission: Secure, Automated, One-Click VPS Provisioning
# ==============================================================================

set -e

# Colors for UI
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${CYAN}"
cat << "EOF"
  ____   ____  ____    ____   ______   ____   ____  ______ 
 |    \ |    ||    \  /    | |      | |    | |    ||      |
 |  o  ) |  | |  _  ||  o  | |      | |    | |  | |      |
 |   _/  |  | |  |  ||     | |_|  |_| |    | |  | |_|  |_|
 |  |    |  | |  |  ||  _  |   |  |   |    | |  |   |  |  
 |  |    |  | |  |  ||  |  |   |  |   |    | |  |   |  |  
 |__|   |____||__|__||__|__|   |__|   |____| |____| |__|  
                                                              
EOF
echo -e "${NC}"
echo -e "${BLUE}[*] Initializing Vantage SOC Deployment...${NC}"

# 1. Dependency Check (Docker & Docker-Compose)
if ! [ -x "$(command -v docker)" ]; then
    echo -e "${YELLOW}[!] Docker not found. Installing...${NC}"
    curl -fsSL https://get.docker.com -o get-docker.sh
    sudo sh get-docker.sh
    sudo usermod -aG docker $USER
    rm get-docker.sh
fi

if ! [ -x "$(command -v docker-compose)" ]; then
    echo -e "${YELLOW}[!] Docker-Compose not found. Installing...${NC}"
    sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
    sudo chmod +x /usr/local/bin/docker-compose
fi

# 2. Host Directory Setup
echo -e "${BLUE}[*] Creating persistent data volumes...${NC}"
mkdir -p vantage_data/logs vantage_data/caddy_logs
touch vantage_data/vantage.db

# 3. Environment Configuration
if [ ! -f .env ]; then
    echo -e "${YELLOW}[!] Creating .env file. Please update the ADMIN_PASS_HASH!${NC}"
    # Default hash for 'vantage-admin-2024'
    echo "VANTAGE_ADMIN_PASS_HASH=\$2a$14\$S8YV5uGj4L7qR9p2XvA.reYp7i1u9Y0x7S7U8Q9vE6t4u2i1o7G1e" > .env
fi

# 4. Capability Hardening (Host Side)
# Ensures the host allows the container to use these caps
echo -e "${BLUE}[*] Applying kernel capability permissions...${NC}"
sudo sysctl -w net.ipv4.ip_unprivileged_port_start=0 || true

# 5. Launch
echo -e "${GREEN}[*] Building and launching Vantage stack...${NC}"
docker-compose -f docker-compose.prod.yml up -d --build

# 6. Final Status & Banner
echo -e "\n${GREEN}================================================================${NC}"
echo -e "${GREEN}      VANTAGE PLATFORM SUCCESSFULLY DEPLOYED TO PRODUCTION      ${NC}"
echo -e "${GREEN}================================================================${NC}"
echo -e "${CYAN}Admin Dashboard:  ${YELLOW}http://$(curl -s ifconfig.me):3334${NC}"
echo -e "${CYAN}Phishing Gateway: ${YELLOW}http://$(curl -s ifconfig.me):80${NC}"
echo -e "${CYAN}Default Admin:    ${YELLOW}vantage-admin / vantage-admin-2024${NC}"
echo -e "${BLUE}----------------------------------------------------------------${NC}"
echo -e "${RED}CAUTION: Change your password immediately upon login!${NC}"
echo -e "${GREEN}================================================================${NC}\n"
