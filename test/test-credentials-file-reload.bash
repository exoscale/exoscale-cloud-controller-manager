#!/usr/bin/env bash

set -e
cd "$(dirname "$(readlink -e "${0}")")"
source "lib/test-helpers.bash"

echo ">>> TESTING API CREDENTIALS FILE RELOADING"

echo "### Checking initial API credentials ..."
_until_success "grep -m 1 \"Exoscale API credentials refreshed, now using test\" ccm.log"

echo "### Refreshing API credentials ..."
# WARNING: the credentials file creation must be atomic (or CCM might read incomplete content)
umask 077 && cat > api-creds.new <<EOF
{
	"name": "good",
	"api_key": "$EXOSCALE_API_KEY",
	"api_secret": "$EXOSCALE_API_SECRET"
}
EOF
ln -fs api-creds.new api-creds

_until_success "grep -m 1 \"Exoscale API credentials refreshed, now using good\" ccm.log"

echo "<<< PASS"