## Files

# Configuration

# (cluster variables)
resource "local_sensitive_file" "shared_tfvars" {
  # We use JSON such as to prevent 'terraform fmt' errors when the file doesn't exist
  # and the '../nodes/terraform.tfvars.json' symlink is broken
  filename        = abspath("./output/shared.tfvars.json")
  file_permission = "0600"
  content         = <<-EOT
  {
    "test_id": "${random_string.test_id.result}",
    "test_cluster_id": "${exoscale_sks_cluster.cluster.id}",
    "test_cluster_sg_id": "${exoscale_security_group.cluster_sg.id}"
  }
  EOT
}

# (Kubeconfig)
resource "local_sensitive_file" "kubeconfig" {
  for_each = exoscale_sks_kubeconfig.kubeconfig

  filename        = abspath("./output/${each.key}.kubeconfig")
  file_permission = "0600"
  content         = exoscale_sks_kubeconfig.kubeconfig[each.key].kubeconfig
}

# (CCM)
resource "local_file" "ccm_rbac" {
  filename = abspath("./output/ccm-rbac.yaml")
  content = templatefile(
    local.ccm_rbac_path,
    {
      cluster_id = exoscale_sks_cluster.cluster.id
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
