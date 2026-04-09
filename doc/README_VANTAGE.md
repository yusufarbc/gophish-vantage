# Vantage – Gophish Security Operations Hub

**Vantage** is a unified security operations platform that merges **Gophish** (phishing simulation) with **ProjectDiscovery tool suite** (reconnaissance & vulnerability scanning). It provides a single dashboard for managing phishing campaigns alongside automated security assessment workflows.

```
┌──────────────────────────────────────────────────────────────┐
│  PHISHING CAMPAIGNS  +  RECON + VULN SCANNING → ONE PLATFORM │
│                       VANTAGE v2.0                            │
└──────────────────────────────────────────────────────────────┘
```

---

## 🎯 Features

✅ **Phishing Campaign Management**  
   - Full Gophish integration: campaigns, landing pages, templates, SMTP profiles  
   - Real-time results tracking with geo-localization

✅ **ProjectDiscovery Scanner Engine**  
   - Direct CLI wrapper for: Subfinder, HTTPx, Nuclei, Naabu, DNSx, Katana, TLSx, ASNMap, Uncover  
   - Asynchronous scan execution with real-time log streaming via WebSocket

✅ **Unified Dashboard (Tailwind CSS)**  
   - Modern OpenVAS-inspired dark theme  
   - Severity color-coded findings (Purple/Red/Orange/Yellow)  
   - Live terminal-style log viewer  
   - Campaign performance metrics

✅ **REST API**  
   - `/api/scanner/start` – Launch vulnerability scans  
   - `/api/scanner/findings` – Query discovered vulnerabilities  
   - `/api/campaigns/` – Manage phishing campaigns  
   - Websocket: `/ws/scanner/logs` – Stream scan output in real-time

✅ **Enterprise Deployment**  
   - Docker Compose orchestration  
   - Caddy reverse proxy with automatic HTTPS  
   - Basic Auth protection  
   - Postfix local mail integration  
   - Tailscale VPN support for internal scanning

✅ **Reverse L3 Tunneling (Chisel)**  
   - Support for reverse TUN/TAP connections from internal agents  
   - Dynamic interface selection in the UI (e.g., `tun0`)  
   - On-demand routing management for internal network subnets  
   - [See Reverse Tunnel Guide for detailed setup](REVERSE_TUNNEL_GUIDE.md)

---

## 🏗️ Architecture

### Stack

- **Backend**: Go (Gophish + Custom Scanner Engine)
- **Frontend**: HTML/CSS/JS (Tailwind CSS)
- **Database**: SQLite (Gophish DB + Vantage findings DB)
- **Reverse Proxy**: Caddy (TLS, Basic Auth)
- **Mail**: Postfix (SMTP delivery)
- **Orchestration**: Docker Compose

### Database Models

#### Gophish (Standard)
- `User`, `Campaign`, `Group`, `Template`, `SMTP`, `Result`, `MailLog`

#### Vantage Extensions
- **Scan** – Metadata about each scan execution
  ```
  {id, target, tool_name, mode, status, created_at, updated_at}
  ```
- **Finding** – Unified vulnerability findings from all PD tools
  ```
  {id, scan_id, tool_name, severity, name, target, detail, template_id, created_at}
  ```

---

## 🚀 Quick Start

### Prerequisites

- Docker & Docker Compose
- (Optional) Go 1.20+ for local development

### 1. Clone & Configure

```bash
git clone https://github.com/your-org/gophish-vantage.git
cd gophish-vantage

# Copy environment template
cp .env.example .env

# Edit .env with your domain and password hash
# Generate password hash: docker run caddy caddy hash-password "yourpassword"
nano .env
```

### 2. Start Services

```bash
docker-compose up -d

# Verify services are running
docker-compose ps
docker-compose logs -f gophish-app
```

### 3. Access Dashboard

```
https://gophish.example.com/
Username: admin
Password: (from .env VANTAGE_PASSWORD_HASH)
```

### 4. Start a Scan

**Via UI:**
1. Click "Scanner" tab
2. Enter target (domain or IP)
3. Select tool (Nuclei, Subfinder, etc.) or enable Discovery Mode
4. Click "START SCAN"
5. Watch real-time logs

**Via API:**
```bash
curl -X POST http://localhost:3333/api/scanner/start \
  -H "Content-Type: application/json" \
  -H "X-API-KEY: your-gophish-api-key" \
  -d '{
    "target": "example.com",
    "tool": "nuclei",
    "discovery_mode": true
  }'
```

---

## 📋 API Reference

### Scanner Endpoints

#### POST `/api/scanner/start`
Start a vulnerability scan.

