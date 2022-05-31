data "http" "my_ip" {
  url = "http://ipconfig.me"
}

resource "exoscale_security_group" "cluster" {
  name             = var.name
  external_sources = ["${chomp(data.http.my_ip.body)}/32"]
}

resource "exoscale_security_group_rule" "cluster" {
  for_each = local.cluster_security_groups

  security_group_id      = exoscale_security_group.cluster.id
  protocol               = each.value["protocol"]
  type                   = "INGRESS"
  icmp_type              = try(each.value.icmp_type, null)
  icmp_code              = try(each.value.icmp_code, null)
  start_port             = try(split("-", each.value.port)[0], each.value.port, null)
  end_port               = try(split("-", each.value.port)[1], each.value.port, null)
  user_security_group_id = exoscale_security_group.cluster.id
}

resource "exoscale_anti_affinity_group" "cluster" {
  name = var.name
}

resource "exoscale_sks_cluster" "cluster" {
  zone    = var.zone
  name    = var.name
  version = try(var.sks_version, null)

  cni            = "calico"
  exoscale_ccm   = false
  metrics_server = true
}

resource "exoscale_sks_kubeconfig" "client" {
  for_each = {
    operator = { user = "admin", groups = ["system:masters"] }
    ccm      = { user = "cloud-controller-manager", groups = ["system:cloud-controller-manager"] }
  }

  zone       = var.zone
  cluster_id = exoscale_sks_cluster.cluster.id

  user        = "admin"
  groups      = ["system:masters"]
  ttl_seconds = 2628000
}

resource "local_file" "sks_dev_env" {
  content  = <<-EOT
  export KUBECONFIG="./${local_sensitive_file.cluster_client["operator"].filename}"
  export EXOSCALE_ZONE="${var.zone}"

  # Helper alias to approve pending CSRs from Kubelets
  alias approve-csr="kubectl get csr -o go-template='{{range .items}}{{if not .status}}{{.metadata.name}}{{\"\\n\"}}{{end}}{{end}}' | xargs kubectl certificate approve"

  # Helper alias to run CCM from local env on the remote cluster
  alias go-run-ccm="go run ../cmd/exoscale-cloud-controller-manager/main.go --kubeconfig=ccm.kubeconfig --authentication-kubeconfig=ccm.kubeconfig --authorization-kubeconfig=ccm.kubeconfig --allow-untagged-cloud"
  EOT
  filename = ".env"
}

resource "local_sensitive_file" "cluster_client" {
  for_each        = exoscale_sks_kubeconfig.client
  content         = exoscale_sks_kubeconfig.client[each.key].kubeconfig
  filename        = "${each.key}.kubeconfig"
  file_permission = "0600"
}

resource "exoscale_sks_nodepool" "cluster" {
  zone          = var.zone
  cluster_id    = exoscale_sks_cluster.cluster.id
  name          = var.name
  instance_type = "standard.medium"
  size          = 2

  anti_affinity_group_ids = [exoscale_anti_affinity_group.cluster.id]
  security_group_ids      = [exoscale_security_group.cluster.id]
}

resource "null_resource" "manifests" {
  depends_on = [exoscale_sks_kubeconfig.client]

  triggers = {
    apply_command  = "kubectl apply -f ./manifests/ccm-rbac.yaml"
    delete_command = "kubectl delete -f ./manifests/ccm-rbac.yaml"
    kubeconfig     = local_sensitive_file.cluster_client["operator"].filename
  }

  provisioner "local-exec" {
    command = self.triggers.apply_command
    environment = {
      KUBECONFIG = self.triggers.kubeconfig
    }
  }

  provisioner "local-exec" {
    when    = destroy
    command = self.triggers.delete_command
    environment = {
      KUBECONFIG = self.triggers.kubeconfig
    }
  }
}