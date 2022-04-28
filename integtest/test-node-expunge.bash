#!/usr/bin/env bash

set -e

source "$INTEGTEST_DIR/test-helpers.bash"

echo ">>> TESTING CCM-MANAGED NODE EXPUNGING"

cd "$INTEGTEST_DIR"

terraform destroy $TERRAFORM_OPTS -target exoscale_instance_pool.test

_until_success "test -z \"\$(kubectl get node $NODEPOOL_INSTANCE_NAME 2> /dev/null)\""

rm -f .terraform.applied

echo "<<< PASS"
