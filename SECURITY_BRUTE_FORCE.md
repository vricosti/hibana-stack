# Protection Anti-Brute Force pour l'Interface Admin

## Résumé

Le backend de l'interface d'administration (`adm.vridev.com`) intègre maintenant une protection robuste contre les attaques par brute force sur les endpoints d'authentification.

## Caractéristiques

### 1. Rate Limiting par IP

- **Limite**: 5 tentatives de connexion échouées maximum
- **Période de blocage initiale**: 15 minutes
- **Blocage progressif**: Le temps de blocage augmente avec chaque nouvelle série de tentatives échouées

### 2. Fonctionnement

#### Tentatives Échouées
- Chaque échec de connexion (utilisateur inexistant ou mauvais mot de passe) incrémente le compteur pour l'IP source
- Après 5 échecs, l'IP est bloquée pendant 15 minutes
- Les tentatives suivantes prolongent le blocage (15min × nombre de fois bloqué)

#### Détection de l'IP Réelle
Le système détecte correctement l'IP du client même derrière Traefik en utilisant :
- `X-Real-IP` (prioritaire, défini par Traefik)
- `X-Forwarded-For` (fallback)
- `RemoteAddr` (dernier recours)

#### Réinitialisation
- Une connexion réussie réinitialise le compteur pour cette IP
- Les entrées sont automatiquement nettoyées après 1 heure d'inactivité

### 3. Logging

Tous les événements sont loggés :
```
Failed login attempt from IP 192.168.1.100 for user admin (attempt 3)
Blocked login attempt from IP 192.168.1.100
Successful login from IP 192.168.1.100 for user admin
```

### 4. Réponses HTTP

- **429 Too Many Requests**: IP bloquée
  ```json
  {
    "success": false,
    "error": "Too many failed login attempts. Try again later."
  }
  ```

- **401 Unauthorized**: Identifiants invalides
  ```json
  {
    "success": false,
    "error": "invalid credentials"
  }
  ```

## Configuration

La configuration se trouve dans `internal/api/server.go` :

```go
// Block after 5 failed attempts for 15 minutes
loginLimiter := ratelimit.NewLoginLimiter(5, 15*time.Minute)
```

Pour modifier :
- **Nombre de tentatives** : Premier paramètre (actuellement `5`)
- **Durée du blocage** : Second paramètre (actuellement `15*time.Minute`)

## Comparaison avec Fail2ban

| Aspect | Protection Backend | Fail2ban |
|--------|-------------------|----------|
| **Scope** | Endpoint `/api/v1/auth/login` uniquement | Tous les logs système |
| **Installation** | Intégré, aucune config externe | Nécessite installation et configuration |
| **Performance** | En mémoire, très rapide | Analyse de fichiers logs |
| **Persistance** | Non (redémarrage = reset) | Oui (via iptables) |
| **Configuration** | Code Go simple | Fichiers de configuration complexes |
| **Granularité** | Par endpoint | Par service (SSH, mail, etc.) |

### Pourquoi ne pas utiliser Fail2ban ?

1. **Fail2ban est déjà actif** sur le serveur pour SSH, Postfix, et Dovecot
2. **L'API est derrière Traefik** : Fail2ban devrait analyser les logs Traefik ou les logs de l'API
3. **Protection native plus fine** : Le backend connaît exactement la nature de chaque requête
4. **Pas de dépendance système** : Fonctionne identiquement en dev et en production

### Quand utiliser Fail2ban ?

Fail2ban serait pertinent si :
- Vous voulez bloquer au niveau firewall (iptables)
- Vous voulez une persistance entre redémarrages
- Vous voulez protéger d'autres endpoints (pas seulement login)

Pour ajouter une jail Fail2ban pour l'API :

```ini
# /etc/fail2ban/jail.d/hibana-api.conf
[hibana-api]
enabled = true
port = http,https
filter = hibana-api
logpath = /srv/vridev.com/api/logs/api.log
maxretry = 5
bantime = 900
findtime = 600
```

```ini
# /etc/fail2ban/filter.d/hibana-api.conf
[Definition]
failregex = ^.*Failed login attempt from IP <HOST>.*$
ignoreregex =
```

## Sécurité du Mot de Passe Admin

### Mot de passe actuel

**⚠️ CONFIDENTIEL**

Le mot de passe admin a été modifié pour plus de sécurité :

- **Stockage sécurisé** : `/etc/hibana/admin_password.txt` (permissions 600)
- **Hash** : bcrypt avec cost 10
- **Longueur** : 40 caractères alphanumériques

Pour se connecter :
```
URL : https://adm.vridev.com
Username : admin
Password : [voir /etc/hibana/admin_password.txt]
```

### Bonnes pratiques

1. **Ne jamais** commit le mot de passe dans Git
2. **Changer régulièrement** le mot de passe admin
3. **Utiliser des mots de passe différents** pour chaque environnement
4. **Activer l'authentification à deux facteurs** (future amélioration)

## Surveillance

### Vérifier les tentatives échouées

```bash
sudo docker logs api-vridev-com | grep "Failed login"
```

### Vérifier les blocages

```bash
sudo docker logs api-vridev-com | grep "Blocked login"
```

### Voir toutes les tentatives de connexion

```bash
sudo docker logs api-vridev-com | grep -E "login|Login"
```

## Tests

### Tester le rate limiting

```bash
# Faire 6 tentatives échouées
for i in {1..6}; do
  curl -k -X POST https://adm.vridev.com/api/v1/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"wrong"}'
  echo ""
done
```

La 6ème requête devrait retourner un code 429.

### Vérifier les logs

```bash
sudo docker logs api-vridev-com | tail -20
```

## Améliorations Futures

1. **Persistance Redis** : Garder l'état entre redémarrages
2. **Whitelist d'IPs** : Ne pas bloquer certaines IPs de confiance
3. **Alertes email** : Notifier l'admin après X tentatives échouées
4. **Captcha** : Ajouter un captcha après 3 tentatives échouées
5. **2FA** : Authentification à deux facteurs obligatoire pour admin
6. **Audit trail** : Enregistrer toutes les connexions dans la base de données
