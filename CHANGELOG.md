# Changelog

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
