# Deploy k8s cluster with kubeadm for Exoscale cloud controller

## WIP documentation

## Install

Follow the instructions in the k8s documentation
https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/create-cluster-kubeadm/

When reaching the control-plane node initialization step (kubeadm init <args>), replace arguments with `--config=kubeadm-config.yml`:
```Shell
sudo kubeadm init --config=kubeadm-config.yml
```
