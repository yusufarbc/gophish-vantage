#!/bin/bash
# setup-tun.sh — Initialize a TUN device for Chisel reverse tunnel on the Vantage VPS.
# Run this before starting the Chisel server if the tun0 device does not exist.
# Requires: NET_ADMIN capability (set in docker-compose.yml)
#
# Usage: ./scripts/setup-tun.sh [device_name] [gateway_ip/cidr]
#   device_name   : Name for the TUN device (default: tun0)
#   gateway_ip    : IP/CIDR to assign to the VPS end of the tunnel (default: 10.100.0.1/24)
#
# Example:
#   ./scripts/setup-tun.sh tun0 10.100.0.1/24

set -euo pipefail

DEVICE="${1:-tun0}"
GW_CIDR="${2:-10.100.0.1/24}"

echo "[vantage-tun] Setting up TUN device: $DEVICE ($GW_CIDR)"

# Check if device already exists
if ip link show "$DEVICE" &>/dev/null; then
    echo "[vantage-tun] Device $DEVICE already exists — skipping creation."
else
    ip tuntap add dev "$DEVICE" mode tun
    echo "[vantage-tun] Created TUN device: $DEVICE"
fi

# Assign the gateway IP if not already assigned
if ! ip addr show "$DEVICE" | grep -q "${GW_CIDR%%/*}"; then
    ip addr add "$GW_CIDR" dev "$DEVICE"
    echo "[vantage-tun] Assigned IP $GW_CIDR to $DEVICE"
fi

# Bring the interface up
ip link set "$DEVICE" up
echo "[vantage-tun] Interface $DEVICE is UP"

echo "[vantage-tun] Done. Run: chisel server --port 9090 --reverse"
