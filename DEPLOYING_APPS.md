# Deploying Web Applications with Hibana Stack

This guide explains how to deploy web applications (React, Vue, static sites, etc.) on your Hibana Stack server.

## Architecture Overview

Each domain has its own directory structure in `/srv/domainname/`:

```
/srv/domainname/
├── .ssh/                   # SSH keys for domain user
│   └── authorized_keys
├── www/                    # Website (www.domainname)
│   ├── src/                # Your application files
│   ├── nginx/
│   ├── docker-compose.yml
│   └── logs/
├── api/                    # API backend (adm.domainname)
├── webmail/                # Webmail (webmail.domainname)
└── logs/                   # Various logs
```

## Domain User Access

When you create a domain with domain user enabled, Hibana Stack creates:
- **Username**: Based on domain name (e.g., `example_com` for example.com)
- **Home directory**: `/srv/domainname/`
- **SSH access**: Key-based authentication only
- **Permissions**: Full access to `/srv/domainname/`, restricted elsewhere
- **Docker access**: Can manage domain containers via limited sudo

## Managing SSH Keys

### Via Admin Interface (Recommended)

1. Access **https://adm.yourdomain.com**
2. Navigate to **Domains**
3. Click **🔑 SSH Keys** for your domain
4. Click **Add SSH Key**
5. Paste your public key (from `~/.ssh/id_rsa.pub` or `~/.ssh/id_ed25519.pub`)
6. Add a label (e.g., "My Laptop", "Work Desktop")

**Benefits:**
- Add multiple keys for team members
- Label keys to track who has access
- Remove keys instantly to revoke access
- View key fingerprints for verification

### Generate SSH Key (if you don't have one)

```bash
# Generate ED25519 key (recommended)
ssh-keygen -t ed25519 -C "your-email@example.com"

# Or RSA key (legacy)
ssh-keygen -t rsa -b 4096 -C "your-email@example.com"

# View your public key
cat ~/.ssh/id_ed25519.pub
```

## Deploying a React Application

### Method 1: Direct Upload (Simple & Fast)

**1. Build your React app locally:**
```bash
cd my-react-app
npm run build
# Creates dist/ or build/ directory
```

**2. Upload files via SCP:**
```bash
scp -r dist/* domainuser@server-ip:/srv/domainname/www/src/
```

**3. Create/Update Dockerfile** in `/srv/domainname/www/src/Dockerfile`:
```dockerfile
FROM nginx:alpine

# Copy built React app
COPY . /usr/share/nginx/html

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

**4. Restart the container:**
```bash
ssh domainuser@server-ip
cd /srv/domainname/www
sudo docker-compose up -d --build
```

Your site is now live at **https://www.domainname/** and **https://domainname/**

### Method 2: Git + Docker Multi-Stage (Production-Ready)

**1. Prepare your repository:**
```
my-react-app/
├── src/
├── public/
├── package.json
├── Dockerfile           # Multi-stage build
└── .dockerignore
```

**2. Create multi-stage Dockerfile:**
```dockerfile
# Stage 1: Build
FROM node:18-alpine AS builder
WORKDIR /app

# Install dependencies
COPY package*.json ./
RUN npm ci --only=production

# Build app
COPY . .
RUN npm run build

# Stage 2: Serve with nginx
FROM nginx:alpine

# Copy built app from builder stage
COPY --from=builder /app/dist /usr/share/nginx/html

# Optional: Custom nginx config
# COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

**3. Deploy:**
```bash
# Clone or pull your repo
ssh domainuser@server-ip
cd /srv/domainname/www/src
git clone https://github.com/user/my-app.git .

# Or update
git pull

# Build and start
cd /srv/domainname/www
sudo docker-compose up -d --build
```

**Benefits:**
- Everything builds in Docker (no local Node.js needed on server)
- Consistent builds
- Easy updates with `git pull`
- Multi-stage keeps final image small

### Method 3: Automated Deployment with rsync

Create a deployment script locally:

