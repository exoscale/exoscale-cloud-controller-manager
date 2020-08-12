# Kubernetes Services with Exoscale Network Load Balancers

This guide explains how to use the Exoscale Cloud Controller Manager (CCM) to
create Kubernetes *Services* of type `LoadBalancer`.

> Note: it is assumed that you have a functional Kubernetes cluster running the
> Exoscale CCM.

When you create a Kubernetes *Service* of type `LoadBalancer`, the CCM
provisions an Exoscale [Network Load Balancer][exo-nlb] (NLB) instance, on
which it will create one [NLB *service*][exo-nlb-svc] corresponding to the
port defined Kubernetes [*Service*][k8s-serviceport-spec] declared in the
manifest.


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
configured with a service listening on the port defined in the Kubernetes
*Service* `ports` spec.

The following manifest illustrates the minimal configuration for exposing a
Kubernetes *Service* via an Exoscale NLB:

```yaml
kind: Service
apiVersion: v1
metadata:
  name: nginx
  annotations:
    service.beta.kubernetes.io/exoscale-loadbalancer-zone: "ch-gva-2"
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


#### `service.beta.kubernetes.io/exoscale-loadbalancer-zone`*

The Exoscale [zone][exo-zones] in which to create the Network Load Balancer
instance.

> Note: a CCM-managed Network Load Balancer must be located in the same zone as
> the Kubernetes Nodes it must forward network traffic to.

If this annotation is not present, the default value will be taken from the `EXOSCALE_DEFAULT_ZONE` environment variable.


#### `service.beta.kubernetes.io/exoscale-loadbalancer-id`

The ID of the Exoscale NLB corresponding to the Kubernetes *Service*. This
annotation is set automatically by the Exoscale CCM after having created the
NLB instance if one was not specified (see the *Multiple Kubernetes Service on
a single Exoscale NLB* section for more information).


#### `service.beta.kubernetes.io/exoscale-loadbalancer-name`

The name of the Exoscale NLB. Defaults to `<Kubernetes Service UID>`.


#### `service.beta.kubernetes.io/exoscale-loadbalancer-description`

The description of the Exoscale NLB.


#### `service.beta.kubernetes.io/exoscale-loadbalancer-service-id`

The ID of the Exoscale NLB service corresponding to the Kubernetes *Service*.
For internal use only, set automatically by the Exoscale CCM. **Do not
modify**.


#### `service.beta.kubernetes.io/exoscale-loadbalancer-service-name`

The name of Exoscale NLB service corresponding to the Kubernetes *Service*
port. Defaults to `<Kubernetes Service UID>-<Service port>`.


#### `service.beta.kubernetes.io/exoscale-loadbalancer-service-description`

The description of the Exoscale NLB service corresponding to the Kubernetes
*Service*.


#### `service.beta.kubernetes.io/exoscale-loadbalancer-service-instancepool-id`

The ID of the Exoscale Instance Pool to forward ingress traffic to. Defaults to
the Instance Pool ID of the cluster *Nodes* ; this information must be
specified in case your *Service* is targeting *Pods* that are subject to
[custom *Node* scheduling][k8s-assign-pod-node].

> Note: the Instance Pool cannot be changed after NLB service creation – the
> k8s Service will have to be deleted and re-created with the annotation
> updated.


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


#### `service.beta.kubernetes.io/exoscale-loadbalancer-service-http-healthcheck-uri`

The Exoscale NLB service health check HTTP request URI (in `http` mode only).
Defaults to `/`.


#### `service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-interval`

The Exoscale NLB service health checking interval in seconds. Defaults to
`10s`.


#### `service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-timeout`

The Exoscale NLB service health checking timeout in seconds. Defaults to `5s`.


#### `service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-retries`

The Exoscale NLB service health checking retries before considering a target
*down*. Defaults to `1`.


### Alternative Exoscale NLB service health checking port

In most cases, applications health checking is performed on the same port used
to serve application traffic, for example on a HTTP endpoint such as `/status`
or `/healthz`. In those cases, the Kubernetes *Service* manifest if
straightforward:

```yaml
kind: Service
apiVersion: v1
metadata:
  name: nginx-http
  annotations:
    ...
spec:
  selector:
    app: my-webapp
  type: LoadBalancer
  ports:
  - port: 8000
```

In some other cases, applications don't handle health checking on the main
traffic port but expose an alternative port dedicated to health checking port.
To configure a Kubernetes *Service* with the Exoscale CCM to supports this use
case, you have to declare 2 **named** Kubernetes *Service* ports: one named
`service` (for serving the main service traffic), and another one named
`healthcheck` (for service the health checking traffic):

```yaml
kind: Service
apiVersion: v1
metadata:
  name: my-app
  annotations:
    ...
