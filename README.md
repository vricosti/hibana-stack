<p align="center">
  <img src="assets/logo-hibana-stack.png" alt="Hibana Stack" width="300">
</p>

# Hibana Stack

A Go CLI utility that configures a complete multi-domain server with DNS, mail, and web services in one command.

## Overview

Hibana Stack automates the setup of a production-ready server infrastructure on Ubuntu. It handles DNS configuration, mail server deployment with full authentication (DMARC, DKIM, SPF), and containerized web services behind a reverse proxy with automatic SSL certificates.

## Supported Platforms

- **Operating System**: Ubuntu Server (tested on Ubuntu 24.04 LTS)
- **DNS Providers**: OVHcloud, Hostinger

## Architecture

```
                                    ┌─────────────────────────────────────────┐
                                    │              Ubuntu Server              │
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
git clone https://github.com/vricosti/hibana-stack.git
cd hibana-stack
sudo ./setup.sh
nano hibana-config.yaml  # Configure your domain and settings
sudo ./bin/hibana init
```

## Configuration

Edit `hibana-config.yaml`:

```yaml
primary_domain: example.com
server_ip: YOUR_SERVER_IP

dns_provider:
  type: external
  name: ovhcloud  # or "hostinger"
  # For OVHcloud:
  app_key: YOUR_APP_KEY
  app_secret: YOUR_APP_SECRET
  consumer_key: YOUR_CONSUMER_KEY
  # For Hostinger:
  # api_token: YOUR_API_TOKEN

mailserver_subdomain: mx  # Subdomain for mail server

subdomains:
  - name: adm
    role: webadmin
  - name: mx
    role: mailserver
  - name: webmail
    role: webmail
  - name: www
    role: website

email_accounts:
  - username: contact
    password: SECURE_PASSWORD
    full_name: Contact

webadmin:
  username: admin
  password: SECURE_PASSWORD

domain_user:
  ssh_key_mode: auto
```

## DNS Provider Setup

### OVHcloud

1. Go to https://manager.eu.ovhcloud.com/#/iam/api-keys/onboarding
2. Create API credentials with these permissions:
   - `GET/POST/PUT/DELETE /domain/zone/*` (DNS management)
   - `GET/POST/PUT/DELETE /ip/*` (PTR records)

### Hostinger

1. Access Hostinger control panel
2. Generate an API token with DNS management permissions

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

## Adding Domains

After initial setup, add more domains via the web admin or CLI:

```bash
sudo hibana add domain newdomain.com
```

Each domain gets:
- A dedicated system user (e.g., `example-com`)
- A home directory at `/srv/example-com/`
- Isolated Docker containers

## Requirements

- Ubuntu Server 24.04 LTS
- Root access
- Domain with DNS managed by OVHcloud or Hostinger
- Minimum: 2GB RAM, 20GB disk

## License

Apache License 2.0
