locals {
  ## System setup

  # Configuration
  system_config_path = abspath("${path.module}/../../../resources/system")


  ## Kubernetes parameters

  # Manifests
  k8s_manifests_path = abspath("${path.module}/../../../resources/manifests")
}
