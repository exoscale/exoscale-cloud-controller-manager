#!/usr/bin/env bash

set -e

source "$INTEGTEST_DIR/test-helpers.bash"

echo ">>> TESTING API CREDENTIALS FILE RELOADING"
CCM_POD="$(kubectl get pods -n kube-system -l app=exoscale-cloud-controller-manager -o name)"

echo "- Checking initial API credentials"

_until_success "kubectl -n kube-system logs \"$CCM_POD\" | grep -m 1 \"Exoscale API credentials refreshed, now using test\""

echo "- Refreshing API credentials"

API_CREDS_JSON="{\"name\":\"good\",\"api_key\":\"$EXOSCALE_API_KEY\",\"api_secret\":\"$EXOSCALE_API_SECRET\"}"
kubectl exec -n kube-system "$CCM_POD" -- env API_CREDS_JSON="$API_CREDS_JSON" ash -c 'echo $API_CREDS_JSON > /tmp/api-creds'

_until_success "kubectl -n kube-system logs \"$CCM_POD\" | grep -m 1 \"Exoscale API credentials refreshed, now using good\""

echo "<<< PASS"
