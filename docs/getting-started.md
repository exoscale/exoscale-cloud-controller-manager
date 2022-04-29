# Getting Started

## Installation

### Using official Docker images (recommended)

The recommended installation method is to use official [Docker
images][docker-hub] in your Kubernetes manifests.


### Compile from sources

If you wish to compile the Exoscale Cloud Controller Manager (CCM) from
sources, run the following command at the root of the sources:

```
make build
```

Upon successful compilation, the resulting `exoscale-cloud-controller-manager`
binary is stored in the `bin/` directory.

If you want to create a Docker image from the sources, run the following
command:

```
make docker
```

Upon successful build, the resulting local image is
`exoscale/cloud-controller-manager:latest`.


## Configuration

> Note: the following guide assumes you have the permissions to create
> resources in the `kube-system` namespace of the target Kubernetes cluster.

In order to interact with the Exoscale API, the Exoscale CCM must be configured
with API credentials. This can be achieved:
* using Kubernetes [*Secrets*][k8s-secrets], by exposing those as container
  environment variables
* as a separate (JSON) credentials file, which will be watched for changes
  (allowing API credentials to be dymically set/refreshed)
* using the (YAML) configuration file
  (`--cloud-config`; see below for further details)

### Using Kubernetes Secrets

The following environment variables are available to configure Exoscale CCM:

* `EXOSCALE_ZONE` [**required**]: the Exoscale zone which the cluster/CCM
  runs in; e.g. `ch-gva-2`

* `EXOSCALE_API_KEY` / `EXOSCALE_API_SECRET` [**required**]: actual Exoscale API
  credentials

* `EXOSCALE_API_CREDENTIALS_FILE` [**optional**]: Exoscale API
  credentials (JSON) file; see further below for its format.
  _Ignored if actual credentials are provided_

Which may be passed to the CCM container thanks to Kubernetes [Secrets][k8s-secrets]

#### Helper script

We provide a convenience script that generates and applies a k8s manifest
declaring Exoscale API credentials as a k8s *Secret* in your cluster from your
local shell environment variables: once created, this *Secret* can be used in
the CCM *Deployment*.

First, start by exporting the Exoscale API credentials (we recommend that you
create dedicated API credentials using the [Exoscale IAM][exo-iam] service) to
provide to the CCM in your shell, as well as the zone in which the cluster is
located:

```Shell
export EXOSCALE_API_KEY="EXOxxxxxxxxxxxxxxxxxxxxxxxx"
export EXOSCALE_API_SECRET="xxxxxxxxx-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
export EXOSCALE_ZONE="ch-gva-2"
```

Next, run the following command from the same shell:

```
./docs/scripts/generate-secret.sh
```

Finally, ensure that the `exoscale-secret` *Secret* has been created
successfully by running the following command:

```
kubectl get secret --namespace kube-system exoscale-credentials
```

### Using the Cloud Configuration File (--cloud-config)

Exoscale CCM may be configured using the so-called (optional) Cloud Configuration File,
specified by the `--cloud-config <path>` option when launching the CCM container.

This file must be YAML-formatted and provides the following parameters (which may used _instead_
of the environment variables mentioned above):

``` yaml
# Global (API) configuration
global:
  zone: "<EXOSCALE_ZONE>"
  apiKey: "<EXOSCALE_API_KEY>"
  apiSecret: "<EXOSCALE_API_SECRET>"
  apiCredentialsFile: "<EXOSCALE_API_CREDENTIALS_FILE>"
```

#### Overrides

The configuration files also allows to statically override (Exoscale API-derived) Instances
metadata or support external Nodes:

``` yaml
instances:
  overrides:
  - name: "some-node"
    addresses:
    - type: "InternalIP"
      address: "192.0.2.123"
    region: "some-alternate-region"
  - name: "/^some-external-/"
    external: true
    type: "some-external-type"
    region: "some-external-region"
```

The following parameters are available to configure overrides:

* `name` [string, **required**]: an individual node name or a `/.../`-specified regular expression
  to match node(s) name(s)

* `external` [boolean, optional]: whether the node/instance is managed
  by the Exoscale API (`false`; the default) or not (`true`)

* `externalID` [string, optional]: the external node/instance ID; ignored if `external=false`.
  Defaults to `external-<sha256sum(name)>` if unspecified.

* `type` [string, optional]: the node/instance Type (overrides the API-provided one).
  Defaults to `external` if unspecified and `external=true`.

* `addresses` [list, optional]: the node/instance addresses (overrides the API-provided one)

  - `type` [string]: the address (Kubernetes `v1.NodeAddressType`) type: _Hostname_, _ExternalIP_, _InternalIP_, _ExternalDNS_, _InternalDNS_

  - `address` [string]: the actual IP address or host name

  If unspecified (empty) and `external=true`, Kubernetes will keep the Kubelet-registered
  _InternalIP_ address "as is".

* `region`: the node/instance Region (does _not_ override the API-provided one).
  Defaults to `external` if unspecified and `external=true`.

### Using API Credentials File

Exoscale API credentials may be dynamically set/refreshed using a
JSON-formatted file:

``` json
{
    "api_key": "EXO<key>",
    "api_secret": "<secret>"
}
```

Which path is specified using the `EXOSCALE_API_CREDENTIALS_FILE` environment variable
or `apiCredentialsFile` Cloud Configuration File parameter.

### Deploying the Exoscale Cloud Controller Manager

> Please first read the official Kubernetes documentation relating to [Cloud
> Controller Managers administration][k8s-ccm-admin] to learn how your cluster
> nodes must be configured to leverage an external Cloud Controller Manager.

To deploy the Exoscale CCM on your Kubernetes cluster, you can use the manifest
provided as example:

```
kubectl apply -f ./docs/examples/cloud-controller-manager.yml
```

To ensure the CCM deployment is successful, run the following command and check
that there is a pod running:

```
kubectl get pods \
    --namespace kube-system \
    --selector app=exoscale-cloud-controller-manager
```


### Usage

You can find in the `./docs/examples/` directory manifests files  illustrating
how to leverage the Exoscale Cloud Controller Manager's integration with the
Exoscale platform.


#### Kubernetes Services with Exoscale Network Load Balancers

You can find out how to use the Exoscale CCM to load balance your Kubernetes
*Services* using Exoscale Network Load Balancers [in this
guide][doc-service-loadbalancer].


[doc-service-loadbalancer]: ./service-loadbalancer.md
[docker-hub]: https://hub.docker.com/repository/docker/exoscale/cloud-controller-manager
[exo-iam]: https://community.exoscale.com/documentation/iam/quick-start/
[exo-sg]: https://community.exoscale.com/documentation/compute/security-groups/
[k8s-ccm-admin]: https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/#cloud-controller-manager
[k8s-secrets]: https://kubernetes.io/docs/concepts/configuration/secret/
[k8s-service-nodeport]: https://kubernetes.io/docs/concepts/services-networking/service/#nodeport
