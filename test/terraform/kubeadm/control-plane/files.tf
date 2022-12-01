## Files

# Configuration

# (shared variables)
resource "local_sensitive_file" "shared_tfvars" {
  # We use JSON such as to prevent 'terraform fmt' errors when the file doesn't exist
  # and the '../nodes/terraform.tfvars.json' symlink is broken
  filename        = abspath("${path.module}/output/shared.tfvars.json")
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

  filename        = abspath("${path.module}/output/${each.key}.kubeconfig")
  file_permission = "0600"
  content = templatefile(
    "${path.module}/resources/kubeconfig.yaml",
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
  filename = abspath("${path.module}/output/ccm-rbac.yaml")
  content = templatefile(
    local.ccm_rbac_path,
    {
      cluster_id = random_string.test_id.result
    }
  )
}

resource "local_file" "ccm_cloud_config" {
  filename = abspath("${path.module}/output/cloud-config.yaml")
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
  filename = abspath("${path.module}/output/shell.env")
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
