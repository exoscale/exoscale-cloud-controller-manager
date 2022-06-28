locals {
  cluster_security_groups = {
    ssh               = { protocol = "TCP", port = 22 } # /for testing purposes
    kube_apiserver    = { protocol = "TCP", port = 6443 },
    kubelet_logs      = { protocol = "TCP", port = 10250 },
    kubelet_nodeports = { protocol = "TCP", port = "30000-32767" }
    calico_bgp        = { protocol = "TCP", port = 179 }
    calico_ipip       = { protocol = "IPIP" }
  }

  name = "${var.name}-${random_string.test_id.result}"
}

locals {
  dns_domain = "cluster.local"
  svc_subnet = "10.96.0.0/12"
  pod_subnet = "192.168.0.0/16"

  # Installation script: containerd

  # REF: https://docs.docker.com/engine/install/ubuntu/#install-using-the-repository
  # NOTE: default containerd settings are fine for our use-case
  containerd_setup = <<EOT
#!/bin/bash

sudo apt-get install \
    ca-certificates \
    curl \
    gnupg \
    lsb-release

sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

sudo apt-get update
sudo apt-get install containerd.io

rm /etc/containerd/config.toml
systemctl restart containerd
EOT

  # According to Kubernetes documentation:
  # - DCCP is unlikely to be needed, has had multiple serious
  # vulnerabilities, and is not well-maintained.
  # - SCTP is not used in most Kubernetes clusters, and has also had
  # vulnerabilities in the past.
  kubernetes_module_blacklist = <<EOT
blacklist dccp
blacklist sctp
EOT

  kubernetes_modules = <<EOT
overlay
br_netfilter
EOT

  kubernetes_sysctls = <<EOT
net.bridge.bridge-nf-call-ip6tables = 1
net.bridge.bridge-nf-call-iptables = 1
net.ipv4.ip_forward = 1
EOT

  # Installation script: kubeadm & kubernetes binaries

  # REF: https://kubernetes.io/fr/docs/setup/production-environment/tools/kubeadm/install-kubeadm/
  kubernetes_kubeadm_setup = <<EOT
#!/bin/bash

sudo apt-get update && sudo apt-get install -y apt-transport-https curl
curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add -
cat <<EOF | sudo tee /etc/apt/sources.list.d/kubernetes.list
deb https://apt.kubernetes.io/ kubernetes-xenial main
EOF
sudo apt-get update
sudo apt-get install -y kubelet kubeadm kubectl
sudo apt-mark hold kubelet kubeadm kubectl
EOT

  # Installation script: kubelet in TLS bootstraping mode

  # REF: TODO
  kubernetes_kubelet_setup = <<EOT
#!/bin/bash

sudo apt-get update && sudo apt-get install -y apt-transport-https curl
curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add -
cat <<EOF | sudo tee /etc/apt/sources.list.d/kubernetes.list
deb https://apt.kubernetes.io/ kubernetes-xenial main
EOF
sudo apt-get update
sudo apt-get install -y kubelet
sudo apt-mark hold kubelet

mkdir -p /etc/systemd/system/kubelet.service.d/

cat > /etc/systemd/system/kubelet.service.d/99-override.conf <<EOF
[Service]
EnvironmentFile=-/etc/default/kubelet
ExecStart=
ExecStart=/usr/bin/kubelet \\
  --bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf \
  --kubeconfig=/etc/kubernetes/kubelet.conf \
  --container-runtime-endpoint="unix:///run/containerd/containerd.sock" \
  --config=/etc/kubernetes/config.yaml

EOF

cat > /etc/kubernetes/config.yaml <<EOF
kind: KubeletConfiguration
apiVersion: kubelet.config.k8s.io/v1beta1
authentication:
  x509:
    clientCAFile: "/etc/kubernetes/pki/ca.crt"
cgroupDriver: "systemd"
clusterDNS:
- 10.96.0.10
resolvConf: "/run/systemd/resolve/resolv.conf"
serverTLSBootstrap: true
registerNode: true

EOF

systemctl daemon-reload

EOT

  kubeadm_configuration = <<EOT
kind: InitConfiguration
apiVersion: kubeadm.k8s.io/v1beta3
bootstrapTokens:
- token: "${random_string.token_id.result}.${random_string.token_secret.result}"
  ttl: 24h0m0s
  groups:
  - system:bootstrappers:kubeadm:default-node-token
  usages:
  - signing
  - authentication
nodeRegistration:
  kubeletExtraArgs:
    cloud-provider: "external"
---
kind: ClusterConfiguration
apiVersion: kubeadm.k8s.io/v1beta3
networking:
  dnsDomain: ${local.dns_domain}
  serviceSubnet: ${local.svc_subnet}
  podSubnet: ${local.pod_subnet}
EOT

  kubelet_bootstrap_kubeconfig = <<EOT
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: ${base64encode(tls_self_signed_cert.root_ca["kubernetes-ca"].cert_pem)}
    server: https://${exoscale_compute_instance.control_plane.public_ip_address}:6443
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: tls-bootstrap-token-user
  name: tls-bootstrap-token-user@kubernetes
current-context: tls-bootstrap-token-user@kubernetes
users:
- name: tls-bootstrap-token-user
  user:
    token: "${random_string.token_id.result}.${random_string.token_secret.result}"
preferences: {}
EOT

  control_plane_user_data = <<EOT
#cloud-config

bootcmd:
- [ cloud-init-per, once, mkdir, /etc/kubernetes ]
- [ cloud-init-per, once, mkdir, /etc/kubernetes/pki ]
- [ cloud-init-per, once, mkdir, /etc/kubernetes/pki/etcd ]

write_files:
- path: /usr/local/bin/setup-containerd
  content: ${base64encode(local.containerd_setup)}
  encoding: b64
  owner: root:root
  permissions: '0700'
- path: /usr/local/bin/setup-kubeadm
  content: ${base64encode(local.kubernetes_kubeadm_setup)}
  encoding: b64
  owner: root:root
  permissions: '0700'
- path: /etc/kubernetes/kubeadm-init.yml
  content: ${base64encode(local.kubeadm_configuration)}
  encoding: b64
  owner: root:root
  permissions: '0644'
- path: /etc/kubernetes/pki/ca.crt
  content: ${base64encode(tls_self_signed_cert.root_ca["kubernetes-ca"].cert_pem)}
  encoding: b64
  owner: root:root
  permissions: '0644'
- path: /etc/kubernetes/pki/ca.key
  content: ${base64encode(tls_private_key.root_ca["kubernetes-ca"].private_key_pem)}
  encoding: b64
  owner: root:root
  permissions: '0600'
- path: /etc/kubernetes/pki/etcd/ca.crt
  content: ${base64encode(tls_self_signed_cert.root_ca["etcd-ca"].cert_pem)}
  encoding: b64
  owner: root:root
  permissions: '0644'
- path: /etc/kubernetes/pki/etcd/ca.key
  content: ${base64encode(tls_private_key.root_ca["etcd-ca"].private_key_pem)}
  encoding: b64
  owner: root:root
  permissions: '0600'
- path: /etc/kubernetes/pki/front-proxy-ca.crt
  content: ${base64encode(tls_self_signed_cert.root_ca["kubernetes-front-proxy-ca"].cert_pem)}
  encoding: b64
  owner: root:root
  permissions: '0644'
- path: /etc/kubernetes/pki/front-proxy-ca.key
  content: ${base64encode(tls_private_key.root_ca["kubernetes-front-proxy-ca"].private_key_pem)}
  encoding: b64
  owner: root:root
  permissions: '0600'
- path: /etc/kubernetes/pki/sa.pub
  content: ${base64encode(tls_private_key.root_ca["etcd-ca"].public_key_pem)}
  encoding: b64
  owner: root:root
  permissions: '0644'
- path: /etc/kubernetes/pki/sa.key
  content: ${base64encode(tls_private_key.root_ca["etcd-ca"].private_key_pem)}
  encoding: b64
  owner: root:root
  permissions: '0600'
- path: /etc/modprobe.d/kubernetes-blacklist.conf
  content: ${base64encode(local.kubernetes_module_blacklist)}
  encoding: b64
  owner: root:root
  permissions: '0644'
- path: /etc/modules-load.d/kubernetes-networking.conf
  content: ${base64encode(local.kubernetes_modules)}
  encoding: b64
  owner: root:root
  permissions: '0644'
- path: /etc/sysctl.d/99-kubernetes-networking.conf
  content: ${base64encode(local.kubernetes_sysctls)}
  encoding: b64
  owner: root:root
  permissions: '0644'

runcmd:
- [sudo, sysctl, --system]
- [sudo, systemctl, restart, systemd-modules-load.service]
- [sudo, setup-containerd]
- [sudo, setup-kubeadm]
- [sudo, kubeadm, init, --config=/etc/kubernetes/kubeadm-init.yml]
- [sudo, kubeadm, token, create, ${random_string.token_id.result}.${random_string.token_secret.result}]

EOT

  node_user_data = <<EOT
#cloud-config

bootcmd:
- [ cloud-init-per, once, mkdir, /etc/kubernetes ]
- [ cloud-init-per, once, mkdir, /etc/kubernetes/pki ]

write_files:
- path: /usr/local/bin/setup-containerd
  content: ${base64encode(local.containerd_setup)}
  encoding: b64
  owner: root:root
  permissions: '0700'
- path: /usr/local/bin/setup-kubelet
  content: ${base64encode(local.kubernetes_kubelet_setup)}
  encoding: b64
  owner: root:root
  permissions: '0700'
- path: /etc/kubernetes/pki/ca.crt
  content: ${base64encode(tls_self_signed_cert.root_ca["kubernetes-ca"].cert_pem)}
  encoding: b64
  owner: root:root
  permissions: '0600'
- path: /etc/kubernetes/bootstrap-kubelet.conf
  content: ${base64encode(local.kubelet_bootstrap_kubeconfig)}
  encoding: b64
  owner: root:root
  permissions: '0600'
- path: /etc/default/kubelet
  content: "KUBELET_OPTS='--cloud-provider=external'"
  owner: root:root
  permissions: '0644'
- path: /etc/modprobe.d/kubernetes-blacklist.conf
  content: ${base64encode(local.kubernetes_module_blacklist)}
  encoding: b64
  owner: root:root
  permissions: '0644'
- path: /etc/modules-load.d/kubernetes-networking.conf
  content: ${base64encode(local.kubernetes_modules)}
  encoding: b64
  owner: root:root
  permissions: '0644'
- path: /etc/sysctl.d/99-kubernetes-networking.conf
  content: ${base64encode(local.kubernetes_sysctls)}
  encoding: b64
  owner: root:root
  permissions: '0644'

runcmd:
- [sudo, sysctl, --system]
- [sudo, systemctl, restart, systemd-modules-load.service]
- [sudo, setup-containerd]
- [sudo, setup-kubelet]
- [sudo, systemctl, restart, kubelet]

EOT

}
