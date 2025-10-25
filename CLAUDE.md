# CLAUDE.md - Hibana Stack Project Context

## Project Overview

Hibana Stack is an open source Go utility that automatically configures an Ubuntu server with a complete domain and email management infrastructure.

## Architecture

### Phase 1: Initial Setup (Go CLI)
The CLI utility configures the server with:
- Primary domain name (maindomain.com)
- PowerDNS for DNS management
- Mail server with DMARC and DKIM
- Traefik as reverse proxy to handle multiple domains
- Web administration interface served via Docker (adm.maindomain.com)

### Phase 2: Web Administration Interface
A web interface accessible via adm.maindomain.com allowing to:
- Add and configure new domains
- Manage mail servers associated with each domain
- Configure DNS records
- Manage SSL certificates via Traefik

## Tech Stack

### Backend
- **Go**: CLI utility and backend API
- **PowerDNS**: Authoritative DNS server
- **Traefik**: Reverse proxy and automatic SSL certificate management
- **Mail server**: Complete setup with DMARC/DKIM

### Frontend
- Dockerized web interface for administration
- Accessible via subdomain adm.maindomain.com

### Infrastructure
- **Target OS**: Ubuntu Server
- **Containerization**: Docker for web interface
- **Configuration**: Automated via Go CLI

## Project Goals

1. Simplify initial configuration of a multi-domain server
2. Provide an all-in-one solution for DNS + Mail + Web hosting
3. Offer an intuitive interface for ongoing management
4. Automate best practices (DKIM, DMARC, SSL)

## Project Structure (planned)

```
hibana-stack/
├── cmd/                    # Main Go CLI
├── internal/              # Internal packages
│   ├── dns/              # PowerDNS configuration
│   ├── mail/             # Mail server configuration
│   ├── traefik/          # Traefik configuration
│   └── installer/        # Installation logic
├── web/                   # Web administration interface
├── docker/                # Dockerfiles and configs
└── docs/                  # Documentation
```

## Installation Flow

1. Run CLI on a fresh Ubuntu server
2. Configure primary domain
3. Install and configure PowerDNS
4. Install and configure mail server (DMARC, DKIM)
5. Install and configure Traefik
6. Deploy web administration interface
7. Access adm.maindomain.com for further configuration

## Development Notes

- Favor idempotent installation operations
- Plan rollbacks in case of errors
- Log all critical operations
- Validate system prerequisites before installation
- Secure administration interface access from the start
