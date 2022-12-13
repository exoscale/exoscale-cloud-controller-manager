locals {
  ## Exoscale

  # Private network
  privnet_netmask  = "255.255.255.0"
  privnet_start_ip = "172.16.0.100"
  privnet_end_ip   = "172.16.0.253"


  ## System setup

  # Configuration
  system_config_path = abspath("${path.module}/../../../resources/system")


  ## Kubernetes parameters

  # Manifests
  k8s_manifests_path = abspath("${path.module}/../../../resources/manifests")

  # DNS
  k8s_dns_domain  = "cluster.local"
  k8s_dns_address = "10.96.0.10"

  # Networks
  k8s_service_subnet = "10.96.0.0/12"
  k8s_pod_subnet     = "192.168.0.0/16"
}
