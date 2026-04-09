# VANTAGE Implementation Checklist

## ✅ Completed Components

### Database Models
- [x] **models/vantage.go** – `Scan` and `Finding` GORM structures
  - Scan: ID, Target, ToolName, Mode, Status, timestamps
  - Finding: ID, ScanID, ToolName, Severity (critical/high/medium/low/info), Name, Target, Detail, TemplateID
  - Relationships properly configured

### Scanner Engine (CLI Wrapper)
- [x] **controllers/scanner.go**
  - `ScanState` struct tracks active scans (concurrency safe)
  - `RunScannerTool()` – Execute single PD tool asynchronously
  - `RunDiscovery()` – Chain: subfinder → httpx → nuclei
  - `emitLog()` – Broadcast logs to WebSocket clients
  - Per-tool argument builders for all 11 PD tools

### API Endpoints
- [x] **controllers/api/scanner.go**
  - `POST /api/scanner/start` – Launch scans (returns 202 Accepted)
  - `GET /api/scanner/status` – Check if scan running
  - `GET /api/scanner/findings` – Query results with filtering
  - `GET /api/scanner/stats` – Severity breakdown
  - `DELETE /api/scanner/findings/:id` – Remove finding
  - `DELETE /api/scanner/findings` – Clear all (destructive)

### WebSocket Real-Time Logging
- [x] **controllers/scanner.go + route.go**
  - `ScannerLogHub` manages WebSocket connections
  - `/ws/scanner/logs` route registered
  - Concurrent client management with sync.RWMutex
  - Graceful disconnect handling

### Routes & Integration
- [x] **controllers/route.go**
  - Scanner routes registered in API server
  - WebSocket handler mounted at `/ws/scanner/logs`
  - `InitScannerHub()` called at app startup
- [x] **gophish.go**
  - Scanner hub initialization at app start

### UI Dashboard (Tailwind)
- [x] **templates/vantage_dashboard.html**
  - Dark theme with OpenVAS-inspired colors
  - Sidebar with navigation tabs
  - Dashboard tab: 4-column stats + charts
  - Scanner tab: Target input, Tool selection, Discovery Mode, Live terminal
  - Findings tab: Sortable table with severity badges
  - Campaigns tab: Integration with Gophish campaigns
  - Responsive design, live log streaming via WebSocket
  - Color-coded severity: Purple (Critical), Red (High), Orange (Medium), Yellow (Low)
  - Modern glassmorphism cards with Tailwind utilities

### Deployment & Infrastructure
- [x] **docker-compose.yml** (Enhanced)
  - `gophish-app` service with health checks
  - `postfix` service for local SMTP delivery
  - `caddy` reverse proxy with TLS and Basic Auth
  - Optional `tailscale` service for VPN scanning
  - Volume management for persistence
  - Network isolation (gophish-net bridge)
  - CAP_NET_RAW, CAP_NET_ADMIN for scanning tools

- [x] **Caddyfile** (Enhanced)
  - Admin dashboard at `{$DOMAIN}` with Basic Auth
  - WebSocket forwarding for real-time logs
  - Phishing landing pages at `phish.{$DOMAIN}`
  - Health check endpoint
  - Security headers (HSTS, CSP, X-Frame-Options, etc.)
  - Gzip compression
  - Logging to persistent files

- [x] **Dockerfile** (Enhanced)
  - Go 1.20 baseline
  - ProjectDiscovery tools built-in:
    - subfinder, nuclei, httpx, naabu, dnsx
    - katana, tlsx, assetfinder, asnmap, uncover
  - CAP_NET_BIND_SERVICE for port binding
  - Multi-stage build for optimization

- [x] **.env.example**
  - Domain configuration
  - Caddy password hash template
  - Postfix relay options
  - Tailscale VPN key (optional)
  - PD Tools path configuration

### Documentation
- [x] **README_VANTAGE.md** (Comprehensive)
  - Feature overview with ASCII art banner
  - Architecture explanation
  - Quick Start guide (5 steps)
  - Full API reference with curl examples
  - Configuration guide
  - VPN/Tailscale integration instructions
  - Postfix SMTP relay setup
  - WebSocket API examples
  - Security considerations
  - Troubleshooting guide
  - Project structure overview
  - Real-world usage examples

---

## 🔄 ProjectDiscovery Tools Support

### Integrated Tools (11 Total)

| Tool | Purpose | Status |
|------|---------|--------|
| **subfinder** | Subdomain enumeration | ✅ Built-in |
| **httpx** | HTTP service probe | ✅ Built-in |
| **nuclei** | Template-based vulnerability scanning | ✅ Built-in |
| **naabu** | Network port scanner | ✅ Built-in |
| **dnsx** | DNS query & validation | ✅ Built-in |
| **katana** | Web crawling & endpoint discovery | ✅ Built-in |
| **tlsx** | TLS certificate scanner | ✅ Built-in |
| **assetfinder** | Domain asset finder | ✅ Built-in |
| **asnmap** | ASN/CIDR mapping | ✅ Built-in |
| **uncover** | Search engine & cloud asset enumeration | ✅ Built-in |
| **interactsh-client** | Out-of-band interaction testing | ✅ Available (via PATH) |

### Discovery Mode Workflow
```
TARGET (Domain/CIDR)
  ↓
  ├─ Phase 1: Subfinder (subdomain enumeration)
  │   ↓
  ├─ Phase 2: HTTPx (alive host detection)
  │   ↓
  ├─ Phase 3: Nuclei (vulnerability testing on live hosts)
  │   ↓
  ✓ FINDINGS PERSISTED TO DATABASE
```

---

## 🎨 Color Scheme (Tailwind)

