# Changelog

## 0.32.3

### Bug Fixes

* fix: malformed API endpoint

## 0.32.2

### Bug Fixes

* Add missing backward compatibility with egoscalev2 environment variables

## 0.32.1

### Bug Fixes

* csr validation: handle hostname case inconsistencies (#105)

### Improvements

* Egoscale v3 rewrite (#97)
* Rework the e2e test suite (#106)
* chore(golang): bump golang from 1.23 to 1.25 (#119)
* chore(deps): update busybox docker tag to v1.37.0 (#118)
* chore(deps): update alpine docker tag to v3.22 (#117)
* chore(deps): update actions/setup-go action to v6 (#116)
* fix(deps): update module github.com/google/go-cmp to v0.7.0 (#113)
* chore(deps): update module golang.org/x/oauth2 to v0.27.0 [security] (#109)
* fix(deps): update module github.com/stretchr/testify to v1.11.1 (#114)
* chore(deps): update actions/checkout action to v5 (#115)
* fix(deps): update module github.com/spf13/pflag to v1.0.10 (#112)
* chore(deps): update module golang.org/x/net to v0.38.0 [security] (#108)

## 0.32.0

### Improvements

* chore(deps): bump Kubernetes SDK from 1.31.0 to 1.32.6
* Add support for refering to name of load balancer instead of id
* Add support for refering to sks nodepool instead of instance pool id
* Add support for changing instance pool destination

## 0.30.1

* Add UDP Support to LoadBalancer service
* fix(instances): ensure internal IP is set when there is only public one

### Improvements

* fix(test): use lowercase instance name prefix in test suite
* chore(deps): bump Kubernetes SDK from 1.30.2 to 1.31.0
* chore(golang): bump golang from 1.22 to 1.23
* chore(doc): add note about make for macOS users
* go.mk: upgrade to v2.0.3 #89

## 0.30.0

### Improvements

* Update examples used in getting-started.md #82
* go.mk: lint with staticcheck #83
* Bump go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc from 0.42.0 to 0.46.0 #85
* Bump Go to 1.22.5
* Bump Kubernetes SDK from 1.29.1 to 1.30.2
* Bump golang/protobuf from 1.5.3 to 1.5.4
* Bump google/cel-go from 0.17.7 to 0.17.8
* Bump hashicorp/go-cleanhttp from 0.5.1 to 0.5.2
* Bump hashicorp/go-retryablehttp from 0.7.1 to 0.7.7
* Bump go.uber.org/zap from 1.19.0 to 1.26.0
* Bump golang.org/x/sync from 0.5.0 to 0.6.0
* Bump golang.org/x/sys from 0.18.0 to 0.20.0
* Remove go.uber.org/atomic

## 0.29.2

### Bug Fixes

* ccm-command: print returned error before exiting #81

## 0.29.1

### Improvements

* go.mk: provide alternative to submodule approach #77
* automate releasing with GH Actions (#73)
* Bump Kubernetes SDK to 1.29.0
* CCM is now built with Go 1.21

### Bug Fixes

* Fix missing hostname from the node list of addresses

## 0.29.0

### Improvements

* README: add new versioning and compatibility policy (#72)
* Bump golang.org/x/net from 0.13.0 to 0.17.0 (#69)
* Bump google.golang.org/grpc from 1.54.0 to 1.56.3 (#70)

## 0.14.1

### Bug Fixes

* Fix CSR approval for instance name with uppercase (#68)

## 0.14.0

* Upgrade Kubernetes SDK to 1.28.1, egoscale to 0.100.3

## 0.13.0

* Fix a rare bug happening on tests when trying to reload configuration
* Upgrade Kubernetes SDK to 1.27.1, egoscale to 0.100.1 and Go to 1.20

## 0.12.0

* Upgrade Kubernetes SDK to 1.25.0, egoscale to 0.90.0 and Go to 1.19

## 0.11.1

* Fix bug with manually deleted NLB

## 0.11.0

* Upgrade Kubernetes SDK to 1.24.1, egoscale to 0.88.1 and Go to 1.18

## 0.10.1

### Bug Fixes

* Fix `standard_init_linux.go:228: exec user process caused: permission denied` when launching container

## 0.10.0

### Features

* Add cloud-config (file) and instances override support
* Build with Go 1.17

## 0.9.0

### Features

* Add support for Kubernetes 1.23.x

## 0.8.1

### Bug Fixes

* Fix handling of alternative Exoscale API environments


## 0.8.0

### Features

* Add support for Kubernetes 1.22.x

### Bug Fixes

* Fix API errors when resetting NLB/NLB service description to an empty string


## 0.7.0

### Features

* Add support for `healthCheckNodePort` with NLB services


## 0.6.1

### Bug Fixes

* Fix external NLB Service management logic


## 0.6.0

### Features

* Add support for Exoscale SKS

### Changes

* An Exoscale zone is now required to be specified via the `EXOSCALE_ZONE`
  environment variable to the CCM. As a result, the `EXOSCALE_DEFAULT_ZONE` has
  been removed, and it is no longer necessary to specify the zone via manifest
  annotations for CCM-managed Kubernetes Services.


## 0.5.0

### Features

* Add support for API credentials configuration via local file


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
