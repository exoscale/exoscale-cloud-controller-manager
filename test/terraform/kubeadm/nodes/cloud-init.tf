## cloud-init configuration (compute node 'user-data')
#  REF: https://cloudinit.readthedocs.io/en/latest/topics/format.html#part-handler

# cloudinit_config
# REF: https://registry.terraform.io/providers/hashicorp/cloudinit/latest/docs/data-sources/cloudinit_config
data "cloudinit_config" "user_data" {
  for_each = toset(["pool", "external"])

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
        # (kubelet)
        kubelet_systemd_bootstrap = templatefile(
          "${path.module}/resources/kubelet.systemd-bootstrap.conf",
          {
            set_node_ip = each.key == "external" ? false : true
        })
        # Kubernetes configuration
        # (kubelet)
        kubelet_set_provider_id = each.key == "external" ? false : true
        kubelet_bootstrap_config = templatefile(
          "${path.module}/resources/kubelet.bootstrap-kubeconfig.yaml",
          {
            cluster_endpoint = var.test_control_plane_endpoint
            cluster_ca       = file("../control-plane/output/kubernetes-ca.pem")
            bootstrap_token  = var.test_nodes_bootstrap_token
        })
        kubelet_config = templatefile(
          "${path.module}/resources/kubelet.config.yaml",
          {
            cluster_dns = local.k8s_dns_address
            taints      = each.key == "external" ? ["node.exoscale.net/external:true:NoSchedule"] : []
        })
        # TLS
        # (control plane)
        tls_ca_kubernetes_cert = file("../control-plane/output/kubernetes-ca.pem")
      }
    )
  }
}
