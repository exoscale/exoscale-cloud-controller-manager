#!/usr/bin/env bash

set -e

source "$INTEGTEST_DIR/test-helpers.bash"

echo ">>> TESTING SKS AGENT NODE CSR VALIDATION"

echo "- Adding a Node to the cluster"

exo instancepool scale -f -z $EXOSCALE_ZONE $NODEPOOL_ID 2 > /dev/null

_until_success "test \$(kubectl get nodes --no-headers -l '!node-role.kubernetes.io/master' | wc -l) -eq 2"

echo "- Checking that the Node CSR got approved"

_until_success "test \$(kubectl get csr --no-headers | awk '\$4 ~ /^system:node:pool-/ && \$5 ~ /Approved/' | wc -l) -eq 2"

echo "<<< PASS"
