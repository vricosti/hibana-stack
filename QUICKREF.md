# Hibana Stack - Quick Reference

## Installation

```bash
# 1. Clone and build
git clone https://github.com/vricosti/hibana-stack.git
cd hibana-stack
make build

# 2. Create config
sudo ./bin/hibana init --config hibana-config.json

# 3. Edit config (change domain, IP, passwords)
nano hibana-config.json

# 4. Run installer
sudo ./bin/hibana init --config hibana-config.json
```

## Configuration

```json
{
  "primary_domain": "primarydomain.com",
  "server_ip": "YOUR_SERVER_IP",
  "subdomains": ["adm", "mail", "webmail", "www"],
  "email_accounts": [
    {
      "username": "admin",
      "password": "SECURE_PASSWORD",
      "full_name": "Administrator"
    }
  ],
  "test_email": "test@example.com"
}
```

## Services

| Service | Port | Description |
|---------|------|-------------|
| PowerDNS | 53 | DNS server |
| Postfix (SMTP) | 25, 587, 465 | Mail sending |
| Dovecot (IMAP) | 143, 993 | Mail receiving |
| SpamAssassin | - | Anti-spam filter |
| Traefik | 80, 443 | Reverse proxy |
| PostgreSQL | 5432 | Database |

## URLs

- `https://www.primarydomain.com` - Website
- `https://webmail.primarydomain.com` - Webmail (Roundcube)
- `https://adm.primarydomain.com` - Admin (placeholder)
- `mail.primarydomain.com` - Mail server

## Commands

### Check Services

```bash
# All services
sudo systemctl status postgresql pdns postfix dovecot opendkim spamassassin

# Docker containers
docker ps

# Mail queue
mailq
```

### Check Logs

```bash
# Mail logs
sudo tail -f /var/log/mail.log

# PowerDNS
sudo journalctl -u pdns -f

# Docker
docker logs hibana-traefik
docker logs hibana-webmail
```

### Test Email

```bash
# Send test
echo "Test" | mail -s "Subject" test@example.com

# Check queue
mailq

# Check ports
sudo netstat -tulpn | grep -E ':(25|587|143|993|53|80|443)'
```

### DNS Queries

```bash
# Test local DNS
dig @localhost primarydomain.com
dig @localhost MX primarydomain.com
dig @localhost TXT default._domainkey.primarydomain.com
```

### Database Access

```bash
# Hibana database
sudo -u postgres psql hibana

# PowerDNS database
sudo -u postgres psql powerdns

# View domains
sudo -u postgres psql powerdns -c "SELECT * FROM domains;"

# View DNS records
sudo -u postgres psql powerdns -c "SELECT * FROM records WHERE domain_id=1;"
```

## File Locations

```
/etc/hibana/              # Main config directory
  passwords/              # Encrypted passwords
  traefik/               # Traefik config
  docker/                # Docker configs

/etc/postfix/            # Postfix config
/etc/dovecot/            # Dovecot config
/etc/opendkim/           # OpenDKIM config
/etc/powerdns/           # PowerDNS config

/var/mail/vhosts/        # Email storage
```

## Troubleshooting

### Email Not Sending

```bash
# Check Postfix
sudo systemctl status postfix
sudo tail -f /var/log/mail.log

# Check OpenDKIM
sudo systemctl status opendkim

# Test SMTP
telnet localhost 25
```

### Email Not Receiving

```bash
# Check Dovecot
sudo systemctl status dovecot

# Check mailboxes
ls -la /var/mail/vhosts/primarydomain.com/
```

### DNS Not Working

```bash
# Check PowerDNS
sudo systemctl status pdns
sudo journalctl -u pdns -n 50

# Test resolution
dig @localhost primarydomain.com
```

### Webmail Not Accessible

```bash
# Check Traefik
docker logs hibana-traefik

# Check Roundcube
docker logs hibana-webmail

# Check network
docker network inspect hibana-network
```

## Firewall Setup

```bash
sudo ufw allow 22/tcp   # SSH
sudo ufw allow 25/tcp   # SMTP
sudo ufw allow 80/tcp   # HTTP
sudo ufw allow 443/tcp  # HTTPS
sudo ufw allow 587/tcp  # Submission
sudo ufw allow 143/tcp  # IMAP
sudo ufw allow 993/tcp  # IMAPS
sudo ufw allow 53/tcp   # DNS
sudo ufw allow 53/udp   # DNS
sudo ufw enable
```

## Security

```bash
# Install fail2ban
sudo apt install fail2ban
sudo systemctl enable fail2ban

# Update system
sudo apt update && sudo apt upgrade -y

# Check passwords location
ls -la /etc/hibana/passwords/
```

## Maintenance

```bash
# Restart all services
sudo systemctl restart postgresql pdns postfix dovecot opendkim spamassassin
docker-compose -f /etc/hibana/docker/docker-compose.yml restart

# Update containers
cd /etc/hibana/docker
docker-compose pull
docker-compose up -d

# Clean mail queue
sudo postsuper -d ALL

# Backup databases
sudo -u postgres pg_dump hibana > hibana-backup.sql
sudo -u postgres pg_dump powerdns > powerdns-backup.sql

# Backup email
sudo tar -czf mail-backup.tar.gz /var/mail/vhosts/
```

## Getting Help

1. Check logs for errors
2. Verify DNS records are propagated
3. Test each service individually
4. Check firewall rules
5. Review `/var/log/mail.log` for email issues
6. Use `dig` and `telnet` for connectivity tests

## Phase 2 Features (Coming Soon)

- Web-based domain management
- Email account management UI
- DNS record editor
- Monitoring dashboard
- Multi-domain support via UI
