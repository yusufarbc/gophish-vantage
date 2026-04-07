# Vantage Deployment & Operations Guide

## 🚀 Initial Deployment (VPS/Cloud)

### Step 1: Server Preparation

**Minimum Requirements:**
- OS: Ubuntu 22.04 LTS or Debian 12
- CPU: 4 cores
- RAM: 8GB
- Storage: 100GB SSD
- Network: Public IP + domain

**System Setup:**
```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker & Docker Compose
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# Install Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

# Verify installation
docker --version
docker-compose --version
```

### Step 2: Configure DNS

Point your domain to the server IP:
```
gophish.example.com  A  1.2.3.4
phish.example.com    A  1.2.3.4  (optional subdomain for landing pages)
landing.example.com  A  1.2.3.4  (optional alias)
```

### Step 3: Clone & Prepare

```bash
cd /opt
sudo git clone https://github.com/your-org/gophish-vantage.git
cd gophish-vantage

# Set proper permissions
sudo chown -R $USER:$USER .

# Copy environment template
cp .env.example .env
```

### Step 4: Generate Caddy Password Hash

```bash
# Option 1: Online (quick)
# Visit: https://caddyserver.com/docs/caddyfile/directives/basicauth

# Option 2: Local Docker
docker run --rm caddy caddy hash-password "your-secure-password"
# Output: $2a$14$R9h/cIPz0gi.URNNX3kh2OPST9/PgBkqquzi.Ee3D6xVxQo0kK7dm
```

### Step 5: Configure .env

```bash
nano .env
```

Edit accordingly:
```bash
DOMAIN=gophish.example.com
VANTAGE_PASSWORD_HASH=$2a$14$R9h/cIPz0gi.URNNX3kh2O...

# If using SMTP relay (Gmail example):
POSTFIX_RELAYHOST=[smtp.gmail.com]:587
POSTFIX_RELAYHOST_USERNAME=your-email@gmail.com
POSTFIX_RELAYHOST_PASSWORD=your-app-password

# Optional: Tailscale
TAILSCALE_AUTH_KEY=tskey-xxxxxxxx
```

### Step 6: Start Services

```bash
# Build image (includes ProjectDiscovery tools)
docker-compose build

# Start all services
docker-compose up -d

# Wait for health checks
sleep 10
docker-compose ps

# Expected output:
# gophish-app    running (healthy)
# postfix        running
# caddy          running
```

### Step 7: Initial Access

```bash
# Check Caddy status
docker-compose logs caddy -f --tail=20

# Wait for "Serving HTTPS..." message
# Then access: https://gophish.example.com/
# Username: admin
# Password: (your password from step 4)
```

### Step 8: Reverse Tunnel Configuration (Optional)

Vantage includes a built-in Chisel-based reverse tunnel server on port **9090**.

1. **Verify exposure**: Port `9090` must be reachable for reverse agents.
2. **Caddy Proxying (Recommended)**: For security, proxy port `9090` via Caddy (port 443) by adding this to your `Caddyfile`:

```caddy
{$DOMAIN} {
    # ... other routes
    handle /chisel/* {
        reverse_proxy gophish-app:9090
    }
}
```

3. **Client Connection**: 
```bash
chisel client https://yourdomain.com/chisel R:tun0:0.0.0.0
```

[Detailed Guide: REVERSE_TUNNEL_GUIDE.md](REVERSE_TUNNEL_GUIDE.md)

---

## 📋 Post-Deployment Configuration

### 1. Configure Gophish Admin User

First login:
1. Access Gophish at `https://gophish.example.com/`
2. You'll see login page (default: admin / password from first_password.txt)
3. If no password, check logs: `docker-compose logs gophish-app | grep password`

### 2. Create SMTP Profile (for phishing emails)

Via UI or API:
```bash
curl -X POST https://gophish.example.com/api/smtp/ \
  -H "Content-Type: application/json" \
  -H "X-API-KEY: your-gophish-api-key" \
  -d '{
    "name": "Local Postfix",
    "host": "postfix:25",
    "from_address": "security@yourdomain.com",
    "username": "",
    "password": "",
    "ignore_cert_errors": false
  }'
```

### 3. Import Email Lists & Groups

Upload CSV to Gophish UI for target groups.

### 4. Create Email Templates

Use Gophish UI or import custom HTML.

### 5. Create Landing Pages

Capture credentials or track interactions.

---

## 🔧 Operations & Monitoring

### Health Checks

```bash
# Check all services
docker-compose ps

# Verify Caddy is routing correctly
curl -I https://gophish.example.com/ -k
# Expected: 401 Unauthorized (Basic Auth required)

# Check scanner connectivity
curl http://localhost:3333/api/scanner/status \
  -H "X-API-KEY: your-api-key"
# Expected: {"running":false}
```

