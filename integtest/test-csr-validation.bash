#!/usr/bin/env bash

set -e

source "$INTEGTEST_DIR/test-helpers.bash"

echo ">>> TESTING SKS AGENT NODE CSR VALIDATION"

echo "- Adding a Node to the cluster"

exo instancepool update $NODEPOOL_ID \
  -z $EXOSCALE_ZONE \
  --size 2 > /dev/null

_until_success "test \$(kubectl get nodes --no-headers -l '!node-role.kubernetes.io/master' | wc -l) -eq 2"

echo "- Checking that the Node CSR got approved"

_until_success "test \$(kubectl get csr --no-headers | awk '\$4 ~ /^system:node:pool-/ && \$5 ~ /Approved/' | wc -l) -eq 2"

CCM_POD="$(kubectl get pods -n kube-system -l app=exoscale-cloud-controller-manager -o name)"
test $(kubectl -n kube-system logs $CCM_POD | grep -c "sks-agent: CSR .* approved") -eq 2

echo "<<< PASS"
