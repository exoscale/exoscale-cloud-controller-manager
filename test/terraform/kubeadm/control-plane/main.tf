## Kubernetes control plane

# Control plane
resource "exoscale_security_group" "cluster_sg" {
  # REF: https://registry.terraform.io/providers/exoscale/exoscale/latest/docs/resources/security_group
  name = local.test_name
}

resource "exoscale_security_group_rule" "cluster_sg_rule" {
  # REF: https://registry.terraform.io/providers/exoscale/exoscale/latest/docs/resources/security_group_rule
  for_each = {
    ssh = { protocol = "TCP", port = 22, cidr = "0.0.0.0/0" }
    # Kubernetes
    # REF: https://kubernetes.io/docs/reference/networking/ports-and-protocols/
    kube_apiserver         = { protocol = "TCP", port = 6443, cidr = "0.0.0.0/0" },
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

resource "exoscale_compute_instance" "control_plane" {
  # REF: https://registry.terraform.io/providers/exoscale/exoscale/latest/docs/resources/compute_instance
  zone = var.exoscale_zone

  name        = "${local.test_name}-control-plane"
  type        = "standard.medium"
  template_id = data.exoscale_template.node_template.id
  disk_size   = 10
  ipv6        = true

  ssh_key   = exoscale_ssh_key.ssh_key.name
  user_data = data.cloudinit_config.user_data.rendered

  security_group_ids = [exoscale_security_group.cluster_sg.id]

  connection {
    type        = "ssh"
    host        = self.public_ip_address
    user        = data.exoscale_template.node_template.default_user
    private_key = tls_private_key.ssh_key.private_key_openssh
  }

  provisioner "remote-exec" {
    inline = [
      "echo 'Waiting for cloud-init to complete (this may take some time) ...'",
      "sudo cloud-init status --wait >/dev/null",
      "sudo cloud-init status --long",
      "echo 'Initializing Kubernetes control plane (this may take some time) ...'",
      "sudo kubeadm init --config=/etc/kubernetes/kubeadm/init-config.yaml",
    ]
  }
}