**Request:**
```json
{
  "target": "example.com",
  "tool": "nuclei",
  "flags": ["-severity", "critical,high"],
  "discovery_mode": false
}
```

**Response:** `202 Accepted`
```json
{
  "message": "scan started",
  "target": "example.com",
  "mode": "nuclei (Templates)"
}
```

#### GET `/api/scanner/status`
Check current scanner state.

**Response:**
```json
{
  "running": true,
  "tool": "nuclei",
  "target": "example.com",
  "since": "2026-04-07T10:30:00Z"
}
```

#### GET `/api/scanner/findings`
Retrieve findings (supports filters).

**Query params:** `?severity=high&tool=nuclei&limit=50`

**Response:**
```json
[
  {
    "id": 1,
    "severity": "critical",
    "tool_name": "nuclei",
    "name": "SQL Injection in /login",
    "target": "example.com",
    "detail": "CVE-2023-12345",
    "created_at": "2026-04-07T10:35:00Z"
  }
]
```

#### DELETE `/api/scanner/findings/:id` 
Remove a single finding.

#### DELETE `/api/scanner/findings`
Clear all findings (destructive).

#### GET `/api/scanner/stats`
Get severity breakdown of findings.

**Response:**
```json
{
  "total": 42,
  "critical": 3,
  "high": 8,
  "medium": 15,
  "low": 12,
  "info": 4
}
```

### v1 API (Network & Tunnel Management)

#### GET `/api/v1/interfaces`
List all network interfaces on the server.

#### GET `/api/v1/tunnel/status`
Check reverse tunnel server and agent connection status.

#### POST `/api/v1/tunnel/start` | `/stop`
Manage the Chisel tunnel server.

#### POST `/api/v1/tunnel/route`
Add a new IP route to a virtual interface.
```json
{
  "cidr": "192.168.1.0/24",
  "interface": "tun0"
}
```

### Campaign Endpoints

#### GET `/api/campaigns/`
List all phishing campaigns.

#### POST `/api/campaigns/`
Create a new campaign.

#### GET `/api/campaigns/{id}/results`
Get results for a campaign.

---

## 🔧 Configuration

### environment Variables (.env)

```bash
# Domain for Caddy
DOMAIN=gophish.example.com

# Basic Auth password hash (generate via: caddy hash-password)
VANTAGE_PASSWORD_HASH=$2a$14$R9h/cIPz0gi.URNNX3kh2O...

# SMTP Relay (optional)
POSTFIX_RELAYHOST=[smtp.gmail.com]:587
POSTFIX_RELAYHOST_USERNAME=user@gmail.com
POSTFIX_RELAYHOST_PASSWORD=app-password

# Tailscale VPN (optional)
TAILSCALE_AUTH_KEY=tskey-xxxxxxxx

# Gophish Ports
GOPHISH_ADMIN_PORT=3333
GOPHISH_PHISH_PORT=80
```

### Caddyfile

Edit `Caddyfile` to customize TLS, reverse proxy routes, and security headers.

Example:
```
{$DOMAIN} {
    basicauth /api/* {
        admin {$VANTAGE_PASSWORD_HASH}
    }
    reverse_proxy gophish-app:3333
}
```

### Gophish config.json

Located at `./config.json`. Standard Gophish configuration:

```json
{
  "admin_server": {
    "listen_url": "0.0.0.0:3333",
    "use_tls": false,
    "csrf_key": "..."
  },
  "phish_server": {
    "listen_url": "0.0.0.0:80",
    "use_tls": false
  },
  "db_name": "sqlite3",
  "db_path": "./gophish.db"
}
```

---

## 🌐 VPN & Internal Network Scanning

### Tailscale Integration

To scan internal networks via Tailscale:

1. **Uncomment** services in `docker-compose.yml`:
   ```yaml
   tailscale:
     image: tailscale/tailscale:latest
     environment:
       TS_AUTHKEY: ${TAILSCALE_AUTH_KEY}
   ```

2. **Generate** Tailscale auth key at https://login.tailscale.com/admin/settings/keys

3. **Set** `TAILSCALE_AUTH_KEY` in `.env`

4. **Restart** containers:
   ```bash
   docker-compose up -d --force-recreate
   ```

5. All PD tools will now scan through the VPN tunnel.

### WireGuard Alternative

For WireGuard, mount the config file:
```yaml
volumes:
  - ./wg0.conf:/etc/wireguard/wg0.conf
cap_add:
  - NET_ADMIN
  - SYS_MODULE
```

---

## 📧 Postfix Configuration

Gophish uses internal Postfix for mail delivery.

### Local Delivery Only
Default setup (no relay): Emails sent by Gophish are queued locally.

