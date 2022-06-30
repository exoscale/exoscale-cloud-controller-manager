#!/usr/bin/env bash

set -e
cd "$(dirname "$(readlink -e "${0}")")"
source "lib/test-helpers.bash"

echo ">>> TESTING SKS AGENT NODE CSR VALIDATION"

echo "### Adding a Node to the cluster ..."

cd "terraform-${TARGET_CLUSTER}"
terraform apply -var 'pool_size=2' -auto-approve > test-csr-validation-terraform.log
cd - > /dev/null

_until_success "test \$(kubectl get nodes --no-headers -l '!node-role.kubernetes.io/control-plane' 2> /dev/null | grep '^test-\S*-pool-' | wc -l) -ge 2"

if [ "$TARGET_CLUSTER" != "sks" ]; then
	echo "### (external node)"
	_until_success "test \$(kubectl get nodes --no-headers -l '!node-role.kubernetes.io/control-plane' 2> /dev/null | grep '^test-\S*-external\s' | wc -l) -ge 1"
fi

echo "### Checking that the Node CSR got approved ..."

_until_success "test \$(kubectl get csr --no-headers | grep '\ssystem:node:test-\S*-pool-\S.*\sApproved' 2> /dev/null | wc -l) -ge 2"

if [ "$TARGET_CLUSTER" != "sks" ]; then
	echo "### (external node)"
	_until_success "test \$(kubectl get csr --no-headers | grep '\ssystem:node:test-\S*-external\s.*\sApproved' 2> /dev/null | wc -l) -ge 1"
fi

echo "<<< PASS"
