## cloud-init configuration (compute node 'user-data')
#  REF: https://cloudinit.readthedocs.io/en/latest/topics/format.html#part-handler

# cloudinit_config
# REF: https://registry.terraform.io/providers/hashicorp/cloudinit/latest/docs/data-sources/cloudinit_config
data "cloudinit_config" "user_data" {
  gzip          = false
  base64_encode = false

  # cloud-config
  part {
    filename     = "init.cfg"
    content_type = "text/jinja2"
    content = templatefile(
      "${path.module}/resources/cloud-init.yaml",
      {
        # System setup
        # (APT)
        apt_key_docker     = file("${local.system_config_path}/apt-key.docker.gpg")
        apt_key_kubernetes = file("${local.system_config_path}/apt-key.kubernetes.gpg")
        # (modules)
        modprobe_kubernetes_blacklist = file("${local.system_config_path}/modprobe.kubernetes-blacklist.conf")
        modules_kubernetes            = file("${local.system_config_path}/modules.kubernetes.conf")
        # (networking)
        sysctl_kubernetes_networking = file("${local.system_config_path}/sysctl.kubernetes-networking.conf")
        # (containerd)
        containerd_config = file("${local.system_config_path}/containerd.config.toml")
        # Kubernetes configuration
        # (kubeadm)
        kubeadm_init_config = templatefile(
          "${path.module}/resources/kubeadm.init-config.yaml",
          {
            bootstrap_token = "${random_string.bootstrap_token_id.result}.${random_string.bootstrap_token_secret.result}"
            dns_domain      = local.k8s_dns_domain
            service_subnet  = local.k8s_service_subnet
            pod_subnet      = local.k8s_pod_subnet
        })
        # TLS
        # (control plane)
        tls_ca_kubernetes_cert     = tls_self_signed_cert.tls_ca["kubernetes"].cert_pem
        tls_ca_kubernetes_key      = tls_private_key.tls_ca["kubernetes"].private_key_pem
        tls_ca_service_account_pub = tls_private_key.tls_ca["service-account"].public_key_pem
        tls_ca_service_account_key = tls_private_key.tls_ca["service-account"].private_key_pem
        # (etcd)
        tls_ca_etcd_cert = tls_self_signed_cert.tls_ca["etcd"].cert_pem
        tls_ca_etcd_key  = tls_private_key.tls_ca["etcd"].private_key_pem
        # (front-proxy)
        tls_ca_front_proxy_cert = tls_self_signed_cert.tls_ca["front-proxy"].cert_pem
        tls_ca_front_proxy_key  = tls_private_key.tls_ca["front-proxy"].private_key_pem
      }
    )
  }
}
