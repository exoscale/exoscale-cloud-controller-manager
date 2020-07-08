#!/usr/bin/env bash

set -e
set -u
# set -x

export INTEGTEST_DIR="${INCLUDE_PATH}/integtest"

[[ -n "$EXOSCALE_API_KEY" ]]
[[ -n "$EXOSCALE_API_SECRET" ]]
[[ -n "$EXOSCALE_API_ENDPOINT" ]]

EXOSCALE_MASTER_NAME=k8s-ccm-master-$(uuidgen | tr '[:upper:]' '[:lower:]')
export EXOSCALE_MASTER_NAME
EXOSCALE_INSTANCEPOOL_NAME=k8s-ccm-nodes-$(uuidgen | tr '[:upper:]' '[:lower:]')
export EXOSCALE_MASTER_NAME
EXOSCALE_SSHKEY_NAME=k8s-ccm-sshkey-$(uuidgen | tr '[:upper:]' '[:lower:]')
export EXOSCALE_SSHKEY_NAME
EXOSCALE_LB_NAME=k8s-ccm-lb-$(uuidgen | tr '[:upper:]' '[:lower:]')
export EXOSCALE_LB_NAME
EXOSCALE_LB_SERVICE_NAME1=k8s-ccm-lb-service-$(uuidgen | tr '[:upper:]' '[:lower:]')
export EXOSCALE_LB_SERVICE_NAME1
EXOSCALE_LB_SERVICE_NAME2=k8s-ccm-lb-service-$(uuidgen | tr '[:upper:]' '[:lower:]')
export EXOSCALE_LB_SERVICE_NAME2

cleanup() {
    rm -rf "${INTEGTEST_DIR}/.ssh"
    rm -rf "${INTEGTEST_DIR}/.kube"
    rm -rf "${INTEGTEST_DIR}/create-nlb.yml"
    rm -rf "${INTEGTEST_DIR}/add-nlb-service.yml"
    rm -rf "${INTEGTEST_DIR}/node-join-cloud-init.yml"
    exo --quiet vm delete --force "$EXOSCALE_MASTER_NAME"
    exo --quiet nlb delete --force "$EXOSCALE_LB_NAME" -z de-fra-1 &>/dev/null || true
    until_success "exo --quiet instancepool delete --force \"${EXOSCALE_INSTANCEPOOL_NAME}\" -z de-fra-1"
    until_success "exo --quiet sshkey delete --force \"$EXOSCALE_SSHKEY_NAME\""
}
trap cleanup EXIT

until_success() {
    declare command="$1"
    timeout 3m bash -c "until $command &>/dev/null; do sleep 5; done"
}

remote_run() {
    declare ip_address="$1"
    declare command="$2"

    ssh -i "${INTEGTEST_DIR}/.ssh/id_rsa" "ubuntu@${ip_address}" "$command"
} 

upload_integtest_sshkey() {
    mkdir -p "${INTEGTEST_DIR}/.ssh"
    ssh-keygen -t rsa -f "${INTEGTEST_DIR}/.ssh/id_rsa" -N ""
    exo --quiet sshkey upload "${EXOSCALE_SSHKEY_NAME}" "${INTEGTEST_DIR}/.ssh/id_rsa.pub"
}

create_exoscale_vm() {
 exo --quiet vm create "${EXOSCALE_MASTER_NAME}" \
        --keypair "${EXOSCALE_SSHKEY_NAME}" \
        --template ci-k8s-node-1.18.3 \
        --template-filter mine \
        --security-group k8s \
        --zone de-fra-1

    EXOSCALE_MASTER_IP=$(exo vm show "${EXOSCALE_MASTER_NAME}" --output-template "{{.IPAddress}}")
    export EXOSCALE_MASTER_IP

    sleep 30
}

