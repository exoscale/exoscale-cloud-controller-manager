# Exoscale Cloud Controller Manager

[![Actions Status](https://github.com/exoscale/exoscale-cloud-controller-manager/workflows/CI/badge.svg)](https://github.com/exoscale/exoscale-cloud-controller-manager/actions?query=workflow%3ACI)

`exoscale-cloud-controller-manager` is the Kubernetes cloud controller manager implementation for Exoscale.
Read more about cloud controller managers [here](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/).
Running `exoscale-cloud-controller-manager` allows you to leverage many of the cloud provider features offered by Exoscale on your kubernetes clusters.

## Getting Started

### Prerequisite (optional)

Learn more about how to bootstrap a k8s for Exoscale cloud controller manager [here](./doc/kubeadm)!

### Setup your secrets

Export your Exoscale credentials in your shell.

```Shell
export EXOSCALE_API_KEY=EXO...
export EXOSCALE_API_SECRET=XXX...
export EXOSCALE_API_ENDPOINT="https://api.exoscale.com/v1"
```

then apply 
```Shell
./deployment/secret.sh
```

### Deploy Exoscale Cloud Controller Manager

```Shell
kubectl apply -f ./deployment/deployment.yml
```
