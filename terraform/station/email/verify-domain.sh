#!/usr/bin/env bash
# Trigger ACS verification after Terraform has published the DNS records.
set -euo pipefail

RESOURCE_GROUP=${RESOURCE_GROUP:-caspar-water-production-backups}
EMAIL_SERVICE=${EMAIL_SERVICE:-caspar-water-email}
DOMAIN=${DOMAIN:-casparwater.us}

for verification in Domain SPF DKIM DKIM2; do
    az communication email domain initiate-verification \
        --resource-group "${RESOURCE_GROUP}" \
        --email-service-name "${EMAIL_SERVICE}" \
        --domain-name "${DOMAIN}" \
        --verification-type "${verification}"
done
