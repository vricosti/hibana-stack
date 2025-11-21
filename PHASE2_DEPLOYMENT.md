# Phase 2 - Déploiement Automatique de l'Interface d'Administration

## 🎯 Objectif

L'interface d'administration (API + Frontend React) est maintenant **automatiquement déployée** lors de l'installation initiale avec `sudo ./bin/hibana init`.

## 🚀 Fonctionnement

### Lors de l'installation (`sudo ./bin/hibana init`)

**Étape 1 : Build automatique**
```
🔨 Building API and admin interface...
  → Building Go API...
    ✓ API binary built
  → Building React admin interface...
    → Installing npm dependencies... (if needed)
    ✓ Frontend built
    ✓ Dockerfile created
    Build artifacts ready in: /tmp/ansible-workspace-xxxxx/api-build
✓ API and frontend built successfully
```

**Étape 2 : Déploiement Ansible**

Les artefacts compilés sont copiés vers `/srv/<domaine>/api/app/` :
- `hibana-api` (binaire Go)
- `web/` (frontend React)
- `Dockerfile` (pour conteneurisation)

**Étape 3 : Conteneur Docker**

Le conteneur Docker est automatiquement créé et démarré avec :
- Image Alpine Linux légère
- Binaire API pré-compilé
- Frontend React pré-buildé
- Configuration Traefik pour SSL automatique

**Étape 4 : Accès via Traefik**

Traefik route automatiquement `adm.<domaine>` vers le conteneur API.

## 📂 Structure de Déploiement

```
/srv/<domaine>/api/
├── app/
│   ├── hibana-api          # Binaire Go
│   ├── web/                # Frontend React compilé
│   │   ├── index.html
│   │   ├── assets/
│   │   └── ...
│   └── Dockerfile
├── docker-compose.yml      # Configuration conteneur
├── secrets/
│   ├── jwt_secret          # Clé JWT auto-générée
│   └── db_password         # Mot de passe DB
├── logs/                   # Logs API
└── data/                   # Données persistantes
```

## 🌐 Accès à l'Interface

### URL

```
https://adm.<votre-domaine>
```

Exemples :
- `https://adm.example.com`
- `https://adm.vridev.com`

### Authentification

**Credentials:**
- **Username:** Premier compte email créé (ex: `admin`)
- **Password:** Mot de passe du compte email (depuis `hibana-config.yaml`)

**Mécanisme:**
- Authentification JWT (tokens valides 24h)
- Tokens stockés dans localStorage
- Renouvellement automatique à chaque requête

## 🔧 Fonctionnalités de l'Interface

### Dashboard
- Statistiques en temps réel
- Nombre de domaines, comptes email, records DNS
- Vue d'ensemble par domaine

### Gestion des Domaines
- **Créer** : Nouveau domaine avec utilisateur SSH optionnel
- **Éditer** : Modifier l'IP du serveur
- **Supprimer** : Supprime domaine + records associés
- **Options** :
  - Création utilisateur système automatique
  - Mode SSH key (auto/manual)
  - Configuration IP serveur

### Gestion Email
- **Créer** : Nouveau compte email pour un domaine
- **Éditer** : Changer mot de passe, nom complet
- **Supprimer** : Retirer un compte
- **Vue** : Liste tous les comptes avec domaine

### Gestion DNS
- **Types supportés** : A, AAAA, CNAME, MX, TXT, NS, SRV, CAA
- **Créer** : Ajouter un nouveau record
- **Éditer** : Modifier content, TTL, priority
- **Supprimer** : Retirer un record
- **Filtrage** : Par domaine

### Gestion des Clés SSH
- **Lister** : Afficher toutes les clés SSH d'un domaine
- **Ajouter** : Ajouter une nouvelle clé SSH avec label
- **Supprimer** : Révoquer l'accès en supprimant une clé
- **Fingerprint** : Affichage du fingerprint SHA256 pour vérification
- **Multi-utilisateurs** : Plusieurs clés SSH par domaine (accès équipe)
- **Emplacement** : `/srv/domainname/.ssh/authorized_keys`

## 🛠️ Commandes Utiles

### Vérifier le statut du conteneur

```bash
docker ps | grep api
```

### Voir les logs

```bash
docker logs -f api-<domaine>

# Ou
tail -f /srv/<domaine>/api/logs/api.log
```

### Redémarrer l'API

```bash
docker-compose -f /srv/<domaine>/api/docker-compose.yml restart
```

