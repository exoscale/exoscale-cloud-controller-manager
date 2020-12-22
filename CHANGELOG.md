# Changelog

## 0.4.0

### Features

* Add support for externally managed NLB instances

### Changes

* Improve Load Balancer Instance Pool detection logic: an error will be
  returned if multiple Instance Pools are detected across the cluster Nodes and
  that no Instance Pool ID is specified in the Kubernetes *Service* annotations
* Docker image is now based on busybox


## 0.3.0

### Bug Fixes

* Fix provider ID formatting logic

### Changes

* The Exoscale Cloud Controller Manager now Supports multiple `ServicePorts`
  per Kubernetes `Service`, previous NLB service-related annotations are
  obsolete: please refer to the documentation for more information.


## 0.2.0

### Features

* New `EXOSCALE_DEFAULT_ZONE` configuration environment variable available to
  set a default Exoscale zone to be used for API operations when none specified
  in Kubernetes manifests annotations.


## 0.1.0

Initial release
