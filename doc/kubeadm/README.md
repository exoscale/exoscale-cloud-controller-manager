# Deploy k8s cluster with kubeadm for Exoscale cloud controller

## WIP documentation

## Install

follow the installation on k8s documentation
https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/create-cluster-kubeadm/

at the step of `kubeadm init`

replace by this operation with this file `kubeadm-config.yml`:
```Shell
sudo kubeadm init --config=kubeadm-config.yml
```