### Rebuild complet

```bash
# 1. Rebuild localement
./build-all.sh

# 2. Copier les nouveaux fichiers
sudo cp bin/hibana-api /srv/<domaine>/api/app/
sudo cp -r web/admin/dist/* /srv/<domaine>/api/app/web/

# 3. Rebuild et redémarrer le conteneur
cd /srv/<domaine>/api
sudo docker-compose up -d --build
```

## 🔐 Sécurité

### Authentification
- Tous les endpoints (sauf `/login` et `/health`) nécessitent JWT
- Tokens expirés automatiquement après 24h
- Logout côté client efface le token

### Base de Données
- Connexions via secrets Docker
- Mots de passe bcrypt (coût 10)
- Requêtes préparées (protection SQL injection)

### Réseau
- Conteneur sur réseau `traefik-network`
- Exposition uniquement via Traefik
- HTTPS forcé avec Let's Encrypt
- CORS configuré

### Secrets
- JWT secret auto-généré (base64, 32 bytes)
- Stocké dans `/srv/<domaine>/api/secrets/`
- Permissions 600 (root seulement)

## 🔄 Fallback : Placeholder API

Si le build échoue (npm manquant, erreur de compilation, etc.), l'installation déploie automatiquement une **API placeholder** :

```
⚠️  Warning: Failed to build API/frontend: <erreur>
   API will use placeholder. You can build manually later with:
   ./build-all.sh && docker-compose -f /srv/<domain>/api/docker-compose.yml up -d --build
```

Le placeholder fournit :
- Endpoint `/health` (health check)
- Endpoint `/` (info service)
- Message "Phase 2 - Coming Soon"

Pour remplacer le placeholder par la vraie API :

```bash
# 1. Installer Node.js si manquant
sudo apt install nodejs npm

# 2. Build
./build-all.sh

# 3. Relancer l'installation (idempotente)
sudo ./bin/hibana init
```

## 📊 Architecture Technique

```
┌──────────────┐
│   Browser    │
└──────┬───────┘
       │ HTTPS
       ▼
┌──────────────┐
│   Traefik    │ (adm.domain.com)
└──────┬───────┘
       │ HTTP :3000
       ▼
┌──────────────────────────┐
│   Docker Container       │
│  ┌────────────────────┐  │
│  │   hibana-api       │  │
│  │   (Go binary)      │  │
│  │                    │  │
│  │   Serves:          │  │
│  │   - API /api/v1/*  │  │
│  │   - Static /       │  │
│  └────────────────────┘  │
│         ▲                │
│         │                │
│    ┌────┴─────┐          │
│    │   web/   │          │
│    │ (React)  │          │
│    └──────────┘          │
└──────────────────────────┘
       │
       │ PostgreSQL
       ▼
┌──────────────┐
│   Database   │
│   - hibana   │
│   - pdns     │
└──────────────┘
```

## 🎯 Points Clés

1. **Automatique** : Pas besoin de commande manuelle après installation
2. **Intégré** : Fait partie du processus d'installation standard
3. **SSL** : Certificat Let's Encrypt automatique via Traefik
4. **Sécurisé** : JWT, HTTPS, secrets protégés
5. **Production-ready** : Conteneur Docker, restart automatique
6. **Fallback** : Placeholder si build échoue

## 🐛 Dépannage

### L'interface n'est pas accessible

```bash
# Vérifier que le conteneur tourne
docker ps | grep api

# Vérifier les logs
docker logs api-<domain>

# Vérifier Traefik
docker logs traefik

# Vérifier le DNS
dig adm.<domain>
```

### Erreur 401 Unauthorized

- Vérifiez que vous utilisez les bons credentials (email account)
- Token peut avoir expiré, reconnectez-vous
- Vérifiez que le JWT secret existe : `ls /srv/<domain>/api/secrets/jwt_secret`

### Erreur de connexion DB

```bash
# Vérifier que PostgreSQL tourne
sudo systemctl status postgresql

# Vérifier le mot de passe
cat /etc/hibana/secrets/postgresql_hibana_password

# Vérifier que l'API peut se connecter
docker exec api-<domain> wget -q -O- http://localhost:3000/health
```

## 📚 Références

- [README.md](README.md) - Documentation principale
- [web/admin/README.md](web/admin/README.md) - Documentation frontend React
- [API Documentation](internal/api/) - Code source API
