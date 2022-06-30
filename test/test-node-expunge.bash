#!/usr/bin/env bash

set -e
cd "$(dirname "$(readlink -e "${0}")")"
source "lib/test-helpers.bash"

echo ">>> TESTING CCM-MANAGED NODE EXPUNGING"

echo "### Removing node-pool from the cluster ..."
cd "terraform-${TARGET_CLUSTER}"
terraform apply -var 'pool_size=0' -auto-approve > test-node-expunge-terraform.log
cd - > /dev/null

_until_success "test -z \"\$(kubectl get nodes 2> /dev/null |grep test-ccm |grep pool)\""

echo "<<< PASS"
