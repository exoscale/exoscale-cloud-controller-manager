# Kubernetes Services with Exoscale Network Load Balancers

This guide explains how to use the Exoscale Cloud Controller Manager (CCM) to
create Kubernetes *Services* of type `LoadBalancer`.

> Note: it is assumed that you have a functional Kubernetes cluster running the
> Exoscale CCM.

When you create a Kubernetes *Service* of type `LoadBalancer`, the Exoscale CCM
provisions an Exoscale [Network Load Balancer][exo-nlb] (NLB) instance, on
which it will create one [NLB *service*][exo-nlb-svc] for every
[`ServicePort`][k8s-serviceport-spec] entry defined Kubernetes declared in the
[*Service*][k8s-service-spec] manifest.


## Prerequisites

The Exoscale CCM service controller only supports managing load balancing to
Kubernetes *Pods* running on *Nodes* managed by Exoscale Instance Pools. We
strongly recommend that you build a [custom Compute instance
template][custom-templates] that is usable by an Instance Pool, for example to
automatically have the new members join your Kubernetes cluster as *Nodes*.


## Configuration

When the Exoscale Cloud Controller Manager is deployed and configured in a
Kubernetes cluster, creating a *Service* of type `LoadBalancer` will
automatically create an Exoscale Network Load Balancer (NLB) instance and
configured with a service listening on every port defined in the Kubernetes
*Service* `ports` spec.

The following manifest illustrates the minimal configuration for exposing a
Kubernetes *Service* via an Exoscale NLB:

```yaml
kind: Service
apiVersion: v1
metadata:
  name: nginx
spec:
  selector:
    app: nginx
  type: LoadBalancer
  ports:
  - port: 80
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  selector:
    matchLabels:
      app: nginx
  replicas: 2
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginxdemos/hello:latest
        ports:
        - containerPort: 80
```

The Exoscale CCM will create an Exoscale NLB instance containing a service
forwarding network traffic received on port 80 to the port 80 of the pods
matching the `app: nginx` selector.


### Annotations 

In addition to the standard Kubernetes [`Service`][k8s-service-spec] object
specifications, the behavior of the Exoscale CCM service node is configurable
by adding annotations in the Kubernetes `Service` object's `annotations` map.
The following annotations are supported (annotations marked by a __*__ are
required):


#### `service.beta.kubernetes.io/exoscale-loadbalancer-id`

The ID of the Exoscale NLB corresponding to the Kubernetes *Service*. This
annotation is set automatically by the Exoscale CCM after having created the
NLB instance if one was not specified (see section *Using an externally
managed NLB instance with the Exoscale CCM*).


#### `service.beta.kubernetes.io/exoscale-loadbalancer-name`

The name of the Exoscale NLB. Defaults to `<Kubernetes Service UID>`.

You can also set it for using an externally managed NLB instance instead of
`exoscale-loadbalancer-id` (see section *Using an externally
managed NLB instance with the Exoscale CCM*).


#### `service.beta.kubernetes.io/exoscale-loadbalancer-description`

The description of the Exoscale NLB.


#### `service.beta.kubernetes.io/exoscale-loadbalancer-external`

If set to `true`, the Exoscale CCM will consider the NLB as externally
managed and will not attempt to create/update/delete the NLB instance
whose ID or Name is specified in the K8s *Service* annotations.


#### `service.beta.kubernetes.io/exoscale-loadbalancer-service-name`

The name of Exoscale NLB service corresponding to the Kubernetes *Service*
port. Defaults to `<Kubernetes Service UID>-<Service port>`.

> Note: this annotation is only honored if a single port is defined in the
> Kubernetes *Service*, and is set to the default value otherwise.


#### `service.beta.kubernetes.io/exoscale-loadbalancer-service-description`

The description of the Exoscale NLB service corresponding to the Kubernetes
*Service*.

> Note: this annotation is only honored if a single port is defined in the
> Kubernetes *Service*.


#### `service.beta.kubernetes.io/exoscale-loadbalancer-service-instancepool-id`

