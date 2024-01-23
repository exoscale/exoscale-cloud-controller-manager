## Kubernetes nodes (workers)

# Private network
resource "exoscale_private_network" "nodes_privnet" {
  # REF: https://registry.terraform.io/providers/exoscale/exoscale/latest/docs/resources/private_network
  zone = var.exoscale_zone
  name = "${local.test_name}-privnet"

  netmask  = local.privnet_netmask
  start_ip = local.privnet_start_ip
  end_ip   = local.privnet_end_ip
}

# Nodes
resource "exoscale_anti_affinity_group" "nodes_aag" {
  # REF: https://registry.terraform.io/providers/exoscale/exoscale/latest/docs/resources/anti_affinity_group
  name = local.test_name
}

# (pool)
resource "exoscale_instance_pool" "nodepool" {
  # REF: https://registry.terraform.io/providers/exoscale/exoscale/latest/docs/resources/instance_pool
  count = var.test_nodes_pool_size > 0 ? 1 : 0

  zone = var.exoscale_zone
  name = local.test_name

  size = var.test_nodes_pool_size

  instance_prefix = "${local.test_name}-pool"
  instance_type   = "standard.small"
  template_id     = data.exoscale_template.node_template.id
  disk_size       = 10
  ipv6            = true

  key_pair  = var.test_nodes_ssh_key_name
  user_data = data.cloudinit_config.user_data["pool"].rendered

  security_group_ids = [var.test_cluster_sg_id]
  network_ids        = [exoscale_private_network.nodes_privnet.id]
}

# (external)
resource "exoscale_compute_instance" "external_node" {
  # REF: https://registry.terraform.io/providers/exoscale/exoscale/latest/docs/resources/compute_instance
  zone = var.exoscale_zone
  name = "${local.test_name}-external"

  type        = "standard.small"
  template_id = data.exoscale_template.node_template.id
  disk_size   = 10
  ipv6        = true

  ssh_key   = var.test_nodes_ssh_key_name
  user_data = data.cloudinit_config.user_data["external"].rendered

  security_group_ids = [var.test_cluster_sg_id]

  connection {
    type        = "ssh"
    host        = self.public_ip_address
    user        = data.exoscale_template.node_template.default_user
    private_key = file("../control-plane/output/ssh.id_ed25519")
  }

  provisioner "remote-exec" {
    inline = [
      "echo 'Waiting for cloud-init to complete (this may take some time) ...'",
      "sudo cloud-init status --wait >/dev/null",
      "sudo cloud-init status --long",
    ]
  }
}

# Load balancer (NLB)
resource "exoscale_nlb" "external_nlb" {
  # REF: https://registry.terraform.io/providers/exoscale/exoscale/latest/docs/resources/nlb
  zone = var.exoscale_zone
  name = local.test_name
}