### Log Monitoring

```bash
# Real-time logs (follow mode)
docker-compose logs -f

# Specific service logs
docker-compose logs -f gophish-app
docker-compose logs -f caddy --tail=50
docker-compose logs -f postfix

# Date-filtered logs
docker-compose logs --since 2026-04-07T10:30:00 gophish-app
```

### Database Backup

```bash
# Backup Gophish database
docker-compose exec gophish-app cp ./gophish.db ./gophish-backup-$(date +%Y%m%d-%H%M%S).db

# Copy from container to host
docker cp gophish-app:/opt/gophish/gophish.db ./backup/gophish-$(date +%Y%m%d).db

# Restore (if needed)
docker-compose down
docker cp ./backup/gophish-2026-04-07.db gophish-app:/opt/gophish/gophish.db
docker-compose up -d
```

### Automated Backups (Optional)

```bash
# Create cron job for daily backup
crontab -e

# Add line:
0 2 * * * cd /opt/gophish-vantage && docker-compose exec -T gophish-app cp ./gophish.db ./backups/gophish-$(date +\%Y\%m\%d).db
```

---

## 🔄 Scanning Operations

### Start a Scan via Dashboard

1. Navigate to **Scanner** tab
2. Enter target (domain, IP, or CIDR)
3. Select tool or enable Discovery Mode
4. Click **START SCAN**
5. Watch real-time logs

### Start a Scan via API

```bash
# Single tool scan
curl -X POST https://gophish.example.com/api/scanner/start \
  -H "Content-Type: application/json" \
  -H "X-API-KEY: your-api-key" \
  -u admin:your-password \
  -d '{
    "target": "acme.com",
    "tool": "nuclei",
    "flags": ["-severity", "critical,high"]
  }'

# Discovery mode (full chain)
curl -X POST https://gophish.example.com/api/scanner/start \
  -H "Content-Type: application/json" \
  -H "X-API-KEY: your-api-key" \
  -u admin:your-password \
  -d '{
    "target": "example.com",
    "discovery_mode": true
  }'
```

### Query Findings

```bash
# All findings
curl https://gophish.example.com/api/scanner/findings \
  -H "X-API-KEY: your-api-key" \
  -u admin:your-password

# Critical findings only
curl 'https://gophish.example.com/api/scanner/findings?severity=critical' \
  -H "X-API-KEY: your-api-key" \
  -u admin:your-password

# Filter by tool and limit
curl 'https://gophish.example.com/api/scanner/findings?tool=nuclei&severity=high&limit=10' \
  -H "X-API-KEY: your-api-key" \
  -u admin:your-password
```

### Advanced Scanning Scenarios

#### Scenario 1: Internal Network via Tailscale

1. Uncomment Tailscale in `docker-compose.yml`
2. Generate auth key at https://login.tailscale.com/admin/settings/keys
3. Set `TAILSCALE_AUTH_KEY=tskey-...` in `.env`
4. Restart: `docker-compose up -d --force-recreate`
5. Now scan internal IPs: `10.0.0.0/8`, `172.16.0.0/12`, etc.

#### Scenario 2: Bulk Domain Scanning

```bash
# Create domains.txt
cat > domains.txt << EOF
example.com
acme.com
test.io
EOF

# Scan each domain with Nuclei
for domain in $(cat domains.txt); do
  curl -X POST https://gophish.example.com/api/scanner/start \
    -H "Content-Type: application/json" \
    -H "X-API-KEY: your-api-key" \
    -u admin:your-password \
    -d '{
      "target": "'$domain'",
      "tool": "nuclei"
    }'
  sleep 5  # Rate limit
done
```

#### Scenario 3: Export Findings to CSV

```bash
# Query findings as JSON, convert to CSV
curl https://gophish.example.com/api/scanner/findings \
  -H "X-API-KEY: your-api-key" \
  -u admin:your-password | \
  jq -r '.[] | [.severity, .tool_name, .name, .target] | @csv' > findings.csv
```

---

## 🛡️ Security Operations

### Regular Tasks