The ID of the Exoscale Instance Pool to forward ingress traffic to. Defaults to
the Instance Pool ID of the cluster *Nodes* ; this information must be
specified in case your *Service* is targeting *Pods* that are subject to
[custom *Node* scheduling][k8s-assign-pod-node].

### `service.beta.kubernetes.io/exoscale-loadbalancer-service-sks-nodepool-name`

Can be used instead of `exoscale-loadbalancer-service-instancepool-id` for pointing
the service to an instance pool. The name of a SKS nodepool must be used then.

`exoscale-loadbalancer-service-instancepool-id` will be then automatically set
with its ID.

When using this you have to specify the sks clustername in the annotation below.

#### `service.beta.kubernetes.io/exoscale-sks-cluster-name`

This is a requirement for
`service.beta.kubernetes.io/exoscale-loadbalancer-service-sks-nodepool-name`

Otherwise this annotation is not needed.

#### `service.beta.kubernetes.io/exoscale-loadbalancer-service-strategy`

The Exoscale NLB Service strategy to use.

Supported values: `round-robin` (default), `source-hash`.

> Note: because Exoscale Network Load Balancers dispatch network traffic across
> Compute instances in the specified Instance Pool (i.e. Kubernetes Nodes), if
> you run multiple replicas of *Pods* spread on several *Nodes* the load
> balancing might be less evenly distributed across all containers – as the
> [`kube-proxy`][k8s-service-kube-proxy] also performs *Node*-local load
> balancing on pods belonging to a same *Deployment*. Similarly, using the
> `source-hash` strategy is not guaranteed to always forward traffic from a
> client *source IP address/port/protocol* tuple to the same container.


#### `service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-mode`

The Exoscale NLB service health checking mode.

Supported values: `tcp` (default), `http`.

#### `service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-port`

Forces an healthcheck port.

Default is `NodePort` of the service when `spec.ExternalTrafficPolicy` is set to `Cluster` (default) or
`spec.HealthCheckNodePort` when `spec.ExternalTrafficPolicy` is set to `Local`.


#### `service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-port`

Forces an healthcheck port.

Default is `NodePort` of the service when `spec.ExternalTrafficPolicy` is set to `Cluster` (default) or
`spec.HealthCheckNodePort` when `spec.ExternalTrafficPolicy` is set to `Local`.


#### `service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-uri`

The Exoscale NLB service health check HTTP request URI (in `http` mode only).


#### `service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-interval`

The Exoscale NLB service health checking interval in seconds. Defaults to
`10s`.


#### `service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-timeout`

The Exoscale NLB service health checking timeout in seconds. Defaults to `5s`.


#### `service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-retries`

The Exoscale NLB service health checking retries before considering a target
*down*. Defaults to `1`.


### Using a Kubernetes Ingress Controller behind an Exoscale NLB

If you wish to expose a Kubernetes [Ingress Controller][k8s-ingress-controller]
(such as the popular [ingress-nginx][ingress-nginx]), or any other *Service*
behind an Exoscale NLB, please note that by default the traffic is forwarded
to all [healthy] Nodes in the destination Instance Pool, whether they actually
host *Pods* targeted by the *Service* or not – which may result in additional
hops inside the Kubernetes cluster, as well as losing the source IP address
(source NAT).

According to the [Kubernetes documentation][k8s-service-source-ip], it is
possible to set the value of the *Service* `spec.externalTrafficPolicy` to
`Local`, which preserves the client source IP and avoids a second hop, but
risks potentially imbalanced traffic spreading. In this configuration, the
Exoscale CCM will configure managed NLB services to use the *Service*
`spec.healthCheckNodePort` value for the NLB service healthcheck port, which
will result in having the ingress traffic forwarded only to *Nodes* running
the target *Pods*. With `spec.externalTrafficPolicy=Cluster` (the default),
the CCM uses `spec.ports[].nodePort`.

### Configuring a UDP service

When pointing the Exoscale NLB to a UDP service, it still requires a TCP health
check port to determine if the respective node or application is reachable.
You can use a sidecar container to provide this functionality.

