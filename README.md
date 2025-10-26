# Hibana Stack

> Configure your Ubuntu server with DNS, mail, and multi-domain support in one command

## Overview

Hibana Stack is an open-source Go utility that automates the complete setup of an Ubuntu server with:

- **PowerDNS** for authoritative DNS management
- **Mail server** with DMARC and DKIM configuration
- **Traefik** reverse proxy for multi-domain hosting with automatic SSL
- **Web admin interface** for easy domain and mail management

## Features

- 🚀 One-command server initialization
- 🌐 Multi-domain support out of the box
- 📧 Complete mail server setup with security best practices and anti-spam filtering
- 🔒 Automatic SSL certificate management via Traefik
- 🎛️ Web-based administration interface (adm.primarydomain.com)
- 🐳 Dockerized components for easy deployment

## Quick Start

```bash
# Clone the repository
git clone https://github.com/vricosti/hibana-stack.git
cd hibana-stack

# Build the binary
make build

# Generate configuration skeleton
sudo ./bin/hibana init --config hibana-config.yaml

# Edit the configuration with your domain and settings
nano hibana-config.yaml

# Run the installer
sudo ./bin/hibana init --config hibana-config.yaml
```

See [INSTALL.md](INSTALL.md) for detailed installation instructions.

## Architecture

```
┌─────────────────────────────────────────┐
│         Ubuntu Server                    │
│  ┌────────────┐  ┌──────────────────┐  │
│  │  PowerDNS  │  │   Mail Server    │  │
│  │            │  │  (DMARC + DKIM)  │  │
│  └────────────┘  └──────────────────┘  │
│  ┌─────────────────────────────────┐   │
│  │         Traefik Proxy           │   │
│  │  ┌───────────┐  ┌────────────┐ │   │
│  │  │  Web App  │  │  Admin UI  │ │   │
│  │  │ (Docker)  │  │  (Docker)  │ │   │
│  │  └───────────┘  └────────────┘ │   │
│  └─────────────────────────────────┘   │
└─────────────────────────────────────────┘
```

## What Gets Installed

After successful installation, you'll have:

- ✅ **mail.primarydomain.com** - Fully configured mail server (Postfix + Dovecot)
- ✅ **webmail.primarydomain.com** - Roundcube webmail interface
- ✅ **adm.primarydomain.com** - Admin interface (placeholder for Phase 2)
- ✅ **www.primarydomain.com** - Website (Hello World example)
- ✅ **PowerDNS** - Authoritative DNS server with all required records
- ✅ **DKIM/DMARC/SPF** - Complete email authentication
- ✅ **SpamAssassin** - Anti-spam filtering with automatic learning
- ✅ **SSL Certificates** - Automatic Let's Encrypt certificates via Traefik

## Configuration

The `hibana-config.yaml` file contains all settings:

```yaml
primary_domain: primarydomain.com
server_ip: YOUR_SERVER_IP
# system_users:  # Optional - Uncomment to create system users
#   - username: devuser
#     password: SECURE_PASSWORD
#     name: Developer
#     sudoers: false
#     ssh_pub_key: ""  # Empty = password auth only
subdomains:
  - name: adm
    role: webadmin
  - name: mail
    role: mailserver
  - name: webmail
    role: webmail
  - name: www
    role: website
email_accounts:
  - username: admin
    password: SECURE_PASSWORD
    full_name: Administrator
test_email: your-email@example.com
```

See [config-example.yaml](config-example.yaml) for a complete example.

## Project Status

**Phase 1: ✅ COMPLETED**
- Core infrastructure setup
- Mail server with DKIM/DMARC
- DNS server (PowerDNS)
- Traefik reverse proxy
- Docker containers
- Webmail (Roundcube)

**Phase 2: 📋 PLANNED**
- Web-based management interface
- Multi-domain management UI
- Email account management through web UI
- DNS record editor
- Monitoring dashboard

## Requirements

- Ubuntu Server 24.04 LTS
- Root or sudo access
- Domain name with DNS control
- At least 2GB RAM
- 20GB disk space

## Documentation

- 📘 [INSTALL.md](INSTALL.md) - Complete installation guide with troubleshooting
- 📋 [QUICKREF.md](QUICKREF.md) - Quick reference for commands and configs
- 📊 [SUMMARY.md](SUMMARY.md) - Phase 1 implementation summary
- 🏗️ [CLAUDE.md](CLAUDE.md) - Project context and architecture
- 🗺️ [PROMPT.md](PROMPT.md) - Development roadmap and specifications
- ⚙️ [config-example.yaml](config-example.yaml) - Example configuration file

## License

Apache License 2.0 - see [LICENSE](LICENSE) file for details

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

## Support

For issues and questions:
- Open an issue on GitHub
- Check [INSTALL.md](INSTALL.md) for troubleshooting
- Review [QUICKREF.md](QUICKREF.md) for common commands

## Roadmap

**✅ Phase 1 - COMPLETED**
- Core infrastructure automation
- CLI tool with YAML configuration
- Full mail server with DKIM/DMARC/SPF + SpamAssassin
- DNS server (PowerDNS)
- Reverse proxy (Traefik)
- Docker containers

**📋 Phase 2 - PLANNED**
- Web-based management interface
- Multi-domain support via UI
- Email account management
- DNS record editor
- Monitoring and statistics

See [PROMPT.md](PROMPT.md) for detailed roadmap.