initialize_k8s_master() {
    rsync -a "${INCLUDE_PATH}/" "ubuntu@${EXOSCALE_MASTER_IP}:/home/ubuntu" \
          -e "ssh -o StrictHostKeyChecking=no -i ${INTEGTEST_DIR}/.ssh/id_rsa"

    remote_run "$EXOSCALE_MASTER_IP" "sudo kubeadm init --config=./integtest/manifests/kubeadm-config-master.yml"
    remote_run "$EXOSCALE_MASTER_IP" "sudo cp --force /etc/kubernetes/admin.conf admin.conf && sudo chown ubuntu:ubuntu admin.conf"

    mkdir -p "${INTEGTEST_DIR}/.kube"
    scp -i "${INTEGTEST_DIR}/.ssh/id_rsa" \
           "ubuntu@${EXOSCALE_MASTER_IP}:/home/ubuntu/admin.conf" \
           "${INTEGTEST_DIR}/.kube/config"

    export KUBECONFIG="${INTEGTEST_DIR}/.kube/config"

    kubectl create -f https://docs.projectcalico.org/manifests/tigera-operator.yaml
    kubectl create -f https://docs.projectcalico.org/manifests/custom-resources.yaml

    kubectl wait "node/${EXOSCALE_MASTER_NAME}" --for=condition=Ready --timeout=180s
}

deploy_exoscale_ccm() {
    remote_run "$EXOSCALE_MASTER_IP" "git tag ci-dev && make docker"

    "${INCLUDE_PATH}/docs/scripts/generate-secret.sh"
    kubectl apply -f "${INTEGTEST_DIR}/manifests/deployment.yml"

    kubectl wait -n kube-system deployment.apps/exoscale-cloud-controller-manager --for=condition=available --timeout=180s
}

instancepool_join_k8s() {
    KUBE_TOKEN=$(remote_run "$EXOSCALE_MASTER_IP" "sudo kubeadm token create")
    export KUBE_TOKEN

    envsubst < "${INTEGTEST_DIR}/manifests/node-join-cloud-init.yaml" > "${INTEGTEST_DIR}/node-join-cloud-init.yml"
    EXOSCALE_INSTANCEPOOL_ID=$(exo instancepool create "${EXOSCALE_INSTANCEPOOL_NAME}" \
                        --keypair "${EXOSCALE_SSHKEY_NAME}" \
                        --template ci-k8s-node-1.18.3 \
                        --template-filter mine \
                        --size 1 \
                        --security-group k8s \
                        --service-offering medium \
                        --zone de-fra-1 \
                        --cloud-init "${INTEGTEST_DIR}/node-join-cloud-init.yml" \
                        --output-template "{{.ID}}" | tail -n 1)
    export EXOSCALE_INSTANCEPOOL_ID

    EXOSCALE_NODE_NAME=$(exo instancepool show "${EXOSCALE_INSTANCEPOOL_ID}" \
                --zone de-fra-1 --output-format json | jq -r '.instances[0]')
    export EXOSCALE_NODE_NAME

    until_success "kubectl get node \"${EXOSCALE_NODE_NAME}\""
    kubectl wait "node/${EXOSCALE_NODE_NAME}" --for=condition=Ready --timeout=180s
}

deploy_nginx_app() {
    kubectl apply -f "${INTEGTEST_DIR}/manifests/app.yml"

    kubectl wait deployment.apps/nginx --for=condition=Available --timeout=180s
}

create_external_loadbalancer() {
    envsubst < "${INTEGTEST_DIR}/manifests/create-nlb.yaml" > "${INTEGTEST_DIR}/create-nlb.yml"
    kubectl create -f "${INTEGTEST_DIR}/create-nlb.yml"

    until_success "exo nlb show \"${EXOSCALE_LB_NAME}\" --zone de-fra-1"
    until_success "exo nlb service show \"${EXOSCALE_LB_NAME}\" \"${EXOSCALE_LB_SERVICE_NAME1}\" --zone de-fra-1"
    sleep 10

    envsubst < "${INTEGTEST_DIR}/manifests/add-nlb-service.yaml" > "${INTEGTEST_DIR}/add-nlb-service.yml"
    kubectl create -f "${INTEGTEST_DIR}/add-nlb-service.yml"

    until_success "exo nlb service show \"${EXOSCALE_LB_NAME}\" \"${EXOSCALE_LB_SERVICE_NAME2}\" --zone de-fra-1"
    sleep 10
}

test_k8s_node_labels() {
    "${INTEGTEST_DIR}/test-labels.bash"
}

test_k8s_external_loadbalancer() {
    "${INTEGTEST_DIR}/test-nlb.bash"
}

main() {
    upload_integtest_sshkey
    create_exoscale_vm
    initialize_k8s_master
    deploy_exoscale_ccm
    instancepool_join_k8s
    deploy_nginx_app
    create_external_loadbalancer
    test_k8s_node_labels
    test_k8s_external_loadbalancer
}

main
