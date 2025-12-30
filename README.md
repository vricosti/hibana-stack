<p align="center">
  <img src="assets/logo-hibana-stack.png" alt="Hibana Stack" width="300">
</p>

# Hibana Stack

A Go CLI utility that configures a complete multi-domain server with DNS, mail, and web services in one command.

## Overview

Hibana Stack automates the setup of a production-ready server infrastructure on Linux. It handles DNS configuration, mail server deployment with full authentication (DMARC, DKIM, SPF), and containerized web services behind a reverse proxy with automatic SSL certificates.

## Supported Platforms

- **Operating System**: Ubuntu Server (tested on Ubuntu 24.04 LTS)
- **DNS Providers**: OVHcloud, Hostinger

## Architecture

```
                                    ┌─────────────────────────────────────────┐
                                    │              Linux Server               │
                                    │                                         │
    Internet ──────► Traefik ──────►│  ┌─────────┐  ┌─────────┐  ┌─────────┐ │
                   (Reverse Proxy)  │  │   adm   │  │ webmail │  │   www   │ │
                   + SSL/TLS        │  │ Docker  │  │ Docker  │  │ Docker  │ │
                                    │  └─────────┘  └─────────┘  └─────────┘ │
                                    │                                         │
                                    │  ┌─────────────────────────────────────┐│
                                    │  │  Mail Server (Postfix + Dovecot)   ││
                                    │  │  DMARC, DKIM, SPF, SpamAssassin    ││
                                    │  └─────────────────────────────────────┘│
                                    └─────────────────────────────────────────┘
```

### Components

| Component | Description |
|-----------|-------------|
| **Traefik** | Reverse proxy managing SSL certificates via Let's Encrypt |
| **adm** | Web administration interface (Docker container) |
| **webmail** | Roundcube webmail client (Docker container) |
| **www** | Website hosting (Docker container) |
| **Postfix + Dovecot** | Mail server with IMAP/SMTP |
| **SpamAssassin** | Spam filtering |

## Quick Start

```bash
sudo -i
git clone https://github.com/vricosti/hibana-stack.git
cd hibana-stack
./setup.sh
./hibana init
```

## Configuration

Configuration is done interactively when running `hibana init`. The CLI will guide you through:

- Primary domain name
- DNS provider selection (OVHcloud or Hostinger)
- DNS provider credentials
- Email accounts setup
- Web admin credentials

All settings are saved to `hibana-config.yaml` for reference.

## Services After Installation

| Service | URL | Description |
|---------|-----|-------------|
| Admin Panel | `https://adm.yourdomain.com` | Domain and email management |
| Webmail | `https://webmail.yourdomain.com` | Roundcube email client |
| Website | `https://www.yourdomain.com` | Your website |
| Mail Server | `mx.yourdomain.com` | SMTP/IMAP endpoints |

## DNS Records Created

Hibana automatically configures:

- **A/AAAA** records for all subdomains
- **MX** record pointing to the mail server
- **SPF** record for email authentication
- **DKIM** record with generated keys
- **DMARC** record for email policy
- **PTR** records (reverse DNS) when supported

## Requirements

- Ubuntu Server 24.04 LTS
- Root access
- Domain with DNS managed by OVHcloud or Hostinger
- Minimum: 2GB RAM, 20GB disk

## License

Apache License 2.0
