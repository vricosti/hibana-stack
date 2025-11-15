#!/bin/bash
# Migration script to reorganize /srv from subdomain-based to domain-based structure
# From: /srv/adm.primarydomain.com, /srv/api.primarydomain.com, etc.
# To: /srv/primarydomain.com/adm, /srv/primarydomain.com/api, etc.

set -e

DOMAIN="${1:-primarydomain.com}"
SRV_DIR="/srv"

echo "========================================="
echo "Domain Structure Migration Script"
echo "========================================="
echo "Domain: $DOMAIN"
echo "Target: $SRV_DIR/$DOMAIN/"
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "ERROR: This script must be run as root (use sudo)"
    exit 1
fi

# Stop all services
echo "Step 1: Stopping Docker services..."
for service in adm api webmail www; do
    if [ -d "$SRV_DIR/${service}.$DOMAIN" ]; then
        echo "  Stopping ${service}.$DOMAIN..."
        cd "$SRV_DIR/${service}.$DOMAIN"
        docker-compose down || true
    fi
done

echo ""
echo "Step 2: Creating new domain directory structure..."
mkdir -p "$SRV_DIR/$DOMAIN"

# Backup old structure
echo ""
echo "Step 3: Creating backup..."
BACKUP_DIR="/tmp/srv-backup-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BACKUP_DIR"

# Move services to new structure
echo ""
echo "Step 4: Moving services to new structure..."

for service in adm api webmail www; do
    OLD_PATH="$SRV_DIR/${service}.$DOMAIN"
    NEW_PATH="$SRV_DIR/$DOMAIN/$service"

    if [ -d "$OLD_PATH" ]; then
        echo "  Moving ${service}.$DOMAIN -> $DOMAIN/$service..."

        # Create backup
        cp -a "$OLD_PATH" "$BACKUP_DIR/"

        # Move to new location
        mv "$OLD_PATH" "$NEW_PATH"

        echo "    ✓ Moved successfully"
    else
        echo "    ⚠ $OLD_PATH not found, skipping"
    fi
done

# Fix permissions
echo ""
echo "Step 5: Fixing permissions..."
chown -R vricosti:vricosti "$SRV_DIR/$DOMAIN/api" || true
chown -R vricosti:vricosti "$SRV_DIR/$DOMAIN/webmail" || true

echo ""
echo "Step 6: Creating symbolic links for backward compatibility..."
for service in adm api webmail www; do
    NEW_PATH="$SRV_DIR/$DOMAIN/$service"
    OLD_LINK="$SRV_DIR/${service}.$DOMAIN"

    if [ -d "$NEW_PATH" ] && [ ! -e "$OLD_LINK" ]; then
        ln -s "$NEW_PATH" "$OLD_LINK"
        echo "  Created symlink: ${service}.$DOMAIN -> $DOMAIN/$service"
    fi
done

echo ""
echo "========================================="
echo "Migration completed successfully!"
echo "========================================="
echo ""
echo "Backup location: $BACKUP_DIR"
echo ""
echo "New structure:"
ls -la "$SRV_DIR/$DOMAIN/"
echo ""
echo "Symbolic links (for backward compatibility):"
ls -la "$SRV_DIR/" | grep "^l"
echo ""
echo "Next steps:"
echo "1. Update Ansible configurations to use new paths"
echo "2. Restart services with: cd /srv/$DOMAIN/<service> && docker-compose up -d"
echo "3. Verify all services are working"
echo "4. Remove symbolic links once confirmed: rm /srv/{adm,api,webmail,www}.$DOMAIN"
echo ""
