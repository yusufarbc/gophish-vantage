# Vantage Reverse L3 Tunnel & Internal Scanning Guide

This guide explains how to use the **Reverse L3 Tunnel** feature in Vantage to scan internal networks that are behind firewalls or NAT.

---

## 🏗️ How it Works

Vantage uses **Chisel** to establish a secure, reverse tunnel from an internal machine (Agent) to your VPS (Vantage Server). 

1. **Vantage Server**: Runs a Chisel server on port `9090`.
2. **Internal Agent**: Connects to the VPS and requests a reverse TUN device.
3. **Virtual Interface**: A `tun0` interface is automatically created on the VPS.
4. **Routing**: You add routes on the VPS to send traffic for internal CIDRs (e.g., `192.168.1.0/24`) through `tun0`.
5. **Scanning**: ProjectDiscovery tools can then bind to `tun0` to reach the internal targets.

---

## 🚀 Step 1: Start the Tunnel Server

1. Open the Vantage Dashboard and navigate to the **Scanner** tab.
2. Locate the **Reverse Tunnel (Chisel)** panel.
3. Click **Start Server**.
4. The status badge should change to **Server Running** (blue).

---

## 💻 Step 2: Connect the Internal Agent

On the internal machine you want to scan from, you need the Chisel binary.

### 1. Install Chisel (Agent)
```bash
# Linux
curl https://i.jpillora.com/chisel! | bash

# Or via Go
go install github.com/jpillora/chisel@latest
```

### 2. Connect to Vantage
Run the following command on the internal machine (replace `<YOUR-VPS-IP>` with your server's IP):
```bash
chisel client http://<YOUR-VPS-IP>:9090 R:tun0:0.0.0.0
```

> [!TIP]
> **Security**: It is highly recommended to use authentication. Set `VANTAGE_TUNNEL_SECRET` environment variable on the VPS and connect with:
> `chisel client --auth user:pass http://<VPS-IP>:9090 R:tun0:0.0.0.0`

---

## 🌐 Step 3: Manage Routing

Once the agent is connected, the **Vantage Agent** status badge in the sidebar will turn green and show the `tun0` IP address.

### Adding a Route
To scan `192.168.1.0/24`:
1. In the **Scanner** tab under **Route Management**:
2. Enter CIDR: `192.168.1.0/24`
3. Select Interface: `tun0`
4. Click **+ Add Route**.

Now, any traffic from the VPS destined for `192.168.1.0/24` will be encapsulated and sent through the tunnel.

---

## 🔍 Step 4: Perform Internal Scans

### Via UI
1. In the **Execute Scan** form:
2. Enter Target: `192.168.1.50` (or any IP in the routed range)
3. **Outbound Interface**: Select `tun0 ⟵ TUN`.
4. Click **START SCAN**.

### Via API (v1)
```bash
curl -X POST http://<VPS-IP>:3333/api/v1/scanner/start \
  -H "X-API-KEY: <GOPHISH_API_KEY>" \
  -d '{
    "target": "192.168.1.50",
    "tool": "naabu",
    "interface": "tun0"
  }'
```

---

## 🛡️ Reverse Proxy Setup (Detailed)

Since Chisel typically runs over TCP, you have two options for exposing it via Caddy or another reverse proxy.

### Option A: Direct Port Exposure (Recommended for Speed)
In `docker-compose.yml`, port `9090` is already exposed. This allows the Chisel agent to connect directly to the Go backend.
```yaml
ports:
  - "9090:9090"
```

### Option B: Caddy Reverse Proxy (For TLS/HTTPS Wrapper)
If you want to wrap the tunnel in HTTPS via Caddy (port 443) for added security and stealth, update your `Caddyfile`:

```caddy
{$DOMAIN} {
    # Existing Gophish Proxy
    reverse_proxy gophish-app:3333

    # Chisel WebSocket Proxy
    handle /chisel/* {
        reverse_proxy gophish-app:9090
    }
}
```

**Agent Connect Command (HTTPS):**
```bash
chisel client https://gophish.example.com/chisel R:tun0:0.0.0.0
```

---

## ⚠️ Troubleshooting

1. **tun0 doesn't appear**: Ensure the Vantage container was started with `cap_add: NET_ADMIN` and `devices: /dev/net/tun`.
2. **Permission Denied**: Run the `setup-tun.sh` script inside the container if it doesn't auto-initialize.
3. **No route to host**: Verify that `ip route show` inside the container includes your internal CIDR via `tun0`.
4. **Tool bind error**: Some tools require root/sudo to bind to specific interfaces. The Vantage Dockerfile already handles `setcap` for the binary.
