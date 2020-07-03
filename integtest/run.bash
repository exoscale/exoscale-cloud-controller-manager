#!/bin/bash

set -e

export INTEGTEST_DIR="$INCLUDE_PATH/integtest"

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
EXOSCALE_LB_SERVICE_NAME=k8s-ccm-lb-service-$(uuidgen | tr '[:upper:]' '[:lower:]')
export EXOSCALE_LB_SERVICE_NAME

until_success() {
    COMMAND=$1
    timeout 3m bash -c "until $COMMAND &>/dev/null; do sleep 5; done"
}

cleanup() {
    rm -rf "$INTEGTEST_DIR/.ssh"
    rm -rf "$INTEGTEST_DIR/.kube"
    rm -rf "$INTEGTEST_DIR/create-nlb.yml"
    rm -rf "$INTEGTEST_DIR/update-nlb.yml"
    rm -rf "$INTEGTEST_DIR/node-join-cloud-init.yml"
    exo -Q vm delete -f "$EXOSCALE_MASTER_NAME"
    exo -Q nlb delete -f "$EXOSCALE_LB_NAME" -z de-fra-1 &>/dev/null || true
    until_success "exo -Q instancepool delete -f \"$EXOSCALE_INSTANCEPOOL_NAME\" -z de-fra-1"
    until_success "exo -Q sshkey delete -f \"$EXOSCALE_SSHKEY_NAME\""
}
trap cleanup EXIT

echo "Start K8S Cluster"

mkdir -p "$INTEGTEST_DIR/.ssh"
ssh-keygen -t rsa -f "$INTEGTEST_DIR/.ssh/id_rsa" -N ""

exo -Q sshkey upload "$EXOSCALE_SSHKEY_NAME" "$INTEGTEST_DIR/.ssh/id_rsa.pub"

exo -Q vm create "$EXOSCALE_MASTER_NAME" \
           -k "$EXOSCALE_SSHKEY_NAME" \
           -t ci-k8s-node-1.18.3 \
           --template-filter mine \
           -s k8s \
           -z de-fra-1

EXOSCALE_MASTER_IP=$(exo vm show "$EXOSCALE_MASTER_NAME" -O json | jq -r '.ip_address')
export EXOSCALE_MASTER_IP

sleep 30

remote_run() {
    ssh -i "$INTEGTEST_DIR/.ssh/id_rsa" "ubuntu@$EXOSCALE_MASTER_IP" "$1"
}

rsync -a "$INCLUDE_PATH/" "ubuntu@$EXOSCALE_MASTER_IP:/home/ubuntu" \
      -e "ssh -o StrictHostKeyChecking=no -i $INTEGTEST_DIR/.ssh/id_rsa"

remote_run "sudo kubeadm init --config=./docs/kubeadm/kubeadm-config-master.yml"
remote_run "sudo cp -f /etc/kubernetes/admin.conf admin.conf && sudo chown ubuntu:ubuntu admin.conf"

mkdir -p "$INTEGTEST_DIR/.kube"
scp -i "$INTEGTEST_DIR/.ssh/id_rsa" \
       "ubuntu@$EXOSCALE_MASTER_IP:/home/ubuntu/admin.conf" \
       "$INTEGTEST_DIR/.kube/config"

export KUBECONFIG="$INTEGTEST_DIR/.kube/config"

kubectl apply -f https://docs.projectcalico.org/v3.14/manifests/calico.yaml

kubectl wait node/"$EXOSCALE_MASTER_NAME" --for=condition=Ready --timeout=180s

remote_run "git tag ci-dev && make docker"

"$INCLUDE_PATH/deployment/secret.sh"
kubectl apply -f "$INTEGTEST_DIR/deployment.yml"

kubectl wait -n kube-system deployment.apps/exoscale-cloud-controller-manager --for=condition=available --timeout=180s

KUBE_TOKEN=$(remote_run "sudo kubeadm token create")
export KUBE_TOKEN

envsubst < "$INTEGTEST_DIR/node-join-cloud-init.yaml" > "$INTEGTEST_DIR/node-join-cloud-init.yml"
EXOSCALE_INSTANCEPOOL_ID=$(exo instancepool create "$EXOSCALE_INSTANCEPOOL_NAME" \
                        -k "$EXOSCALE_SSHKEY_NAME" \
                        -t ci-k8s-node-1.18.3 \
                        --template-filter mine \
                        --size 1 \
                        -s k8s \
                        -o medium \
                        -z de-fra-1 \
                        -c "$INTEGTEST_DIR"/node-join-cloud-init.yml --output-template "{{.ID}}" | tail -n 1)
export EXOSCALE_INSTANCEPOOL_ID

EXOSCALE_NODE_NAME=$(exo instancepool show "$EXOSCALE_INSTANCEPOOL_ID" -z de-fra-1 -O json | jq -r '.instances[0]')
export EXOSCALE_NODE_NAME

until_success "kubectl get node \"$EXOSCALE_NODE_NAME\""
kubectl wait node/"$EXOSCALE_NODE_NAME" --for=condition=Ready --timeout=180s

echo "Test k8s Nodes Labels"

"$INTEGTEST_DIR/test-labels.bash"

echo "Deploy nginx app"

kubectl apply -f "$INTEGTEST_DIR/app.yml"

kubectl wait deployment.apps/nginx --for=condition=Available --timeout=180s

echo "Create k8s External LoadBalancer"

envsubst < "$INTEGTEST_DIR/create-nlb.yaml" > "$INTEGTEST_DIR/create-nlb.yml"
kubectl create -f "$INTEGTEST_DIR/create-nlb.yml"

until_success "exo nlb show \"$EXOSCALE_LB_NAME\" -z de-fra-1"
until_success "exo nlb service show \"$EXOSCALE_LB_NAME\" \"$EXOSCALE_LB_SERVICE_NAME\" -z de-fra-1"

echo "Test k8s External LoadBalancer"

"$INTEGTEST_DIR/test-nlb.bash"