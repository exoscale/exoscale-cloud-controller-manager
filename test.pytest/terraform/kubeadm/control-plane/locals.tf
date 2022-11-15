locals {
  ## Test parameters

  # Unique test ID/name
  test_name = "${var.test_name}-${random_string.test_id.result}"


  ## CCM

  # API credentials file
  ccm_api_credentials_path = abspath("./output/api-credentials.json")

  # Source and executable
  ccm_main_path = abspath("../../../../cmd/exoscale-cloud-controller-manager/main.go")
  ccm_exe_path  = abspath("./output/ccm")

  # Configuration
  ccm_rbac_path         = abspath("../../../resources/manifests/ccm-rbac.yaml")
  ccm_cloud_config_path = abspath("../../../resources/cloud-config.yaml")


  ## Helpers
  shell_environment_path = abspath("../../../resources/shell.env")
}