#### Weekly
- [ ] Review scan logs for anomalies
- [ ] Check Caddy certificate validity (auto-renewed by Let's Encrypt)
- [ ] Monitor disk usage: `df -h`
- [ ] Update ProjectDiscovery tools:
  ```bash
  docker-compose exec gophish-app go install -u github.com/projectdiscovery/nuclei/v2/cmd/nuclei@latest
  ```

#### Monthly
- [ ] Backup databases
- [ ] Review Gophish campaign results
- [ ] Rotate API keys (regenerate in Gophish UI)
- [ ] Update Docker images: `docker-compose pull && docker-compose up -d`

#### Quarterly
- [ ] Security audit of configurations
- [ ] Review Postfix relay settings
- [ ] Assess storage usage and retention policy

### Incident Response

**Scan Stuck/Frozen:**
```bash
# Check status
docker-compose exec gophish-app curl http://localhost:3333/api/scanner/status

# If running=true but appears hung, restart:
docker-compose restart gophish-app
```

**High Memory Usage:**
```bash
# Monitor memory
docker stats gophish-app

# Limit memory in docker-compose.yml:
deploy:
  resources:
    limits:
      memory: 2G  # Add this
```

**Phishing Emails Not Delivering:**
```bash
# Check Postfix queue
docker-compose exec postfix postqueue -p

# View Postfix logs
docker-compose logs postfix | grep error

# Force queue flush
docker-compose exec postfix postfix flush
```

---

## 📊 Reporting & Analytics

### Via Dashboard
1. Go to **Findings** tab
2. Filter by severity/tool
3. Export or screenshot

### Via API - Generate Report

```bash
#!/bin/bash
# Generate markdown report

echo "# Vantage Security Report - $(date +%Y-%m-%d)"
echo ""

# Stats
curl -s https://gophish.example.com/api/scanner/stats \
  -H "X-API-KEY: $API_KEY" \
  -u admin:$PASSWORD | \
  jq 'to_entries | .[] | "- \(.key): \(.value)"'

echo ""
echo "## Critical Findings"
curl -s "https://gophish.example.com/api/scanner/findings?severity=critical" \
  -H "X-API-KEY: $API_KEY" \
  -u admin:$PASSWORD | \
  jq '.[] | "- [\(.name)](\(.target)) - \(.detail)"'
```

### Custom Webhook for Notifications

Add to Gophish config.json:
```json
"webhooks": {
  "url": "https://your-webhook-service.com/gophish",
  "secret": "webhook-secret"
}
```

---

## 🔐 Security Hardening Checklist

- [ ] Change default Gophish API key
- [ ] Enable HTTPS with valid certificates (Let's Encrypt via Caddy)
- [ ] Use strong Caddy password
- [ ] Restrict Docker port exposure (only 80/443 public)
- [ ] Enable Postfix relay authentication
- [ ] Use VPN for internal network scanning
- [ ] Implement rate limiting for API
- [ ] Setup audit logs (Caddy logging)
- [ ] Regular backups (encrypted storage)
- [ ] Monitor for unauthorized access logs

---

## 🆘 Troubleshooting

### Issue: Can't Access Dashboard

```bash
# Check if services running
docker-compose ps

# If not, start:
docker-compose up -d

# Check Caddy logs for routing errors
docker-compose logs caddy
```

### Issue: Scanner Not Working

```bash
# Verify tools are installed
docker-compose exec gophish-app which nuclei subfinder httpx

# If missing, rebuild:
docker-compose build --no-cache
docker-compose up -d

# Check scanner status
curl http://localhost:3333/api/scanner/status
```

### Issue: Phishing Emails Not Sent

```bash
# Verify Postfix is running
docker-compose logs postfix

# Check mail queue
docker-compose exec postfix postqueue -p

# Test SMTP connectivity
docker-compose exec gophish-app telnet postfix 25
```

### Issue: WebSocket Connection Failed

```bash
# Check Caddyfile WebSocket configuration
vim Caddyfile

### Issue: Basic Auth Not Working
- Regenerate password hash:
  ```bash
  docker run --rm caddy caddy hash-password "newpassword"
  ```
- Update `.env` with new hash
- Restart Caddy: `docker-compose restart caddy`

### Issue: Reverse Tunnel (tun0) Not Appearing
- Ensure Vantage container is in **privileged mode** or has `NET_ADMIN` + `/dev/net/tun`.
- Restart: `docker-compose up -d --force-recreate`.
- Check if Chisel server is started in the Dashboard (Scanner tab).
- See [REVERSE_TUNNEL_GUIDE.md](REVERSE_TUNNEL_GUIDE.md) for full steps.
```

---

## 📚 Additional Resources

- **Gophish Docs**: https://getgophish.com/
- **ProjectDiscovery**: https://projectdiscovery.io/
- **Docker Compose**: https://docs.docker.com/compose/
- **Caddy Documentation**: https://caddyserver.com/docs/
- **Tailwind CSS**: https://tailwindcss.com/

---

**Deployment Date:** ________________  
**Last Updated:** ________________  
**Backup Location:** ________________  
**Support Contact:** ________________

---

**Next Steps:**
1. Complete deployment checklist
2. Test scanner with sample domain
3. Configure SMTP for phishing campaigns
4. Schedule regular backups
5. Monitor logs for errors