### Relay to External SMTP
Edit `.env`:
```bash
POSTFIX_RELAYHOST=[smtp.gmail.com]:587
POSTFIX_RELAYHOST_USERNAME=your-email@gmail.com
POSTFIX_RELAYHOST_PASSWORD=your-app-password
```

### Test Email Delivery
```bash
docker-compose exec postfix postqueue -p
docker-compose exec postfix postlog
```

---

## 📊 WebSocket Real-Time Logs

The dashboard stream logs via WebSocket at `/ws/scanner/logs`:

```javascript
const ws = new WebSocket(`ws://${window.location.host}/ws/scanner/logs`);
ws.onmessage = (evt) => {
    console.log('[SCANNER]', evt.data);
};
```

---

## 🛡️ Security Considerations

1. **Basic Auth**: Protect admin UI with `basicauth` in Caddyfile
2. **API Key**: Gophish API requires `X-API-KEY` header
3. **HTTPS**: Caddy handles automatic cert provisioning (Let's Encrypt)
4. **Network Isolation**: VPN/internal networks isolated via Docker bridge
5. **DB Access**: SQLite DB persisted in Docker volume (encrypted VM volume recommended)

---

## 📝 Logs & Debugging

### Application Logs
```bash
docker-compose logs gophish-app
```

### Reverse Proxy Logs
```bash
docker-compose logs caddy
```

### Postfix Logs
```bash
docker-compose logs postfix
```

### Scanner Output
Stream live at dashboard `/ws/scanner/logs` or API `/api/scanner/findings`.

---

## 🔄 Updating ProjectDiscovery Tools

To update PD tools inside the container:

```bash
# Option 1: Rebuild Docker image
docker-compose down
docker-compose build --no-cache gophish-app
docker-compose up -d

# Option 2: Exec into container and update
docker-compose exec gophish-app bash
go install -v github.com/projectdiscovery/nuclei/v2/cmd/nuclei@latest
```

---

## 🐛 Troubleshooting

### Scan Not Starting
```bash
# Check scanner status
curl http://localhost:3333/api/scanner/status

# Verify PD tools are installed
docker-compose exec gophish-app which nuclei
```

### WebSocket Connection Failed
- Ensure reverse proxy forwards `/ws/*` paths
- Check Caddyfile for WebSocket configuration

### Postfix Not Delivering Mail
```bash
docker-compose exec postfix mailq
docker-compose logs postfix -f
```

### Basic Auth Not Working
- Regenerate password hash:
  ```bash
  docker run --rm caddy caddy hash-password "newpassword"
  ```
- Update `.env` with new hash
- Restart Caddy: `docker-compose restart caddy`

---

## 📦 Project Structure

```
gophish-vantage/
├── controllers/
│   ├── scanner.go            # Scanner engine (WebSocket, CLI wrapper)
│   ├── api/
│   │   └── scanner.go        # Scanner API endpoints
│   └── route.go              # Routes setup
├── models/
│   └── vantage.go            # Scan & Finding models
├── templates/
│   └── vantage_dashboard.html # Tailwind dashboard
├── static/                    # Gophish static assets
├── docker-compose.yml        # Orchestration
├── Dockerfile               # Build image with PD tools
├── Caddyfile                # Reverse proxy config
├── .env.example             # Environment template
└── config.json              # Gophish config
```

---

## 🎓 Examples

### Scan a Domain with Discovery Mode
```bash
curl -X POST http://localhost:3333/api/scanner/start \
  -H "Content-Type: application/json" \
  -H "X-API-KEY: your-api-key" \
  -d '{
    "target": "acme.com",
    "discovery_mode": true
  }'
```

### Get Critical Findings
```bash
curl "http://localhost:3333/api/scanner/findings?severity=critical" \
  -H "X-API-KEY: your-api-key"
```

### Create Phishing Campaign
```bash
curl -X POST http://localhost:3333/api/campaigns/ \
  -H "Content-Type: application/json" \
  -H "X-API-KEY: your-api-key" \
  -d '{
    "name": "2026-Q2 Security Awareness",
    "template_id": 1,
    "page_id": 1,
    "smtp_id": 1,
    "group_id": 1
  }'
```

---

## 📄 License

This project extends **Gophish** (MIT License) and integrates **ProjectDiscovery** tools.  
See [LICENSE](./LICENSE) for full details.

---

## 🤝 Contributing

Contributions welcome! Please:
1. Fork the repo
2. Create a feature branch
3. Submit a pull request with description

---

## 📧 Support

For issues, questions, or feature requests:
- **GitHub Issues**: https://github.com/your-org/gophish-vantage/issues
- **Security**: Please report privately to security@example.com

---

**Built with ❤️ for offensive security teams** – 2026
