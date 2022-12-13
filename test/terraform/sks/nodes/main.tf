## SKS

# Nodes
resource "exoscale_anti_affinity_group" "nodes_aag" {
  # REF: https://registry.terraform.io/providers/exoscale/exoscale/latest/docs/resources/anti_affinity_group
  name = local.test_name
}

# (pool)
resource "exoscale_sks_nodepool" "nodepool" {
  # REF: https://registry.terraform.io/providers/exoscale/exoscale/latest/docs/resources/sks_nodepool
  count = var.test_nodes_pool_size > 0 ? 1 : 0

  zone = var.exoscale_zone
  name = local.test_name

  cluster_id      = var.test_cluster_id
  instance_type   = "standard.small"
  instance_prefix = "${local.test_name}-pool"
  size            = var.test_nodes_pool_size

  anti_affinity_group_ids = [exoscale_anti_affinity_group.nodes_aag.id]
  security_group_ids      = [var.test_cluster_sg_id]
}

# Load balancer (NLB)
resource "exoscale_nlb" "external_nlb" {
  # REF: https://registry.terraform.io/providers/exoscale/exoscale/latest/docs/resources/nlb
  zone = var.exoscale_zone
  name = local.test_name
}
