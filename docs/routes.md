# VPC Pod routes

The Exoscale CCM can route each Node Pod CIDR (`node.spec.podCIDRs`) to its Node
through an Exoscale VPC, so that Pods get native, unencapsulated addresses that
are routable from any instance of the VPC. It implements the Kubernetes
[`Routes`][k8s-routes] cloud provider interface: the upstream route controller
watches Nodes and calls the CCM to create and delete the routes.

This is CNI-agnostic: any CNI using the Node Pod CIDR for IPAM, without
encapsulation, works on top of it (e.g. Cilium `ipam.mode: kubernetes` +
`routingMode: native`, or Calico `ipam.type: HostLocal` + `encapsulation: None`).

## Requirements

* Every Node instance is **created with `ip-forwarding`**: without it, the
  platform anti-spoofing drops every Pod-sourced packet. The CCM refuses to
  create a route for a Node lacking it, which surfaces as a
  `FailedToCreateRoute` Event and keeps the Node tainted
  `node.kubernetes.io/network-unavailable`.
* Node CIDR allocation enabled on the kube-controller-manager
  (`--allocate-node-cidrs`, `--cluster-cidr`).
* The Node security groups admit traffic sourced from the cluster Pod CIDR
  (security groups match on source address inside a VPC Subnet).
* Each Node sends the cluster Pod CIDR to the Subnet gateway through its VPC
  interface, e.g. `ip route add <cluster Pod CIDR> via <Subnet gateway> dev eth1`.

## Configuration

```yaml
routes:
  enabled: true
  vpc: <VPC ID>
  subnet: <Subnet ID>     # Nodes Subnet, Nodes not attached yet are attached to it
  additionalSubnets: []   # other Subnets whose instances must reach Pods directly
```

Or through environment variables, which take precedence over the configuration
file:

```
EXOSCALE_ROUTES_ENABLED=true
EXOSCALE_ROUTES_VPC=<VPC ID>
EXOSCALE_ROUTES_SUBNET=<Subnet ID>
EXOSCALE_ROUTES_ADDITIONAL_SUBNETS=<Subnet ID>,<Subnet ID>
```

The CCM must run with:

```
--configure-cloud-routes=true
--cluster-cidr=<cluster Pod CIDR>
--cluster-name=<unique cluster name>
```

Routes are created in the Nodes Subnet and in every additional Subnet: return
traffic is routed by the replying instance's Subnet, so a Subnet missing the
routes can reach Pods but never gets the replies.

Routes are owned through their description (`k8s/<cluster name>/<node>`): the
CCM never lists, modifies nor deletes any other route, so several clusters can
share a Subnet as long as their cluster names and Pod CIDRs differ.

[k8s-routes]: https://pkg.go.dev/k8s.io/cloud-provider#Routes