spec:
  selector:
    app: my-app
  type: LoadBalancer
  ports:
  - name: service
    port: 8000
  - name: healthcheck
    port: 8001
---
kind: Deployment
apiVersion: apps/v1
metadata:
  name: my-app
spec:
  selector:
    matchLabels:
      app: my-app
  replicas: 5
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
      - name: my-app
        image: my-app
        args: ["-listen=:8000", "-healthcheck-listen=:8001"]
        ports:
        - name: app
          containerPort: 8000
        - name: healthcheck
          containerPort: 8001
```


### Multiple Kubernetes Service on a single Exoscale NLB

Due to technical limitations, it is not possible to map multiple *ports* of a
single Kubernetes Service directly to multiple services of an Exoscale NLB –
only a single *Service* port is supported.

However, it is effectively possible to co-locate multiple Kubernetes *Services*
on a single Exoscale NLB instance (up to 10 services) by creating multiple
Kubernetes *Services* and explicitly specifying the ID of the same NLB in the
*Service* annotations. Here is an example of 2 different Kubernetes *Services*
created on the same NLB instance:

```yaml
kind: Service
apiVersion: v1
metadata:
  name: nginx-http
  annotations:
    service.beta.kubernetes.io/exoscale-loadbalancer-id: "81729656-e1d3-4bd6-8515-d9267aa4491b"
    service.beta.kubernetes.io/exoscale-loadbalancer-zone: "ch-gva-2"
    service.beta.kubernetes.io/exoscale-loadbalancer-service-name: "k8s-svc-nginx-http"
spec:
  selector:
    app: nginx
  type: LoadBalancer
  ports:
  - port: 80
---
kind: Service
apiVersion: v1
metadata:
  name: nginx-https
  annotations:
    service.beta.kubernetes.io/exoscale-loadbalancer-id: "81729656-e1d3-4bd6-8515-d9267aa4491b"
    service.beta.kubernetes.io/exoscale-loadbalancer-zone: "ch-gva-2"
    service.beta.kubernetes.io/exoscale-loadbalancer-service-name: "k8s-svc-nginx-https"
spec:
  selector:
    app: nginx
  type: LoadBalancer
  ports:
  - port: 443
```

In the result after applying the manifest, we can see that the 2 services share
the same external IP address:

```console
$ kubectl get svc
NAME          TYPE           CLUSTER-IP       EXTERNAL-IP       PORT(S)         AGE
...
nginx-http    LoadBalancer   10.107.25.129    194.182.181.104   80:30699/TCP    4m54s
nginx-https   LoadBalancer   10.103.219.252   194.182.181.104   443:31094/TCP   4m54s
```

When looking at the Exoscale NLB instance using the `exo` CLI, we can confirm
that the 2 Kubernetes *Services* have been created on the same NLB instance:

```console
$ exo nlb show -O json 81729656-e1d3-4bd6-8515-d9267aa4491b | jq -r '.services[].name'
k8s-svc-nginx-http
k8s-svc-nginx-https
```


## ⚠️ Important Notes

* Currently, the Exoscale CCM doesn't support UDP service load balancing due to
  a [technical limitation in Kubernetes][k8s-issue-no-proto-mix].
* As `NodePort` created by k8s *Services* are picked randomly [within a defined
  range][k8s-service-nodeport] (by default `30000-32767`), don't forget to
  configure [Security Groups][exo-sg] used by your Compute Instance Pools to
  accept ingress traffic in this range, otherwise the Exoscale Network Load
  Balancers won't be able to forward traffic to your *Pods*.


[custom-templates]: https://community.exoscale.com/documentation/compute/custom-templates/#create-a-custom-template
[exo-nlb-svc]: https://community.exoscale.com/documentation/compute/network-load-balancer/#network-load-balancer-services
[exo-nlb]: https://community.exoscale.com/documentation/compute/network-load-balancer/
[exo-sg]: https://community.exoscale.com/documentation/compute/security-groups/
[exo-zones]: https://www.exoscale.com/datacenters/
[k8s-assign-pod-node]: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/
[k8s-issue-no-proto-mix]: https://github.com/kubernetes/kubernetes/issues/23880
[k8s-service-kube-proxy]: https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies
[k8s-service-nodeport]: https://kubernetes.io/docs/concepts/services-networking/service/#nodeport
[k8s-service-spec]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#service-v1-core
[k8s-serviceport-spec]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#serviceport-v1-core
