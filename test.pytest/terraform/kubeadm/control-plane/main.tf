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
  template_id = data.exoscale_compute_template.node_template.id
  disk_size   = 10
  ipv6        = true

  ssh_key   = exoscale_ssh_key.ssh_key.name
  user_data = data.cloudinit_config.user_data.rendered

  security_group_ids = [exoscale_security_group.cluster_sg.id]

  connection {
    type        = "ssh"
    host        = self.public_ip_address
    user        = data.exoscale_compute_template.node_template.username
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

# Configuration

# (shared variables)
resource "local_sensitive_file" "shared_tfvars" {
  # We use JSON such as to prevent 'terraform fmt' errors when the file doesn't exist
  # and the '../nodes/terraform.tfvars.json' symlink is broken
  filename        = abspath("./output/shared.tfvars.json")
  file_permission = "0600"
  content         = <<-EOT
  {
    "test_id": "${random_string.test_id.result}",
    "test_cluster_id": "${random_string.test_id.result}",
    "test_cluster_sg_id": "${exoscale_security_group.cluster_sg.id}",
    "test_control_plane_endpoint": "https://${exoscale_compute_instance.control_plane.public_ip_address}:6443",
    "test_nodes_bootstrap_token": "${random_string.bootstrap_token_id.result}.${random_string.bootstrap_token_secret.result}",
    "test_nodes_ssh_key_name": "${exoscale_ssh_key.ssh_key.name}"
  }
  EOT
}

# (Kubeconfig)
resource "local_sensitive_file" "kubeconfig" {
  for_each = {
    "admin" = { user = "admin" }
    "ccm"   = { user = "ccm-${random_string.test_id.result}" }
  }

  filename        = abspath("./output/${each.key}.kubeconfig")
  file_permission = "0600"
  content = templatefile(
    "./resources/kubeconfig.yaml",
    {
      username        = each.value.user
      server          = exoscale_compute_instance.control_plane.public_ip_address
      tls_ca          = base64encode(tls_self_signed_cert.tls_ca["kubernetes"].cert_pem)
      tls_client_cert = base64encode(tls_locally_signed_cert.tls_client[each.key].cert_pem)
      tls_client_key  = base64encode(tls_private_key.tls_client[each.key].private_key_pem)
    },
  )
}

# (CCM)
resource "local_file" "ccm_rbac" {
  filename = abspath("./output/ccm-rbac.yaml")
  content = templatefile(
    local.ccm_rbac_path,
    {
      cluster_id = random_string.test_id.result
    }
  )
}

resource "local_file" "ccm_cloud_config" {
  filename = abspath("./output/cloud-config.yaml")
  content = templatefile(
    local.ccm_cloud_config_path,
    {
      exoscale_zone        = var.exoscale_zone
      api_credentials_path = local.ccm_api_credentials_path
    }
  )
}

# (shell environment)
resource "local_file" "shell_environment" {
  filename = abspath("./output/shell.env")
  content = templatefile(
    local.shell_environment_path,
    {
      exoscale_zone         = var.exoscale_zone
      kubeconfig_admin_path = local_sensitive_file.kubeconfig["admin"].filename
      kubeconfig_ccm_path   = local_sensitive_file.kubeconfig["ccm"].filename
      ccm_cloud_config_path = local_file.ccm_cloud_config.filename
      ccm_main_path         = local.ccm_main_path
    }
  )
}
