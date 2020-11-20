#!/usr/bin/env bash

set -e
set -x

export INTEGTEST_DIR="${INCLUDE_PATH}/integtest"
export INTEGTEST_TMP_DIR="${INTEGTEST_DIR}/tmp"
mkdir "$INTEGTEST_TMP_DIR"

[[ -n "$EXOSCALE_API_KEY" ]]
[[ -n "$EXOSCALE_API_SECRET" ]]
[[ -n "$EXOSCALE_API_ENDPOINT" ]]

source "$INTEGTEST_DIR/test-helpers.bash"

export EXOSCALE_ZONE=${EXOSCALE_ZONE:-de-fra-1}
export KUBECTL_OPTS="--timeout=600s"
export TERRAFORM_OPTS="-auto-approve -backup=-"


cleanup() {
  echo ">>> CLEANING UP <<<"

  terraform destroy $TERRAFORM_OPTS
  rm -rf "${INTEGTEST_TMP_DIR}"
}

deploy_cluster() {
  echo ">>> DEPLOYING CLUSTER INFRASTRUCTURE <<<"

  cd "$INTEGTEST_DIR"
  printf "zone = \"%s\"\ntmpdir = \"%s\"\n" $EXOSCALE_ZONE "$INTEGTEST_TMP_DIR" > terraform.tfvars
  terraform init
  terraform apply $TERRAFORM_OPTS

  # Workaround for a problem using GitHub Action hashicorp/setup-terraform@v1:
  # https://github.com/hashicorp/setup-terraform/issues/20
  export TEST_ID=$(terraform-bin output test_id)
  export NODEPOOL_ID=$(terraform-bin output nodepool_id)
  export KUBECONFIG="${INTEGTEST_TMP_DIR}/kubeconfig"

  _until_success "kubectl cluster-info"
}

deploy_ingress_controller() {
  echo ">>> DEPLOYING CLUSTER INGRESS CONTROLLER <<<"

  export EXOSCALE_CCM_LB_NAME="test-k8s-ccm-${TEST_ID}"

  sed -r \
    -e "s/%%EXOSCALE_ZONE%%/$EXOSCALE_ZONE/" \
    -e "s/%%EXOSCALE_CCM_LB_NAME%%/$EXOSCALE_CCM_LB_NAME/" \
    "${INTEGTEST_DIR}/manifests/ingress-nginx.yml.tpl" \
    | kubectl $KUBECTL_OPTS apply -f -

  # It is not possible to `kubectl wait` on an Ingress resource, so we wait until
  # we see a public IP address associated to the Service Load Balancer...
  _until_success "test -n \"\$(kubectl --namespace ingress-nginx get svc/ingress-nginx-controller \
    -o=jsonpath='{.status.loadBalancer.ingress[].ip}')\""

  export INGRESS_NLB_IP=$(kubectl --namespace ingress-nginx get svc/ingress-nginx-controller \
    -o=jsonpath='{.status.loadBalancer.ingress[].ip}')

  export INGRESS_NLB_ID=$(exo nlb list -z $EXOSCALE_ZONE -O text \
    | awk "/${INGRESS_NLB_IP}/ { print \$1 }")
}

deploy_test_app() {
  echo ">>> DEPLOYING TEST APPLICATION <<<"

  kubectl apply -f "${INTEGTEST_DIR}/manifests/hello.yml"
  kubectl $KUBECTL_OPTS wait --for condition=Available deployment.apps/hello
}

test_node_labels() {
  echo ">>> TESTING CCM-MANAGED KUBERNETES NODE LABELS <<<"
  . "${INTEGTEST_DIR}/test-labels.bash"
  echo "PASS"
}

test_ingress_nlb() {
  echo ">>> TESTING CCM-MANAGED NLB INSTANCE <<<"
  . "${INTEGTEST_DIR}/test-nlb.bash"
  echo "PASS"
}

test_node_expunge() {
  echo ">>> TESTING CCM-MANAGED NODE EXPUNGING <<<"
  . "${INTEGTEST_DIR}/test-node-expunge.bash"
  echo "PASS"
}

trap cleanup EXIT
deploy_cluster
deploy_ingress_controller
deploy_test_app
test_node_labels
test_ingress_nlb
test_node_expunge
