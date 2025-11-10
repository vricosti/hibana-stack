# CLAUDE.md - Hibana Stack Project Context

## Project Overview

Hibana Stack is an open source Go utility that provides a complete solution for managing a multi-domain server. It enables easy server configuration through a command-line interface in Phase 1, followed by a web-based administration interface in Phase 2 for ongoing domain and email management.

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
- Optionally add webmail for each domain
- Manage email accounts (create, modify, delete)
- Configure email redirections and aliases
- Manage DNS records for domains
- Configure mail servers associated with each domain
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

The goal is to provide a comprehensive solution for managing a multi-domain server with:

**Phase 1: Command-Line Setup**
- Easy initial server configuration via a single command-line utility
- Automated installation and configuration of all required services
- Quick deployment on a fresh Ubuntu server

**Phase 2: Web-Based Administration**
- User-friendly web interface for ongoing management (adm.maindomain.com)
- Add and configure new domain names easily
- Optional webmail integration for each domain
- Complete email management: accounts, redirections, aliases
- DNS record management for all domains
- Automated best practices (DKIM, DMARC, SSL certificates)

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

## Development Guidelines

**Code Standards**
- All code comments must be written in English
- Use clear, descriptive variable and function names

**Git Commit Messages**
- Write clear, concise commit messages describing the changes
- Do not reference AI assistants or tools (e.g., Claude) in commit messages
- Focus on what changed and why, not how it was created

**Documentation Guidelines**
- Do not create multiple .md files unnecessarily
- When adding documentation, prefer updating existing files, especially README.md
- Keep documentation consolidated and maintainable
- Only create new documentation files when absolutely necessary for organization
