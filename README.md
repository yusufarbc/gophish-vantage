# Vantage (Gophish Security Operations Hub)

![Vantage Logo](https://raw.githubusercontent.com/gophish/gophish/master/static/images/gophish_purple.png)

**Vantage** is an advanced fork of **Gophish**, transformed into a unified Security Operations Hub. It integrates industry-standard reconnaissance and vulnerability scanning tools directly into the phishing simulation workflow.

---

## 🚀 Key Features

*   **Unified Dashboard**: Manage phishing campaigns and security scans from a single modern UI.
*   **Vulnerability Scanning**: Direct integration with **ProjectDiscovery** tools (Nuclei, Subfinder, HTTPx, etc.).
*   **Reverse L3 Tunneling**: Perform internal network scans through a secure Chisel-based reverse tunnel.
*   **Real-time Insights Discovery**: WebSocket-based live scan logs and campaign performance tracking.
*   **Enterprise-Ready Deployment**: Orchestrated via Docker Compose with Caddy (HTTPS) and Postfix integration.

---

## 📚 Documentation

For detailed setup and usage instructions, please refer to the following guides:

*   📖 **[Vantage Overview & API Reference](README_VANTAGE.md)** - Main project documentation.
*   🚀 **[Deployment & Operations Guide](DEPLOYMENT_GUIDE.md)** - Step-by-step VPS/Server deployment.
*   🌐 **[Reverse L3 Tunnel Guide](REVERSE_TUNNEL_GUIDE.md)** - **[NEW]** Setup and usage for internal scanning.

---

## 🏗️ Quick Start

```bash
git clone https://github.com/your-org/gophish-vantage.git
cd gophish-vantage
cp .env.example .env
# Edit .env and start
docker-compose up -d
```

Access your dashboard at `https://yourdomain.com/` (as configured in `.env`).

---

## ⚖️ License

This project extends **Gophish** (MIT License) and integrates **ProjectDiscovery** tools.
See [LICENSE](./LICENSE) for full details.

---

**Built with ❤️ for offensive security teams.**
