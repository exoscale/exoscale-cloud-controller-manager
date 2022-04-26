#!/usr/bin/env bash

set -e

source "$INTEGTEST_DIR/test-helpers.bash"

echo ">>> DEPLOYING TEST CLUSTER INFRASTRUCTURE"

{
  cd "$INTEGTEST_DIR"
  if [ ! -e terraform.tfvars ]; then
    printf "zone = \"%s\"\ntmpdir = \"%s\"\n" $EXOSCALE_ZONE "$INTEGTEST_TMP_DIR" > terraform.tfvars
  fi
  if [ ! -e .terraform.lock.hcl ]; then
    terraform init
  fi
  if [ ! -e .terraform.applied ]; then
    terraform apply $TERRAFORM_OPTS
    touch .terraform.applied
  fi

  # Workaround for a problem using GitHub Action hashicorp/setup-terraform@v1:
  # https://github.com/hashicorp/setup-terraform/issues/20
  # Starting from 0.14.0 `terraform output` now displays quotes around output values:
  # https://github.com/hashicorp/terraform/issues/26831
  export TEST_ID="$(terraform output -json | jq -r .test_id.value)"
  export NODEPOOL_ID="$(terraform output -json | jq -r .nodepool_id.value)"
  export EXTERNAL_NLB_ID="$(terraform output -json | jq -r .external_nlb_id.value)"
  export EXTERNAL_NLB_NAME="$(terraform output -json | jq -r .external_nlb_name.value)"
  export EXTERNAL_NLB_DESC="$(terraform output -json | jq -r .external_nlb_desc.value)"
  export EXTERNAL_NLB_IP="$(terraform output -json | jq -r .external_nlb_ip.value)"
  export KUBECONFIG="${INTEGTEST_TMP_DIR}/kubeconfig"
}

echo "### Checking control-plane availability ..."
_until_success "kubectl cluster-info"

echo "### Waiting for (and approving) node CSRs ..."
_until_success "test \$(kubectl get csr --field-selector spec.signerName=kubernetes.io/kubelet-serving -o name | wc -l) -ge 3"
kubectl certificate approve $(kubectl get csr --field-selector spec.signerName=kubernetes.io/kubelet-serving -o name)

echo "<<< DONE"
