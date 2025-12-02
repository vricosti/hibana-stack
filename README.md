# Hibana Stack

Configure your Ubuntu server with DNS, mail, and multi-domain support in one command.

## Features

- PowerDNS for authoritative DNS management
- Mail server with DMARC, DKIM, SPF and SpamAssassin
- Traefik reverse proxy with automatic SSL (Let's Encrypt)
- Web admin interface at `adm.yourdomain.com`
- Dockerized components

## Quick Start

```bash
# Clone and prepare
git clone https://github.com/vricosti/hibana-stack.git
cd hibana-stack
./prepare-install.sh

# Generate config and install
sudo ./bin/hibana init
nano hibana-config.yaml
sudo ./bin/hibana init
```

## Configuration

Edit `hibana-config.yaml`:

```yaml
primary_domain: example.com
server_ip: YOUR_SERVER_IP

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

webadmin:
  username: admin
  password: SECURE_PASSWORD

domain_user:
  ssh_key_mode: auto  # or "manual" with ssh_public_key

# Optional: redirect other domains
# domain_redirects:
#   - from: example.fr
#     to: https://example.com
#     permanent: true
```

## What Gets Installed

| Service | URL |
|---------|-----|
| Mail server | mail.yourdomain.com |
| Webmail | webmail.yourdomain.com |
| Admin interface | adm.yourdomain.com |
| Website | www.yourdomain.com |

## Admin Interface

Access `https://adm.yourdomain.com` with your webadmin credentials.

Features:
- Domain management
- Email account management
- DNS record editor
- SSH key management for domain users

## Domain User

A restricted system user is created for deploying apps:

```bash
# Deploy files
scp -r dist/* user@server:/srv/yourdomain.com/www/src/

# Restart containers
ssh user@server
cd /srv/yourdomain.com/www
sudo docker-compose up -d --build
```

## Requirements

- Ubuntu Server 24.04 LTS
- Root access
- Domain with DNS control
- 2GB RAM, 20GB disk

## License

Apache License 2.0
