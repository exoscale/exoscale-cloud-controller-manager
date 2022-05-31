locals {
  cluster_security_groups = {
    kubelet_logs      = { protocol = "TCP", port = 10250 },
    kubelet_nodeports = { protocol = "TCP", port = "30000-32767" }
    calico_vxlan      = { protocol = "UDP", port = 4789 }
  }

  sks_domain                 = "${var.environment == "preprod" ? "ppsks" : "sks"}-${var.zone}.exo.io"
  cluster_id                 = exoscale_sks_cluster.cluster.id
  cluster_apiserver_endpoint = "${local.cluster_id}.${local.sks_domain}"
}
