#!/bin/bash
# add-route.sh — Add a host route through a specific network interface.
# Called by the Vantage backend (network.SetupRoute) or manually by the operator.
#
# Usage: ./scripts/add-route.sh <cidr> <interface>
#
# Examples:
#   ./scripts/add-route.sh 192.168.1.0/24 tun0
#   ./scripts/add-route.sh 10.10.0.0/16   tun0

set -euo pipefail

CIDR="${1:?Usage: $0 <cidr> <interface>}"
IFACE="${2:?Usage: $0 <cidr> <interface>}"

echo "[vantage-route] Adding route: $CIDR via $IFACE"

# Check if the interface is up
if ! ip link show "$IFACE" | grep -q "UP"; then
    echo "[vantage-route] WARNING: Interface $IFACE does not appear to be UP."
fi

# Add route — fail gracefully if it already exists
if ip route show "$CIDR" dev "$IFACE" &>/dev/null; then
    echo "[vantage-route] Route $CIDR via $IFACE already exists — skipping."
else
    ip route add "$CIDR" dev "$IFACE"
    echo "[vantage-route] Route added: $CIDR via $IFACE"
fi

# Verify
echo "[vantage-route] Verification:"
ip route get "${CIDR%%/*}" || echo "[vantage-route] Could not verify route."
