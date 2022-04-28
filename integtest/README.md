# Running integration test locally

## Prerequisites

Make sure you have the following programs installed:

* [Terraform](https://www.terraform.io)
* [Exoscale CLI](https://github.com/exoscale/cli/releases)
* [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)

Export your Exoscale credentials as described in the main [README file](https://github.com/exoscale/exoscale-cloud-controller-manager#setup-your-secrets).

## Execute integration tests

```Shell
make test-integration
```

## Refresh tests resources

```Shell
# Calico
wget https://projectcalico.docs.tigera.io/manifests/calico.yaml -O manifests/calico.yml

# NGINX ingress
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
helm template --namespace ingress-nginx ingress-nginx ingress-nginx/ingress-nginx > manifests/ingress-nginx.yml
```