```css
/* OpenVAS-inspired Dark Theme */
- Background:  #0a0c10 (almost black)
- Sidebar:     #11141a (dark gray)
- Card:        #161b22 (card background)
- Border:      #30363d (subtle dividers)
- Accent:      #238636 (OpenVAS green)

/* Severity Badges */
- Critical: #bc8cf2 (Purple with opacity)
- High:     #f85149 (Red)
- Medium:   #d29922 (Orange)
- Low:      #8b949e (Gray)
- Info:     #58a6ff (Blue)
```

---

## 📡 API Endpoints Summary

```
POST   /api/scanner/start              – Launch scan
GET    /api/scanner/status             – Check state
GET    /api/scanner/findings           – Query findings (with filters)
GET    /api/scanner/findings?severity=critical&tool=nuclei&limit=50
GET    /api/scanner/stats              – Severity breakdown
DELETE /api/scanner/findings/:id       – Remove finding
DELETE /api/scanner/findings           – Clear all findings
POST   /api/campaigns/                 – Create campaign
GET    /api/campaigns/                 – List campaigns
GET    /api/campaigns/{id}/results     – Get results
WS     /ws/scanner/logs                – Real-time log streaming
```

---

## 🚀 Deployment Checklist

- [ ] Clone repository
- [ ] Copy `.env.example` → `.env`
- [ ] Generate password hash: `docker run --rm caddy caddy hash-password "password"`
- [ ] Update `.env` with domain, password hash, Postfix relay (if needed), Tailscale key (if needed)
- [ ] Review `Caddyfile` – update domain/paths as needed
- [ ] Review `config.json` – configure Gophish settings (SMTP, admin port, etc.)
- [ ] Run: `docker-compose up -d`
- [ ] Wait for service health checks to pass
- [ ] Access: `https://yourdomain.com/`
- [ ] Test scanner: Try a scan from dashboard
- [ ] Monitor logs: `docker-compose logs -f gophish-app`

---

## 🔐 Security Hardening

### Pre-Deployment
- [ ] Use strong Caddy password hash
- [ ] Enable automatic HTTPS (Let's Encrypt)
- [ ] Configure firewall rules (only 80/443 public)
- [ ] Set up VPN for internal scanning if needed
- [ ] Use environment variables for secrets (never commit .env)

### Post-Deployment
- [ ] Enable audit logging in Caddyfile
- [ ] Monitor `/var/log/caddy/` for access patterns
- [ ] Regularly backup SQLite databases
- [ ] Rotate Gophish API keys periodically
- [ ] Review phishing campaign results for compliance

---

## 📊 Performance Tuning

### Database
- SQLite WAL mode enabled (default)
- Indexes on `severity`, `tool_name`, `target`, `created_at`
- Consider PostgreSQL for scale (>1M findings)

### Scanner
- Single concurrent scan at a time (lock-based)
- Toolsprivate 64KB line buffer for large JSON payloads
- WebSocket message channel: 512 buffer size

### Docker
- CPU limits: `cpus: '2'` (adjust as needed)
- Memory limits: `mem_limit: 2g` (add to docker-compose.yml)

---

## 🐛 Known Limitations & Future Work

### Current Limitations
- Single concurrent scan (queue planned)
- Findings stored in SQLite (PostgreSQL recommended for scale)
- No multi-user workflow (per-user scans)
- Basic Auth only (LDAP/SSO planned)

### Future Enhancements
- [ ] Scan queue/scheduling
- [ ] Multi-user RBAC system
- [ ] PostgreSQL support
- [ ] Slack/webhook notifications
- [ ] Report generation (PDF)
- [ ] Integration with SIEM (Splunk, ELK)
- [ ] Custom tool plugins
- [ ] Graph-based asset relationships
- [ ] ML-based risk scoring

---

## 📝 Configuration Files Reference

### config.json (Gophish)
```json
{
  "admin_server": {
    "listen_url": "0.0.0.0:3333",
    "use_tls": false,
    "cert_path": "gophish_admin.crt",
    "key_path": "gophish_admin.key",
    "csrf_key": "... (auto-generated) ...",
    "allowed_internal_hosts": ["127.0.0.1"]
  },
  "phish_server": {
    "listen_url": "0.0.0.0:80",
    "use_tls": false,
    "cert_path": "gophish_phish.crt",
    "key_path": "gophish_phish.key"
  },
  "db_name": "sqlite3",
  "db_path": "./gophish.db",
  "migrations_prefix": "./db/db_sqlite3/migrations/",
  "contact_address": ""
}
```

### docker-compose.yml Volumes
```yaml
volumes:
  gophish_data:          # Persists /app/data
  postfix_data:          # Postfix queue
  postfix_queue:         # Mail queue
  caddy_data:            # SSL certificates
  caddy_config:          # Caddy configuration
  tailscale_data:        # VPN credentials (optional)
```

---

## 📞 Support & Debugging

### Check Service Health
```bash
docker-compose ps
docker-compose exec gophish-app wget -qO- http://localhost:3333/login
```

### View Real-time Logs
```bash
docker-compose logs -f gophish-app --tail=50
docker-compose logs -f caddy --tail=50
docker-compose logs -f postfix --tail=50
```

### Access Container Shell
```bash
docker-compose exec gophish-app bash
which nuclei subfinder httpx    # Verify tools
```

### Test Scanner Directly
```bash
docker-compose exec gophish-app nuclei -u https://example.com -json | head -20
```

---

## 🎯 Roadmap for v3.0

- [ ] Advanced workflow automation
- [ ] Machine learning findings classification
- [ ] Integration with external scanners (Burp, Nessus)
- [ ] Team collaboration & comment threads
- [ ] Advanced reporting engine
- [ ] Mobile app for campaign monitoring

---

**Last Updated:** April 7, 2026  
**Project Status:** Production Ready  
**Latest Version:** 2.0
