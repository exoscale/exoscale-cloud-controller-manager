# Exoscale Cloud Controller Manager

[![Actions Status](https://github.com/exoscale/exoscale-cloud-controller-manager/workflows/CI/badge.svg)](https://github.com/exoscale/exoscale-cloud-controller-manager/actions?query=workflow%3ACI)

`exoscale-cloud-controller-manager` is the Kubernetes [Cloud Controller
Manager][k8s-ccm] (CCM) implementation for Exoscale. This component enables a
tighter integration of Kubernetes clusters with the Exoscale Compute platform.

The Exoscale CCM implements the following [controllers][k8s-ccm-controllers]:

* Node controller: dynamically annotates Exoscale Compute instances registered
  as *Nodes* with platform-specific information (e.g. instance type, zone), and
  detects when a Compute instance previously registered as a k8s Node doesn't
  exist anymore and de-registers it from the cluster.
* Service controller: dynamically manages Exoscale [Network Load
  Balancers][exo-nlb-doc] with k8s *Services* of type
  [`LoadBalancer`][k8s-service-lb] to transparently forward traffic to k8s
  *Pods* running on  Compute Instance Pools-managed cluster *Nodes*.


## Getting Started

To get started with the Exoscale Cloud Controller Manager, please read the
[following guide](docs/getting-started.md).


## Contributing

* If you think you've found a bug in the code or you have a question regarding
  the usage of this software, please reach out to us by opening an issue in
  this GitHub repository.
* Contributions to this project are welcome: if you want to add a feature or a
  fix a bug, please do so by opening a Pull Request in this GitHub repository.
  In case of feature contribution, we kindly ask you to open an issue to
  discuss it beforehand.


[exo-nlb-doc]: https://community.exoscale.com/documentation/compute/network-load-balancer/
[k8s-ccm-controllers]: https://kubernetes.io/docs/concepts/architecture/cloud-controller/#functions-of-the-ccm
[k8s-ccm]: https://kubernetes.io/docs/concepts/architecture/cloud-controller/#functions-of-the-ccm
[k8s-service-lb]: https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer
