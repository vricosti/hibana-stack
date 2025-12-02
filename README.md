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

**⚠️ Important:** Lancez `prepare-install.sh` AVANT l'installation pour builder l'API et le frontend !

```bash
# 1. Clone the repository
git clone https://github.com/vricosti/hibana-stack.git
cd hibana-stack

# 2. Prepare the build (API + React frontend)
./prepare-install.sh

# 3. Generate configuration
sudo ./bin/hibana init

# 4. Edit the configuration with your domain and settings
nano hibana-config.yaml

# 5. Run the complete installation
sudo ./bin/hibana init
```

**Le script `prepare-install.sh` va :**
- Compiler l'API Go
- Vérifier si Node.js est installé
- Proposer d'installer Node.js si absent
- Builder le frontend React
- Créer tous les artefacts nécessaires

**Sans ce script**, l'installation utilisera une API placeholder basique.

See [QUICK_START.md](QUICK_START.md) for detailed step-by-step guide.

### Phase 2: Web Administration

After Phase 1 installation, the admin interface is **automatically deployed** and available at:

```
https://adm.yourdomain.com
```

**Login credentials:**
- Username: First email account from config (e.g., `admin`)
- Password: Email account password

**The admin interface is automatically:**
- Built during installation (API + React frontend)
- Deployed as a Docker container
- Accessible via Traefik with automatic SSL
- Running on port 3000 internally

**Admin interface features:**
- 📊 Dashboard with real-time statistics
- 🌐 Domain management (add/edit/delete)
- 📧 Email account management
- 🌍 DNS record editor (A, AAAA, CNAME, MX, TXT, etc.)
- 👤 Automatic domain user creation with SSH keys
- 🔑 SSH key management (add/remove keys for domain users)
- 🔒 JWT authentication

**Manual rebuild (if needed):**
```bash
# Rebuild API and frontend
./build-all.sh

# Restart API container
docker-compose -f /srv/yourdomain.com/api/docker-compose.yml up -d --build
```

## Architecture

```
┌─────────────────────────────────────────┐
│         Ubuntu Server                   │
│  ┌────────────┐  ┌──────────────────┐   │
│  │  PowerDNS  │  │   Mail Server    │   │
│  │            │  │  (DMARC + DKIM)  │   │
│  └────────────┘  └──────────────────┘   │
│  ┌─────────────────────────────────┐    │
│  │         Traefik Proxy           │    │
│  │  ┌───────────┐  ┌────────────┐ │     │
│  │  │  Web App  │  │  Admin UI  │ │     │
│  │  │ (Docker)  │  │  (Docker)  │ │     │
│  │  └───────────┘  └────────────┘ │     │
│  └─────────────────────────────────┘    │
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
```

See [config-example.yaml](config-example.yaml) for a complete example.

### Domain User Management

Hibana Stack can automatically create a dedicated system user for each domain with restricted SSH access. This user:

- **Has access only to `/srv/domainname/`** for deploying applications
- **Can restart and manage Docker containers** for the domain (limited sudo)
- **SSH key authentication only** - no password login
- **Member of `hibana-domains` group** for shared resources

**Configuration options:**

```yaml
domain_user:
  # Auto mode: Generate SSH key pair automatically
  ssh_key_mode: auto

  # Manual mode: Provide your own SSH public key
  # ssh_key_mode: manual
  # ssh_public_key: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... user@host"
```

**Auto mode** generates an ED25519 SSH key pair and displays the private key once during installation. Save it securely!

**Manual mode** requires you to provide your SSH public key in the configuration file.

The domain user can:
- Deploy applications in `/srv/domainname/`
- Run `docker ps`, `docker logs`, `docker restart` on domain containers
- Use `docker-compose` in `/srv/domainname/` directories
- **Cannot** access other parts of the system or other domains

### SSH Key Management

After initial setup, you can manage SSH keys for your domain users via the admin interface:

**Via Web Interface (https://adm.yourdomain.com):**
1. Navigate to **Domains**
2. Click **🔑 SSH Keys** for your domain
3. Add, view, or remove SSH keys
4. Each key can have a label for easy identification

**Via SSH:**
- Domain user home directory: `/srv/domainname/`
- SSH keys location: `/srv/domainname/.ssh/authorized_keys`
- Connect: `ssh username@server-ip`

**Benefits:**
- ✅ Multiple keys per domain (team access)
- ✅ Easy key rotation without reinstallation
- ✅ Key fingerprints for verification
- ✅ Labels to identify which device/person uses each key
- ✅ Revoke access instantly by removing a key

**Deploying Your Applications:**
```bash
# Upload files with SCP
scp -r dist/* username@server:/srv/domainname/www/src/

# Or with rsync (faster for updates)
rsync -avz --delete dist/ username@server:/srv/domainname/www/src/

# Rebuild and restart containers
ssh username@server
cd /srv/domainname/www
sudo docker-compose up -d --build
```

## Project Status

**Phase 1: ✅ COMPLETED**
- Core infrastructure setup
- Mail server with DKIM/DMARC
- DNS server (PowerDNS)
- Traefik reverse proxy
- Docker containers
- Webmail (Roundcube)
- Domain user management with SSH key authentication

**Phase 2: ✅ COMPLETED**
- ✅ REST API backend (Go)
- ✅ JWT authentication
- ✅ Domain management API (CRUD operations)
- ✅ Email account management API
- ✅ DNS record management API
- ✅ React-based admin interface
- ✅ Responsive UI with modern design
- ✅ Real-time statistics dashboard

## Requirements

- Ubuntu Server (24.04 LTS recommended)
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
