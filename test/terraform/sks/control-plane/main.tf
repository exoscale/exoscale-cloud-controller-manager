## SKS

# Cluster
resource "exoscale_security_group" "cluster_sg" {
  # REF: https://registry.terraform.io/providers/exoscale/exoscale/latest/docs/resources/security_group
  name = local.test_name
}

resource "exoscale_security_group_rule" "cluster_sg_rule" {
  # REF: https://registry.terraform.io/providers/exoscale/exoscale/latest/docs/resources/security_group_rule
  for_each = {
    # Kubernetes
    # REF: https://kubernetes.io/docs/reference/networking/ports-and-protocols/
    kubelet_logs           = { protocol = "TCP", port = 10250, cidr = "0.0.0.0/0" },
    kubelet_nodeports_ipv4 = { protocol = "TCP", port = "30000-32767", cidr = "0.0.0.0/0" }
    kubelet_nodeports_ipv6 = { protocol = "TCP", port = "30000-32767", cidr = "::/0" }
    # Calico
    # REF: https://projectcalico.docs.tigera.io/getting-started/kubernetes/requirements#network-requirements
    calico_typha = { protocol = "TCP", port = 5473, sg = exoscale_security_group.cluster_sg.id }
    calico_bgp   = { protocol = "TCP", port = 179, sg = exoscale_security_group.cluster_sg.id }
    calico_ipip  = { protocol = "IPIP", sg = exoscale_security_group.cluster_sg.id }
    calico_vxlan = { protocol = "UDP", port = 4789, sg = exoscale_security_group.cluster_sg.id }
  }

  security_group_id      = exoscale_security_group.cluster_sg.id
  protocol               = each.value["protocol"]
  type                   = "INGRESS"
  start_port             = try(split("-", each.value.port)[0], each.value.port, null)
  end_port               = try(split("-", each.value.port)[1], each.value.port, null)
  cidr                   = try(each.value.cidr, null)
  user_security_group_id = try(each.value.sg, null)
}

resource "exoscale_sks_cluster" "cluster" {
  # REF: https://registry.terraform.io/providers/exoscale/exoscale/latest/docs/resources/sks_cluster
  zone = var.exoscale_zone
  name = local.test_name

  version        = try(var.sks_version, null)
  cni            = var.kubernetes_cni
  exoscale_ccm   = false
  metrics_server = true
  service_level  = "starter"
}

resource "exoscale_sks_kubeconfig" "kubeconfig" {
  # REF: https://registry.terraform.io/providers/exoscale/exoscale/latest/docs/resources/sks_kubeconfig
  for_each = {
    "admin" = { user = "admin", groups = ["system:masters"] }
    "ccm"   = { user = "ccm-${exoscale_sks_cluster.cluster.id}", groups = ["system:cloud-controller-manager"] }
  }

  zone = var.exoscale_zone

  cluster_id  = exoscale_sks_cluster.cluster.id
  user        = each.value.user
  groups      = each.value.groups
  ttl_seconds = 604800 # 7 days
}
