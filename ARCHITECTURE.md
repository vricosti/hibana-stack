# Hibana Stack Architecture

## Overview

Hibana Stack uses a **microservices architecture** with each service isolated in its own directory and Docker Compose configuration. Services communicate via a shared Traefik reverse proxy network that handles routing, SSL termination, and security.

## Architecture Principles

1. **Service Isolation**: Each service is self-contained in `/srv/[service]/`
2. **Configuration as Code**: All configuration in version-controllable files
3. **Declarative Infrastructure**: Docker Compose for reproducible deployments
4. **Security by Default**: HTTPS everywhere via Traefik + Let's Encrypt
5. **Scalability**: Services can be scaled, moved, or replaced independently

## Directory Structure

```
/srv/
├── traefik/                    # Reverse proxy & SSL termination
│   ├── docker-compose.yml
│   ├── traefik.yml            # Static configuration
│   ├── dynamic/
│   │   └── middlewares.yml    # Security headers, rate limiting
│   └── acme.json              # Let's Encrypt certificates
│
├── www.vridev.com/            # Main website
│   ├── docker-compose.yml
│   ├── src/                   # React application source
│   ├── nginx/                 # Web server configuration
│   └── logs/
│
├── adm.vridev.com/            # Administration interface
│   ├── docker-compose.yml
│   ├── app/                   # Admin UI application
│   ├── data/
│   └── logs/
│
├── webmail.vridev.com/        # Roundcube webmail
│   ├── docker-compose.yml
│   ├── .env                   # Service configuration
│   ├── data/                  # SQLite database
│   └── logs/
│
└── api.vridev.com/            # REST API backend
    ├── docker-compose.yml
    ├── app/                   # Go API source code
    ├── secrets/               # Credentials & secrets
    ├── data/
    └── logs/
```

### Why `/srv/` Instead of Alternatives?

| Location | Pros | Cons | Decision |
|----------|------|------|----------|
| `/etc/hibana/docker/` | Single config location | Monolithic, not standard for web apps | ❌ Rejected |
| `/var/www/` | Traditional web server location | Mixed concerns (static + dynamic apps) | ❌ Rejected |
| `/srv/` | FHS standard for "site-specific data" | Requires sudo for initial setup | ✅ **Selected** |

**Rationale for `/srv/`:**
- Follows Filesystem Hierarchy Standard (FHS) for service data
- Clear separation: one directory per service
- Easy to understand structure for operations and maintenance
- Standard practice for containerized multi-service deployments
- Clean backup/restore strategy (one directory per service)

## Service Architecture

### 1. Traefik (Reverse Proxy)

**Role:** Central entry point for all HTTP/HTTPS traffic

