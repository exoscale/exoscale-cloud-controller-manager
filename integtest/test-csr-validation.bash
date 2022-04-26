#!/usr/bin/env bash

set -e

source "$INTEGTEST_DIR/test-helpers.bash"

echo ">>> TESTING SKS AGENT NODE CSR VALIDATION"

echo "- Adding a Node to the cluster"

exo compute instance-pool scale -f -z $EXOSCALE_ZONE $NODEPOOL_ID 2 > /dev/null

_until_success "test \$(kubectl get nodes --no-headers -l '!node-role.kubernetes.io/master' | grep '^test-\S*-pool-' | wc -l) -ge 2"

echo "- Checking that the Node CSR got approved"

_until_success "test \$(kubectl get csr --no-headers | grep '\ssystem:node:test-\S*-pool-\S.*\sApproved' | wc -l) -ge 2"

echo "<<< PASS"
