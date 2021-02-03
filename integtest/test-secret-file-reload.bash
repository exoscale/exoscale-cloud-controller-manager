#!/usr/bin/env bash

set -e

source "$INTEGTEST_DIR/test-helpers.bash"

echo ">>> TESTING RELOADING OF SECRET FILE"

kubectl certificate approve $(kubectl get csr --field-selector spec.signerName=kubernetes.io/kubelet-serving -o name)
CCM_POD="$(kubectl get pods -n kube-system -l app=exoscale-cloud-controller-manager -o name)"

_until_success "kubectl -n kube-system logs \"$CCM_POD\" | grep -m 1 \"Exoscale API credentials refreshed, now using test\""

IAM_SECRET_JSON="{\"name\":\"good\",\"api_key\":\"$EXOSCALE_API_KEY\",\"api_secret\":\"$EXOSCALE_API_SECRET\"}"
kubectl exec -ti -n kube-system "$CCM_POD" -- env IAM_SECRET_JSON="$IAM_SECRET_JSON" ash -c 'echo $IAM_SECRET_JSON > /tmp/iam-keys'

_until_success "kubectl -n kube-system logs \"$CCM_POD\" | grep -m 1 \"Exoscale API credentials refreshed, now using good\""

echo "<<< PASS"
