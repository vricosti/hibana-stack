# Hibana Stack Installation Guide

## Prerequisites

- Fresh Ubuntu 24.04 LTS server
- Root or sudo access
- Domain name with DNS control
- At least 2GB RAM
- 20GB disk space

## Installation Steps

### 1. Clone the Repository

```bash
git clone https://github.com/vricosti/hibana-stack.git
cd hibana-stack
```

### 2. Build the Binary

```bash
# Install Go if not already installed
sudo apt update
sudo apt install -y golang-go

# Build hibana
make build

# Or install system-wide
make install
```

### 3. Create Configuration File

The first time you run `hibana init`, it will create a configuration skeleton:

```bash
sudo ./bin/hibana init --config ./hibana-config.yaml
```

This creates `hibana-config.yaml` with this structure:

```yaml
primary_domain: primarydomain.com
server_ip: YOUR_SERVER_IP
# system_users:  # Optional - Uncomment to create system users
#   - username: devuser
#     password: SECURE_PASSWORD
#     name: Developer
#     sudoers: false
#     ssh_pub_key: ""
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
    password: CHANGE_THIS_PASSWORD
    full_name: Administrator
test_email: your-email@example.com
```

**Note**: The `system_users` section is optional. See `config-example.yaml` for an example of how to add system users.

### 4. Edit Configuration

Edit `hibana-config.yaml` with your actual values:

```bash
nano hibana-config.yaml
```

**Important fields:**
- `primary_domain`: Your domain name (e.g., `primarydomain.com`)
- `server_ip`: Your server's public IP address
- `subdomains`: List of subdomains with their roles
  - `name`: Subdomain name
  - `role`: Subdomain role (webadmin, mailserver, webmail, website)
- `system_users`: (Optional) List of system users to create
  - `username`: Linux username
  - `password`: User password
  - `name`: Full name
  - `sudoers`: `true` to grant sudo access, `false` otherwise
  - `ssh_pub_key`: SSH public key for key-based authentication (leave empty for password-only)
- `email_accounts`: List of email accounts to create
  - `username`: Email username (will become username@primarydomain.com)
  - `password`: Strong password for the email account
- `test_email`: External email to test mail delivery

**Example configuration:**

```yaml
primary_domain: primarydomain.com
server_ip: 203.0.113.10
# system_users:  # Optional - Uncomment to create system users
#   - username: devuser
#     password: SecureP@ssw0rd456!
#     name: Developer Account
#     sudoers: false
#     ssh_pub_key: ""  # Leave empty for password-only auth
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
    password: SecureP@ssw0rd123!
    full_name: System Administrator
  - username: contact
    password: AnotherSecureP@ss!
    full_name: Contact
test_email: your-personal-email@gmail.com
```

### 5. Run Installation

```bash
sudo ./bin/hibana init --config ./hibana-config.yaml
```

The installer will:
1. ✓ Check Ubuntu 24.04
2. ✓ Install required packages
3. ✓ Setup PostgreSQL databases
4. ✓ Configure PowerDNS
5. ✓ Setup mail server (Postfix/Dovecot)
6. ✓ Configure DKIM/DMARC
7. ✓ Add DNS records
8. ✓ Setup Traefik reverse proxy
9. ✓ Deploy Docker containers
10. ✓ Test email functionality

### 6. Configure DNS

After installation, you need to configure your domain's nameservers to point to your server.

**Option A: Use your domain registrar's DNS**

Add these records in your domain registrar's DNS panel:

```
Type    Name                    Value                           TTL
A       @                       YOUR_SERVER_IP                  300
A       mail                    YOUR_SERVER_IP                  3600
A       webmail                 YOUR_SERVER_IP                  300
A       adm                     YOUR_SERVER_IP                  300
A       www                     YOUR_SERVER_IP                  300
MX      @                       mail.primarydomain.com             14400
TXT     @                       v=spf1 ip4:YOUR_SERVER_IP -all  14400
TXT     _dmarc                  v=DMARC1; p=none; rua=mailto... 3600
TXT     default._domainkey      v=DKIM1; h=sha256; k=rsa; p=... 14400
```

**Option B: Use PowerDNS as authoritative nameserver**

Update your domain's nameservers at your registrar to point to:
- `ns1.primarydomain.com` (YOUR_SERVER_IP)

Then add glue records for `ns1.primarydomain.com` pointing to YOUR_SERVER_IP.

### 7. Verify Installation

After DNS propagation (usually 5-60 minutes), verify your services:

```bash
# Check web services
curl https://www.primarydomain.com
curl https://adm.primarydomain.com
curl https://webmail.primarydomain.com

# Check mail server
telnet mail.primarydomain.com 25
telnet mail.primarydomain.com 587

# Check DNS
dig primarydomain.com
dig MX primarydomain.com
dig TXT default._domainkey.primarydomain.com
```

### 8. Access Your Services

- **Website**: https://www.primarydomain.com
- **Admin Panel**: https://adm.primarydomain.com (Phase 2)
- **Webmail**: https://webmail.primarydomain.com
- **Email**: username@primarydomain.com

**Webmail Login:**
- Server: mail.primarydomain.com
- Username: username@primarydomain.com
- Password: (password from config)

## Re-running the Installer

Hibana Stack is idempotent. You can safely re-run the installer:

```bash
sudo ./bin/hibana init --config ./hibana-config.yaml
```

It will:
- Skip already installed packages
- Update configurations
- Add new email accounts
- Update DNS records
- Restart services as needed

## Troubleshooting

### Check Service Status

```bash
# PostgreSQL
sudo systemctl status postgresql

# PowerDNS
sudo systemctl status pdns

# Postfix
sudo systemctl status postfix

# Dovecot
sudo systemctl status dovecot

# OpenDKIM
sudo systemctl status opendkim

# Docker containers
docker ps
```

### Check Logs

```bash
# Postfix logs
sudo tail -f /var/log/mail.log

# PowerDNS logs
sudo journalctl -u pdns -f

# Docker containers
docker logs hibana-traefik
docker logs hibana-webmail
docker logs hibana-www
docker logs hibana-adm
```

### Test Email Sending

```bash
# Send test email
echo "Test email body" | mail -s "Test Subject" test@example.com

# Check mail queue
mailq

# Check if ports are listening
sudo netstat -tulpn | grep -E ':(25|587|143|993|110|995|53|80|443)'
```

### DNS Issues

```bash
# Test local DNS resolution
dig @localhost primarydomain.com

# Check PowerDNS database
sudo -u postgres psql powerdns -c "SELECT * FROM domains;"
sudo -u postgres psql powerdns -c "SELECT * FROM records WHERE domain_id=1;"
```

## Security Recommendations

1. **Change default passwords** in the configuration file
2. **Setup firewall** (ufw):
   ```bash
   sudo ufw allow 22/tcp
   sudo ufw allow 25/tcp
   sudo ufw allow 80/tcp
   sudo ufw allow 443/tcp
   sudo ufw allow 587/tcp
   sudo ufw allow 143/tcp
   sudo ufw allow 993/tcp
   sudo ufw allow 53/tcp
   sudo ufw allow 53/udp
   sudo ufw enable
   ```
3. **Enable fail2ban**:
   ```bash
   sudo apt install fail2ban
   sudo systemctl enable fail2ban
   ```
4. **Keep system updated**:
   ```bash
   sudo apt update && sudo apt upgrade -y
   ```

## Next Steps

- Monitor your mail server reputation
- Configure spam filtering (SpamAssassin)
- Setup backups for `/var/mail` and PostgreSQL databases
- Wait for Phase 2 for web-based domain management