Also make sure to allow UDP for the NodePort ports in your security group.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: udp-echo-deployment
  labels:
    app: udp-echo
spec:
  replicas: 2
  selector:
    matchLabels:
      app: udp-echo
  template:
    metadata:
      labels:
        app: udp-echo
    spec:
      containers:
      - name: udp-echo
        image: alpine
        command: ["sh", "-c", "while true; do echo -n 'Echo' | nc -u -l -p 8080 -w 1; done"]
        ports:
        - containerPort: 8080
          protocol: UDP
      - name: tcp-healthcheck
        image: k8s.gcr.io/echoserver:1.10
        ports:
        - containerPort: 8080
          protocol: TCP
---
apiVersion: v1
kind: Service
metadata:
  name: udp-echo-service
  annotations:
  # If you use externalTrafficPolicy: Cluster (default) you either have to use the same nodePort for both the
  #    TCP (healthcheck) and UDP service or define here a healthcheck port which needs to be the same
  #    as the TCP (healthcheck)'s NodePort
  # If you use externalTrafficPolicy: local remove that annotation, it will use spec.healthCheckNodePort then
    service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-port: "31621"
spec:
  type: LoadBalancer
  # externalTrafficPolicy: Local
  ports:
    - name: udpapplication
      port: 8080                 # External UDP Port
      protocol: UDP
      targetPort: 8080           # ContainerPort
    # You can remove the following service + sidecar container when using externalTrafficPolicy: Local
    - name: udpapphealthcheck
      port: 8080                 # Health check port (TCP)
      targetPort: 8080           # ContainerPort for TCP health checks
      nodePort: 31621            # Must match the annotation above
      protocol: TCP
  selector:
    app: udp-echo

```

**Notes:**

* A [long-standing bug][k8s-same-port-bug] in kubectl prevents adding the same
  port with different protocols (e.g., TCP and UDP) to an already existing service
  with kubectl apply, kubectl edit, or similar commands. If you attempt this,
  Kubernetes may delete the respective second port.  
  Use `kubectl apply --server-side` to avoid or fix this issue.
* If `externalTrafficPolicy: Local` is used, the CCM will automatically assign
  `spec.healthCheckNodePort` (which checks wether a given node is online and holds endpoint for the service),
  so the explicit annotation for the health check port and the sidecar is unnecessary.  
  For externalTrafficPolicy: Cluster, ensure the annotation
  `service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-port` matches the TCP `nodePort`.

### Configuring a UDP service

When pointing the Exoscale NLB to a UDP service, it still requires a TCP health
check port to determine if the respective node or application is reachable.
You can use a sidecar container to provide this functionality.

Also make sure to allow UDP for the NodePort ports in your security group.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: udp-echo-deployment
  labels:
    app: udp-echo
spec:
  replicas: 2
  selector:
    matchLabels:
      app: udp-echo
  template:
    metadata:
      labels:
        app: udp-echo
    spec:
      containers:
      - name: udp-echo
        image: alpine
        command: ["sh", "-c", "while true; do echo -n 'Echo' | nc -u -l -p 8080 -w 1; done"]
        ports:
        - containerPort: 8080
          protocol: UDP
      - name: tcp-healthcheck
        image: k8s.gcr.io/echoserver:1.10
        ports:
        - containerPort: 8080
          protocol: TCP
---
apiVersion: v1
kind: Service
metadata:
  name: udp-echo-service
  annotations:
  # If you use externalTrafficPolicy: Cluster (default) you either have to use the same nodePort for both the
  #    TCP (healthcheck) and UDP service or define here a healthcheck port which needs to be the same
  #    as the TCP (healthcheck)'s NodePort
  # If you use externalTrafficPolicy: local remove that annotation, it will use spec.healthCheckNodePort then
    service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-port: "31621"
spec:
  type: LoadBalancer
  # externalTrafficPolicy: Local
  ports:
    - name: udpapplication
      port: 8080                 # External UDP Port
      protocol: UDP
      targetPort: 8080           # ContainerPort
    # You can remove the following service + sidecar container when using externalTrafficPolicy: Local
    - name: udpapphealthcheck
      port: 8080                 # Health check port (TCP)
      targetPort: 8080           # ContainerPort for TCP health checks
      nodePort: 31621            # Must match the annotation above
      protocol: TCP
  selector:
    app: udp-echo

```

