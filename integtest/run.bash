#!/bin/bash

set -e

INTEGTEST_DIR="$INCLUDE_PATH/integtest"

[[ -n "$EXOSCALE_API_KEY" ]]
[[ -n "$EXOSCALE_API_SECRET" ]]
[[ -n "$EXOSCALE_API_ENDPOINT" ]]

EXOSCALE_INSTANCE_NAME=ci-ccm-$(uuidgen)
export EXOSCALE_INSTANCE_NAME

cleanup() {
    exo -Q sshkey delete -f "$EXOSCALE_INSTANCE_NAME"
    exo -Q vm delete -f "$EXOSCALE_INSTANCE_NAME"
    rm -rf "$INTEGTEST_DIR/.ssh"
    rm -rf "$INTEGTEST_DIR/.kube"
}
trap cleanup EXIT

echo "Start K8S Cluster"

mkdir -p "$INTEGTEST_DIR/.ssh"
ssh-keygen -t rsa -f "$INTEGTEST_DIR/.ssh/id_rsa" -N ""

exo -Q sshkey upload "$EXOSCALE_INSTANCE_NAME" "$INTEGTEST_DIR/.ssh/id_rsa.pub"

exo -Q vm create "$EXOSCALE_INSTANCE_NAME" \
           -k "$EXOSCALE_INSTANCE_NAME" \
           -t ci-k8s-node-1.18.3 \
           --template-filter mine \
           -z de-fra-1

EXOSCALE_INSTANCE_IP=$(exo vm show "$EXOSCALE_INSTANCE_NAME" -O json | jq -r '.ip_address')

sleep 30

remote_run() {
    ssh -i "$INTEGTEST_DIR/.ssh/id_rsa" "ubuntu@$EXOSCALE_INSTANCE_IP" "$1"
}

rsync -a "$INCLUDE_PATH/" "ubuntu@$EXOSCALE_INSTANCE_IP:/home/ubuntu" \
      -e "ssh -o StrictHostKeyChecking=no -i $INTEGTEST_DIR/.ssh/id_rsa"

remote_run "sudo kubeadm init --config=./doc/kubeadm/kubeadm-config.yml"
remote_run "sudo cp -f /etc/kubernetes/admin.conf admin.conf && sudo chown ubuntu:ubuntu admin.conf"

mkdir -p "$INTEGTEST_DIR/.kube"
scp -i "$INTEGTEST_DIR/.ssh/id_rsa" \
       "ubuntu@$EXOSCALE_INSTANCE_IP:/home/ubuntu/admin.conf" \
       "$INTEGTEST_DIR/.kube/config"

KUBECONFIG="$INTEGTEST_DIR/.kube/config"
export KUBECONFIG

kubectl apply -f https://docs.projectcalico.org/v3.14/manifests/calico.yaml

remote_run "git tag ci-dev && make docker"
    
"$INCLUDE_PATH/deployment/secret.sh"
kubectl apply -f "$INTEGTEST_DIR/deployment.yml"

sleep 10

echo "Run CCM Integration Test"

"$INTEGTEST_DIR/test.bash"
