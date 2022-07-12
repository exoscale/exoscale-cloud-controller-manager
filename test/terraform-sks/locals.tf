locals {
  cluster_security_groups = {
    kubelet_logs           = { protocol = "TCP", port = 10250, cidr = "0.0.0.0/0" },
    kubelet_nodeports_ipv4 = { protocol = "TCP", port = "30000-32767", cidr = "0.0.0.0/0" }
    kubelet_nodeports_ipv6 = { protocol = "TCP", port = "30000-32767", cidr = "::/0" }
    calico_vxlan           = { protocol = "UDP", port = 4789, sg = exoscale_security_group.cluster.id }
  }

  name = "${var.name}-${random_string.test_id.result}"
}

locals {
  sks_domain                 = "${var.environment == "preprod" ? "ppsks" : "sks"}-${var.zone}.exo.io"
  cluster_id                 = exoscale_sks_cluster.cluster.id
  cluster_apiserver_endpoint = "${local.cluster_id}.${local.sks_domain}"


  generated_manifest_hello_no_ingress = <<EOT
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello
  labels:
    app: hello
spec:
  selector:
    matchLabels:
      app: hello
  replicas: 2
  template:
    metadata:
      labels:
        app: hello
    spec:
      containers:
      - name: nginx
        image: nginxdemos/hello:plain-text
        ports:
        - containerPort: 80
---
kind: Service
apiVersion: v1
metadata:
  name: hello
  annotations:
    service.beta.kubernetes.io/exoscale-loadbalancer-zone: "${var.zone}"
    service.beta.kubernetes.io/exoscale-loadbalancer-id: "${exoscale_nlb.external.id}"
    service.beta.kubernetes.io/exoscale-loadbalancer-external: "true"
    service.beta.kubernetes.io/exoscale-loadbalancer-service-instancepool-id: "${var.pool_size > 0 ? exoscale_sks_nodepool.pool[0].instance_pool_id : "nop"}"
    service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-interval: "5s"
    service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-timeout: "2s"
    service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-retries: "1"
spec:
  selector:
    app: hello
  type: LoadBalancer
  ports:
    - port: 80
EOT
}