**Notes:**

* A [long-standing bug][k8s-same-port-bug] in kubectl prevents adding the same
  port with different protocols (e.g., TCP and UDP) to an already existing service
  with kubectl apply, kubectl edit, or similar commands. If you attempt this,
  Kubernetes may delete the respective second port.  
  Use `kubectl apply --server-side` to avoid or fix this issue.
* If `externalTrafficPolicy: Local` is used, the CCM will automatically assign
  `spec.healthCheckNodePort` (which checks wether a given node is online and holds endpoint for the service),
  so the explicit annotation for the health check port and the sidecar is unnecessary.  
  For externalTrafficPolicy: Cluster, ensure the annotation
  `service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-port` matches the TCP `nodePort`.

### Using an externally managed NLB instance with the Exoscale CCM

If you prefer to manage the NLB instance yourself using different tools
(e.g. [Terraform][exo-tf-provider]), you can specify the ID or Name of the NLB instance
to use in the K8s *Service* annotations as well as an annotation instructing
the Exoscale CCM not to create/update/delete the specified NLB instance:

```yaml
kind: Service
apiVersion: v1
metadata:
  name: nginx
  annotations:
    service.beta.kubernetes.io/exoscale-loadbalancer-id: "09191de9-513b-4270-a44c-5aad8354bb47"
    # or (name takes precedence over id if both specified)
    # service.beta.kubernetes.io/exoscale-loadbalancer-name: "my-exoscale-loadbalancer"
    service.beta.kubernetes.io/exoscale-loadbalancer-external: "true"
spec:
  selector:
    app: nginx
  type: LoadBalancer
  ports:
  - port: 80
```

**Notes:**

* The NLB instance referenced in the annotations **must** exist before
  the K8s *Service* is created.
* When deploying a K8s Service to an external NLB, be careful not to use a
  *Service* port already used by another *Service* attached to the same
  external NLB, as **it will overwrite the existing NLB Service with the new
  K8s Service port**.


## ⚠️ Important Notes

* As `NodePort` created by K8s *Services* are picked randomly [within a defined
  range][k8s-service-nodeport] (by default `30000-32767`), don't forget to
  configure [Security Groups][exo-sg] used by your Compute Instance Pools to
  accept ingress traffic in this range, otherwise the Exoscale Network Load
  Balancers won't be able to forward traffic to your *Pods*.


[custom-templates]: https://community.exoscale.com/documentation/compute/custom-templates/#create-a-custom-template
[exo-nlb-svc]: https://community.exoscale.com/documentation/compute/network-load-balancer/#network-load-balancer-services
[exo-nlb]: https://community.exoscale.com/documentation/compute/network-load-balancer/
[exo-tf-provider]: https://registry.terraform.io/providers/exoscale/exoscale/latest/docs
[exo-sg]: https://community.exoscale.com/documentation/compute/security-groups/
[ingress-nginx]: https://kubernetes.github.io/ingress-nginx/
[k8s-assign-pod-node]: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/
[k8s-ingress-controller]: https://kubernetes.io/docs/concepts/services-networking/ingress-controllers/
[k8s-service-kube-proxy]: https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies
[k8s-service-nodeport]: https://kubernetes.io/docs/concepts/services-networking/service/#nodeport
[k8s-service-source-ip]: https://kubernetes.io/docs/tutorials/services/source-ip/#source-ip-for-services-with-type-loadbalancer
[k8s-service-spec]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#service-v1-core
[k8s-serviceport-spec]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#serviceport-v1-core
[k8s-same-port-bug]: https://github.com/kubernetes/kubernetes/issues/105610
