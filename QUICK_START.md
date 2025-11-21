# Quick Start Guide - Hibana Stack

## Installation en 3 Étapes

### Étape 1 : Préparer l'installation

```bash
# Cloner le repository
git clone https://github.com/vricosti/hibana-stack.git
cd hibana-stack

# Préparer le build (API + Frontend)
./prepare-install.sh
```

**Ce script va :**
- Compiler l'API Go
- Vérifier si Node.js est installé
- Proposer d'installer Node.js si absent
- Builder le frontend React
- Créer tous les artefacts nécessaires

### Étape 2 : Configurer

```bash
# Générer la configuration
sudo ./bin/hibana init

# Éditer avec vos informations
nano hibana-config.yaml
```

**Configuration minimale :**

```yaml
primary_domain: votre-domaine.com
server_ip: VOTRE_IP_SERVEUR

# Optionnel : Créer automatiquement un utilisateur SSH par domaine
domain_user:
  ssh_key_mode: auto  # ou "manual" avec votre clé

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
    password: VOTRE_MOT_DE_PASSE_FORT
    full_name: Administrator
```

### Étape 3 : Installer

```bash
# Lancer l'installation complète
sudo ./bin/hibana init
```

**Durée :** 5-10 minutes

**L'installation va :**
- ✅ Installer et configurer PowerDNS
- ✅ Configurer le serveur mail (Postfix + Dovecot)
- ✅ Installer Traefik avec SSL automatique
- ✅ Déployer tous les conteneurs Docker
- ✅ Configurer DKIM, DMARC, SPF
- ✅ Déployer l'interface d'administration
- ✅ Créer l'utilisateur de domaine (si configuré)

## Accès aux Services

Une fois l'installation terminée :

### Interface d'Administration
```
https://adm.votre-domaine.com
```
**Login:** Utilisez les credentials du compte email `admin`

### Webmail
```
https://webmail.votre-domaine.com
```

### Site Web
```
https://www.votre-domaine.com
```

### Serveur Mail
```
mail.votre-domaine.com
```

## Prochaines Étapes

### 1. Configurer les DNS

Chez votre registrar, configurez les nameservers pour pointer vers votre serveur :

```
ns1.votre-domaine.com -> VOTRE_IP_SERVEUR
```

### 2. Vérifier les Services

```bash
# Vérifier tous les conteneurs
docker ps

# Vérifier Traefik
docker logs traefik

# Vérifier l'API
docker logs api-votre_domaine_com

# Vérifier le mail
sudo systemctl status postfix
sudo systemctl status dovecot
```

### 3. Tester l'Email

```bash
# Envoyer un email de test
echo "Test" | mail -s "Test" votre-email@example.com
```

### 4. Gérer via l'Interface Web

Connectez-vous à `https://adm.votre-domaine.com` pour :
- Ajouter de nouveaux domaines
- Créer des comptes email
- Gérer les enregistrements DNS
- Voir les statistiques

## Dépannage

### Node.js n'est pas installé

Si vous sautez l'installation de Node.js, l'API placeholder sera utilisée.

**Pour installer Node.js plus tard :**

```bash
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt-get install -y nodejs

# Puis rebuild
./build-all.sh

# Redéployer l'API
cd /srv/votre-domaine.com/api
sudo docker-compose up -d --build
```

### Vérifier les logs

```bash
# Logs API
docker logs -f api-votre_domaine_com

# Logs Traefik
docker logs -f traefik

# Logs Mail
sudo tail -f /var/log/mail.log

# Logs DNS
sudo journalctl -u pdns -f
```

### Problèmes de certificat SSL

```bash
# Vérifier Traefik
docker logs traefik | grep -i certificate

# Forcer le renouvellement
docker restart traefik
```

### L'interface admin ne répond pas

```bash
# Vérifier le conteneur
docker ps | grep api

# Restart
docker restart api-votre_domaine_com

# Vérifier les logs
docker logs api-votre_domaine_com
```

## Commandes Utiles

### Gestion des Conteneurs

```bash
# Lister tous les conteneurs
docker ps

# Restart tous les services
cd /srv/votre-domaine.com/api && docker-compose restart
cd /srv/votre-domaine.com/webmail && docker-compose restart

# Voir les logs
docker logs -f <container-name>
```

### Gestion du Mail

```bash
# Status
sudo systemctl status postfix
sudo systemctl status dovecot

# Restart
sudo systemctl restart postfix
sudo systemctl restart dovecot

# Queue mail
sudo mailq

# Test SMTP
telnet localhost 25
```

### Gestion DNS

```bash
# Status PowerDNS
sudo systemctl status pdns

# Tester une requête
dig @localhost votre-domaine.com

# Voir les zones
sudo pdnsutil list-all-zones
```

## Sécurité

### Firewall

Par défaut, Hibana ouvre ces ports :

```
22/tcp   - SSH
25/tcp   - SMTP
80/tcp   - HTTP (redirect vers HTTPS)
443/tcp  - HTTPS
53/tcp   - DNS
53/udp   - DNS
587/tcp  - SMTP Submission
993/tcp  - IMAPS
```

### Backups

**Importantes choses à sauvegarder :**

```bash
# Configuration
/etc/hibana/

# Secrets
/etc/hibana/secrets/

# Base de données
pg_dump hibana > hibana_backup.sql
pg_dump pdns > pdns_backup.sql

# Mail
/var/mail/
/var/vmail/

# Docker volumes
docker volume ls
```

### Mises à jour

```bash
# Système
sudo apt update && sudo apt upgrade

# Conteneurs Docker
cd /srv/votre-domaine.com/api
sudo docker-compose pull
sudo docker-compose up -d
```

## Support

- 📘 Documentation complète : [README.md](README.md)
- 🐛 Issues : https://github.com/vricosti/hibana-stack/issues
- 📖 Phase 2 Details : [PHASE2_DEPLOYMENT.md](PHASE2_DEPLOYMENT.md)

## License

Apache License 2.0 - Voir [LICENSE](LICENSE)