```bash
#!/bin/bash
# deploy.sh

DOMAIN="example.com"
USER="example_com"
SERVER="your-server-ip"

echo "Building React app..."
npm run build

echo "Uploading to server..."
rsync -avz --delete \
  --exclude 'node_modules' \
  --exclude '.git' \
  dist/ ${USER}@${SERVER}:/srv/${DOMAIN}/www/src/

echo "Restarting container..."
ssh ${USER}@${SERVER} "cd /srv/${DOMAIN}/www && sudo docker-compose up -d --build"

echo "✅ Deployment complete!"
echo "Visit: https://www.${DOMAIN}"
```

Make it executable:
```bash
chmod +x deploy.sh
```

Deploy with:
```bash
./deploy.sh
```

## Deploying Other Frameworks

### Vue.js

Same as React, but build command may differ:
```bash
npm run build  # Creates dist/
```

### Static HTML/CSS/JS

No build step needed:
```bash
rsync -avz --delete \
  your-static-site/ \
  domainuser@server:/srv/domainname/www/src/
```

### Next.js / Nuxt

Use framework-specific Docker images:

**Next.js Dockerfile:**
```dockerfile
FROM node:18-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:18-alpine
WORKDIR /app
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/public ./public
COPY --from=builder /app/package*.json ./
RUN npm ci --only=production

EXPOSE 3000
CMD ["npm", "start"]
```

Update `docker-compose.yml` to expose port 3000 instead of 80.

## Custom Nginx Configuration

The default nginx config supports SPAs (single-page applications). To customize:

**1. Create custom config** in `/srv/domainname/www/nginx/default.conf`:
```nginx
server {
    listen 80;
    server_name _;

    root /usr/share/nginx/html;
    index index.html;

    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript
               application/javascript application/xml+rss
               application/json;

    # Cache static assets
    location ~* \.(jpg|jpeg|png|gif|ico|css|js|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    # SPA fallback
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
}
```

**2. Mount it in docker-compose.yml** (already configured by default):
```yaml
volumes:
  - ./nginx/default.conf:/etc/nginx/conf.d/default.conf:ro
```

## Environment Variables

If your app needs environment variables:

**1. Add them to `.env` file** in `/srv/domainname/www/`:
```env
REACT_APP_API_URL=https://api.example.com
REACT_APP_GA_ID=UA-XXXXX-Y
```

**2. Update docker-compose.yml:**
```yaml
services:
  web:
    env_file:
      - .env
```

**Note:** For React/Vue, environment variables must be built into the app at build time. Consider building on the server or using runtime configuration.

## Troubleshooting

### Container not starting
```bash
# Check logs
sudo docker logs www-domainname

# Check docker-compose status
cd /srv/domainname/www
sudo docker-compose ps
```

### Permission issues
```bash
# Fix ownership
sudo chown -R domainuser:hibana-domains /srv/domainname/
sudo chmod -R 755 /srv/domainname/www/src/
```

### SSL certificate not working
```bash
# Check Traefik logs
sudo docker logs traefik

# Verify DNS points to server
dig www.domainname
```

### Changes not appearing
```bash
# Force rebuild
cd /srv/domainname/www
sudo docker-compose down
sudo docker-compose up -d --build --force-recreate
```

## Security Best Practices

1. **Never commit secrets** to Git
   - Use `.env` files
   - Add `.env` to `.gitignore`

2. **Use SSH keys only**
   - Never enable password authentication
   - Rotate keys regularly
   - Use different keys for different team members

3. **Limit container resources**
   ```yaml
   services:
     web:
       deploy:
         resources:
           limits:
             cpus: '0.5'
             memory: 512M
   ```

4. **Keep dependencies updated**
   ```bash
   npm audit fix
   docker pull nginx:alpine
   ```

## Multi-Environment Setup

For staging/production environments:

```
/srv/domainname/
├── www/          # Production (www.domainname)
├── staging/      # Staging (staging.domainname)
│   ├── src/
│   ├── nginx/
│   └── docker-compose.yml
```

Add subdomain in DNS and Traefik labels in docker-compose.yml.

## Next Steps

- Configure CI/CD with GitHub Actions for automatic deployments
- Set up monitoring with Prometheus/Grafana
- Add application logging to `/srv/domainname/logs/`
- Implement automated backups

## Support

For issues:
- Check [INSTALL.md](INSTALL.md) for troubleshooting
- Review [PHASE2_DEPLOYMENT.md](PHASE2_DEPLOYMENT.md) for admin interface
- Open an issue on GitHub
