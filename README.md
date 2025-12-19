# Hibana Stack

Configure your Ubuntu server with DNS, mail, and multi-domain support in one command.

## Features

- DNS management: PowerDNS (self-hosted) or external providers (Hostinger, OVH, Cloudflare)
- Mail server with DMARC, DKIM, SPF and SpamAssassin
- Traefik reverse proxy with automatic SSL (Let's Encrypt)
- Web admin interface at `adm.yourdomain.com`
- Multi-domain support with per-domain users
- Dockerized components

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

# DNS: "local" (PowerDNS) or "external" (Hostinger, OVH, Cloudflare)
dns_provider:
  type: external
  name: hostinger
  api_token: YOUR_API_TOKEN

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
  - username: contact
    password: SECURE_PASSWORD
    full_name: Contact

webadmin:
  username: admin
  password: SECURE_PASSWORD

domain_user:
  ssh_key_mode: auto  # or "manual" with ssh_public_key
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

## Adding Domains

After initial setup, add more domains via the web admin or CLI:

```bash
# Prepare domain on server (creates user, directories, Traefik config)
sudo hibana add domain newdomain.com

# Then add via web admin at adm.yourdomain.com
```

## Domain User

A restricted system user is created for each domain. The username is derived from the domain name (dots replaced by hyphens):

| Domain | Username | Directory |
|--------|----------|-----------|
| example.com | example-com | /srv/example-com/ |

```bash
# Deploy files
scp -r dist/* example-com@server:/srv/example-com/www/src/

# Restart containers
ssh example-com@server
cd /srv/example-com/www
sudo docker-compose up -d --build
```

## Requirements

- Ubuntu Server 24.04 LTS
- Root access
- Domain with DNS control
- 2GB RAM, 20GB disk

## License

Apache License 2.0
