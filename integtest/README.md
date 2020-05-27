# Running integration test locally

## Prerequisites

Make sure you have the following programs installed:

* [Exoscale CLI](https://github.com/exoscale/cli/releases)
* [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)

Export your Exoscale credentials as described in the main [README file](https://github.com/exoscale/exoscale-cloud-controller-manager#setup-your-secrets).

Register the K8S CI Custom Template in **DE-FRA-1**

* name `ci-k8s-node-1.18.3`
* URL https://sos-ch-dk-2.exo.io/eat-templates/ci-k8s-node-1.18.3.qcow2
* Checksum `6cff454cda4d4845d5bb34398366a5fc`
* Login Username `ubuntu`

## Execute integration tests

```Shell
make integtest
```