**Features:**
- Automatic service discovery via Docker labels
- SSL certificate management (Let's Encrypt)
- HTTP to HTTPS redirect
- Security headers injection
- Rate limiting
- Request routing based on hostname

**Network:** `traefik-network` (bridge network shared with all services)

**Key Configuration:**
```yaml
# Services connect by adding these labels:
labels:
  - "traefik.enable=true"
  - "traefik.http.routers.[name].rule=Host(`example.com`)"
  - "traefik.http.routers.[name].tls.certresolver=letsencrypt"
```

### 2. www.vridev.com (Main Website)

**Stack:** React + nginx

**Build Process:**
1. Multi-stage Docker build compiles React app
2. Production build served by nginx
3. Optimized for SPA (single-page application)

**Features:**
- Static asset caching (1 year for hashed files)
- Gzip compression
- Security headers
- Client-side routing support (React Router)

### 3. adm.vridev.com (Admin Interface)

**Status:** Phase 1 placeholder

**Purpose:** Web-based administration for:
- Domain management
- Email account administration
- DNS record editing
- SSL certificate monitoring
- Server statistics

**Future Stack (Phase 2):**
- Frontend: React + Admin UI framework
- Backend: Connects to api.vridev.com
- Auth: JWT-based authentication

### 4. webmail.vridev.com (Roundcube)

**Stack:** Roundcube (PHP/Apache) + SQLite

**Features:**
- Connects to host mail server via `mail.vridev.com`
- IMAP on port 143 (STARTTLS)
- SMTP on port 587 (STARTTLS)
- SQLite for simplicity (can migrate to PostgreSQL if needed)

**Host Connection:**
```yaml
extra_hosts:
  - "mail.vridev.com:host-gateway"
```
This maps mail.vridev.com to the Docker host IP.

### 5. api.vridev.com (REST API)

**Stack:** Go 1.21 + PostgreSQL

**Status:** Phase 1 placeholder with health check

**Future Features (Phase 2):**
- RESTful API for all admin operations
- JWT authentication
- Database connection to host PostgreSQL (hibana database)
- Comprehensive endpoint coverage for domain/email/DNS management

**Security:**
- Secrets managed via Docker secrets
- Rate limiting via Traefik
- CORS configuration
- Database credentials isolated in secrets files

## Network Architecture

```
                   Internet
                      │
                      ▼
              ┌───────────────┐
              │   Traefik     │  :80, :443
              │  (Container)  │
              └───────┬───────┘
                      │
          ┌───────────┴───────────┬─────────────┐
          │                       │             │
    ┌─────▼──────┐         ┌─────▼─────┐  ┌───▼────┐
    │    www     │         │  webmail  │  │  api   │
    │ Container  │         │ Container │  │Container│
    └────────────┘         └─────┬─────┘  └───┬────┘
                                 │            │
                                 │            │
                           ┌─────▼────────────▼─────┐
                           │   Host Services        │
                           │  - Postfix/Dovecot     │
                           │  - PostgreSQL          │
                           │  - PowerDNS            │
                           └────────────────────────┘
```

### Networks

1. **traefik-network** (external bridge network)
   - All web services connect to this network
   - Traefik discovers and routes to services on this network
   - Created once: `docker network create traefik-network`

2. **host-gateway connections**
   - Containers access host services via `host.docker.internal`
   - Configured per service with `extra_hosts`

## Deployment Workflow

### Initial Setup (One-time)

```bash
# 1. Create Traefik network
docker network create traefik-network

# 2. Start Traefik first
cd /srv/traefik
docker-compose up -d

# 3. Deploy each service
cd /srv/www.vridev.com && docker-compose up -d
cd /srv/adm.vridev.com && docker-compose up -d
cd /srv/webmail.vridev.com && docker-compose up -d
cd /srv/api.vridev.com && docker-compose up -d
```

### Updates & Maintenance

```bash
# Update a single service (zero-downtime if load-balanced)
cd /srv/[service]
git pull                    # Update code
docker-compose build        # Rebuild image
docker-compose up -d        # Recreate container

# View logs
docker-compose logs -f

# Restart service
docker-compose restart

# Stop service
docker-compose down
```

### Backup Strategy

```bash
# Each service is self-contained - backup the entire directory
tar -czf www-backup.tar.gz /srv/www.vridev.com
tar -czf webmail-backup.tar.gz /srv/webmail.vridev.com
tar -czf api-backup.tar.gz /srv/api.vridev.com
# Exclude node_modules, build artifacts if needed

# Traefik certificates
tar -czf traefik-certs.tar.gz /srv/traefik/acme.json
```

## Service Communication

### External (Internet → Services)

```
User Request → Traefik (SSL) → Service Container
```

All external traffic flows through Traefik:
1. User requests `https://webmail.vridev.com`
2. Traefik matches Host header to routing rule
3. Traefik terminates SSL and forwards to webmail container
4. Response flows back through Traefik to user

### Internal (Service → Host)

Services access host resources via `host.docker.internal`:

```yaml
# In docker-compose.yml
extra_hosts:
  - "host.docker.internal:host-gateway"

# Then in application:
DATABASE_HOST=host.docker.internal
MAIL_SERVER=host.docker.internal
```

### Internal (Service → Service)

Services can communicate via Docker network:

```bash
# From api container
curl http://webmail-vridev/health
```

Container names become DNS names within the `traefik-network`.

## Security

### SSL/TLS

- Automatic certificate issuance via Let's Encrypt
- Certificates stored in `/srv/traefik/acme.json`
- Auto-renewal handled by Traefik
- HTTP → HTTPS redirect enforced

### Security Headers

Applied to all services via Traefik middleware:

```yaml
# /srv/traefik/dynamic/middlewares.yml
http:
  middlewares:
    security-headers:
      headers:
        customResponseHeaders:
          X-Frame-Options: "SAMEORIGIN"
          X-Content-Type-Options: "nosniff"
          X-XSS-Protection: "1; mode=block"
          Referrer-Policy: "strict-origin-when-cross-origin"
```

### Secrets Management

Sensitive data stored in secrets files:

```bash
/srv/api.vridev.com/secrets/
├── db_password          # Never committed to git
├── jwt_secret           # Generated with openssl rand
└── .gitignore          # Excludes all except .example files
```

**Best Practices:**
- Use Docker secrets for production
- Rotate secrets regularly
- Never commit secrets to git
- Use `.example` files as templates

### Network Isolation

- Web services: Only exposed via Traefik (no direct port binding)
- Host services: Only accessible via host-gateway
- No service-to-service traffic except through defined networks

## Scaling Considerations

### Horizontal Scaling

For high-traffic services, scale containers:

```yaml
# docker-compose.yml
services:
  api:
    deploy:
      replicas: 3
```

Traefik automatically load-balances across replicas.

### Vertical Scaling

Resource limits per service:

```yaml
services:
  api:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
```

### Multi-Server Deployment

Current architecture supports migration to:
- Docker Swarm (built-in orchestration)
- Kubernetes (using same container images)
- Separate servers per service (with Traefik on gateway server)

## Monitoring & Observability

### Logs

All services log to:
- `docker-compose logs` (ephemeral)
- `/srv/[service]/logs/` (persistent)

Aggregate logs using:
- ELK Stack (Elasticsearch, Logstash, Kibana)
- Grafana Loki
- CloudWatch / Datadog

### Metrics

Traefik exposes Prometheus metrics:

```yaml
# traefik.yml
metrics:
  prometheus:
    entryPoint: metrics
```

Monitor:
- Request rates
- Error rates
- Response times
- Certificate expiry

### Health Checks

Each service should implement:
- `GET /health` endpoint
- Docker healthcheck directive
- Automated monitoring alerts

## Migration from Old Architecture

### Old Structure (Deprecated)

```
/etc/hibana/docker/
└── docker-compose.yml      # Monolithic file with all services
```

**Problems:**
- All services in one compose file = tight coupling
- Can't update one service without affecting others
- Doesn't scale well
- Not standard filesystem location for web services

### New Structure (Current)

```
/srv/[service]/
├── docker-compose.yml      # One per service
└── [service-specific files]
```

**Benefits:**
- Service independence
- Standard filesystem location
- Clear ownership and boundaries
- Easy backup/restore per service
- Simpler CI/CD pipelines

### Migration Steps

1. Create new `/srv/` structure with all services
2. Update DNS if needed (should already point correctly)
3. Start new services: `cd /srv/[service] && docker-compose up -d`
4. Verify each service is accessible
5. Stop old monolithic stack: `cd /etc/hibana/docker && docker-compose down`
6. Monitor for issues
7. Remove old `/etc/hibana/docker/` after verification period

## Ansible Integration

The Hibana Stack installer (Ansible roles) should:

1. **Generate service directories**
   ```yaml
   # ansible/roles/docker/tasks/main.yml
   - name: Create service directories
     file:
       path: "/srv/{{ item }}"
       state: directory
     loop:
       - traefik
       - www.{{ domain }}
       - adm.{{ domain }}
       - webmail.{{ domain }}
       - api.{{ domain }}
   ```

2. **Template docker-compose files**
   ```yaml
   - name: Deploy docker-compose files
     template:
       src: "{{ item }}/docker-compose.yml.j2"
       dest: "/srv/{{ item }}/docker-compose.yml"
   ```

3. **Start services in order**
   ```yaml
   - name: Start Traefik first
     docker_compose:
       project_src: /srv/traefik
       state: present

   - name: Start web services
     docker_compose:
       project_src: "/srv/{{ item }}"
       state: present
     loop: "{{ web_services }}"
   ```

## Troubleshooting

### Service not accessible

```bash
# Check if container is running
docker ps | grep [service-name]

# Check Traefik routing
docker logs traefik | grep [hostname]

# Verify DNS
dig [hostname]

# Check SSL certificate
curl -vI https://[hostname]
```

### Container can't reach host services

```bash
# Inside container, test host connectivity
docker exec [container] ping host.docker.internal

# Check if host service is listening
sudo ss -tlnp | grep [port]

# Verify extra_hosts configuration
docker inspect [container] | grep -A5 ExtraHosts
```

### SSL certificate issues

```bash
# Check certificate status
docker logs traefik | grep -i certificate

# View acme.json
cat /srv/traefik/acme.json | jq

# Force certificate renewal (remove domain from acme.json)
# Traefik will re-request on next startup
```

## Best Practices

1. **Always use `.env` files** for configuration (never hardcode)
2. **Tag Docker images** with versions (not just `:latest`)
3. **Implement health checks** in all services
4. **Use Docker secrets** for sensitive data in production
5. **Keep services small** and focused (single responsibility)
6. **Document environment variables** in README.md
7. **Version control everything** except secrets and data
8. **Test locally** before deploying to production
9. **Monitor resource usage** and set limits
10. **Backup data directories** regularly

## Future Enhancements

### Phase 2: Full Implementation

- Complete admin UI (React dashboard)
- Full REST API implementation
- JWT authentication system
- Real-time WebSocket updates
- Comprehensive API documentation (OpenAPI/Swagger)

### Infrastructure Improvements

- [ ] Prometheus + Grafana monitoring
- [ ] Centralized logging (ELK or Loki)
- [ ] Automated backups with retention policies
- [ ] CI/CD pipeline (GitHub Actions)
- [ ] Staging environment
- [ ] Database connection pooling
- [ ] Redis cache layer
- [ ] CDN integration for static assets

### Security Enhancements

- [ ] Fail2ban integration for API
- [ ] Two-factor authentication (2FA)
- [ ] API key management
- [ ] Audit logging
- [ ] Security scanning (Trivy, Clair)
- [ ] Regular security updates automation

## Conclusion

The Hibana Stack microservices architecture provides:

✅ **Isolation** - Services are independent and self-contained
✅ **Scalability** - Easy to scale individual services
✅ **Maintainability** - Clear structure and boundaries
✅ **Security** - HTTPS everywhere, secrets management
✅ **Flexibility** - Swap, upgrade, or move services independently
✅ **Standards** - Follows FHS and Docker best practices

This architecture positions Hibana Stack for growth from a single-server deployment to a distributed, high-availability system.
