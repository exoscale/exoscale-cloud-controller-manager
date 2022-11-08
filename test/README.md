# Cloud Controller Manager: Development & Test environment

The purpose of this subdirectory is to allow the deployment of a simple
development & test environment for the Exoscale Cloud Controller Manager.

Required tools are:
- `terraform`
- `kubectl`
- `go`

There is two possible test environments:
- `terraform-kubeadm`: An "unmanaged" Kubernetes cluster, provisioned using kubeadm on top of Ubuntu instances.
- `terraform-sks`: A managed SKS environment, deployed without the default/integrated Cloud Controller Manager instance.

## Run the integration/acceptance tests

Simply execute either `./run-kubeadm-tests.sh` or `./run-sks-tests.sh`.

**Everything** (Terraform, CCM launch, tests, etc.) will be automatically handled for you.

Once done and confident no resources linger behind, you may run `./clean-up.sh` (recommended before
running another test).

## Provisioning the development infrastructure

You will need an Exoscale API key and secret. You can provide them as environment variables:
- `EXOSCALE_API_KEY`
- `EXOSCALE_API_SECRET`

Alternatively, you can create a `terraform.tfvars` file containing them:
```hcl
api_key = "EXOyour-exo-api-key"
api_secret = "your-exo-api-secret"
```

Then, you can create the actual development environment:

```bash
## Initialize Terraform Providers
terraform init
# [output]
# Initializing the backend...
#
# Initializing provider plugins...
#
# ... truncated ...
#
# Terraform has been successfully initialized!
#
# You may now begin working with Terraform. Try running "terraform plan" to see
# any changes that are required for your infrastructure. All Terraform commands
# should now work.
#
# If you ever set or change modules or backend configuration for Terraform,
# rerun this command to reinitialize your working directory. If you forget, other
# commands will detect it and remind you to do so if necessary.

## Create the development infrastructure
terraform apply
# [output]
# Terraform used the selected providers to generate the following execution plan. Resource actions are indicated with the following symbols:
#   + create
#
# Terraform will perform the following actions:
#
# ... truncated ...
#
# exoscale_nlb.external: Creating...
# exoscale_nlb.external: Still creating... [10s elapsed]
# exoscale_nlb.external: Creation complete after 19s [id=8cda6823-8d96-42d5-8516-cbacc19ba150]
#
# Apply complete! Resources: 34 added, 0 changed, 0 destroyed.
```

Once terraform completed provisioning tasks (it usually takes around 3mn), you have:
- a Kubernetes cluster (without CCM) in your Exoscale account
- the operator Kubeconfig file (`operator.kubeconfig`) for use with kubectl
- the ccm Kubeconfig for use by the CCM (`ccm.kubeconfig`).

You can source the `.env` file to set up some useful environment variables and aliases:
- `EXOSCALE_ZONE` environment variable: required for the CCM to work properly, this variable exposes the zone in which the
development cluster was deployed
- `KUBECONFIG` environment variable: allows interaction with the cluster, using `system:master` privileges
- `approve-csr` alias: automatically approve all pending CSR. This can be used in rare situations for troubleshooting because one of the features of the CCM is to automatically approve CSRs.
- `go-run-ccm` alias: this alias calls `go run` on your current code base and authenticates directly to the target development infrastructure.

The CCM will also require credentials as well (you need to provide them in environment variables too):
- `EXOSCALE_API_KEY`
- `EXOSCALE_API_SECRET`

## Interacting with the development infrastructure

Move to this directory, then source the `.env` file:

```bash
source terraform-kubeadm/.env
```

or

```bash
source terraform-sks/.env
```

Alternatively, you can just export a `KUBECONFIG` environment variable with the full path to `operator.kubeconfig`.
Then, you can now interact with the cluster using the standard `kubectl` tool. Example:

```bash
kubectl get nodes,csr,pods -A
# [output]
# NAME                    STATUS   ROLES    AGE   VERSION
# node/pool-1d0f4-dnofp   Ready    <none>   56s   v1.23.6
# node/pool-1d0f4-wamkb   Ready    <none>   59s   v1.23.6
#
# NAME                                                      AGE   SIGNERNAME                                    REQUESTOR                      REQUESTEDDURATION   CONDITION
# certificatesigningrequest.certificates.k8s.io/csr-42gzg   61s   kubernetes.io/kube-apiserver-client-kubelet   system:bootstrap:9f8e1d        <none>              Approved,Issued
# certificatesigningrequest.certificates.k8s.io/csr-7mzx2   57s   kubernetes.io/kubelet-serving                 system:node:pool-1d0f4-wamkb   <none>              Pending
# certificatesigningrequest.certificates.k8s.io/csr-ml72d   64s   kubernetes.io/kube-apiserver-client-kubelet   system:bootstrap:9f8e1d        <none>              Approved,Issued
# certificatesigningrequest.certificates.k8s.io/csr-zg5zl   55s   kubernetes.io/kubelet-serving                 system:node:pool-1d0f4-dnofp   <none>              Pending
#
# NAMESPACE     NAME                                          READY   STATUS    RESTARTS   AGE
# kube-system   pod/calico-kube-controllers-7c845d499-rcs8r   1/1     Running   0          116s
# kube-system   pod/calico-node-2blz4                         1/1     Running   0          59s
# kube-system   pod/calico-node-vpjw7                         1/1     Running   0          56s
# kube-system   pod/coredns-f647577f6-8hxsm                   1/1     Running   0          108s
# kube-system   pod/coredns-f647577f6-nkf54                   1/1     Running   0          108s
# kube-system   pod/konnectivity-agent-67ff7d99b5-sx8bg       1/1     Running   0          104s
# kube-system   pod/konnectivity-agent-67ff7d99b5-th28f       1/1     Running   0          104s
# kube-system   pod/kube-proxy-7rc9j                          1/1     Running   0          56s
# kube-system   pod/kube-proxy-l66s2                          1/1     Running   0          59s
# kube-system   pod/metrics-server-875d768c4-sks45            0/1     Running   0          103s
```

NOTE: You can see in the above output that the metrics-server Pod is not ready. This is because CSR must be approved to allow
it to run properly.

## Running the CCM locally

Move to this directory, then source the `.env` file:

```bash
source terraform-kubeadm/.env
```

or

```bash
source terraform-sks/.env
```

As already mentioned, the CCM will require Exoscale credentials to validate instances. You have to export them
as environment variables: `EXOSCALE_API_KEY` and `EXOSCALE_API_SECRET`.

Then, you can run the CCM simply by invoking the `go-run-ccm` alias. Under the wood, this command will call 
`go run` and set proper flags to let the CCM authenticate correctly on the remote API server.

```bash
go-run-ccm
# [output]
# Flag --allow-untagged-cloud has been deprecated, This flag is deprecated and will be removed in a future release. A cluster-id will be required on cloud instances.
# I0531 15:29:25.467196  119973 serving.go:348] Generated self-signed cert in-memory
# W0531 15:29:25.891155  119973 main.go:74] detected a cluster without a ClusterID.  A ClusterID will be required in the future.  Please tag your cluster to avoid any future issues
# I0531 15:29:25.891184  119973 controllermanager.go:143] Version: v0.0.0-master+$Format:%H$
# I0531 15:29:25.928589  119973 requestheader_controller.go:169] Starting RequestHeaderAuthRequestController
# I0531 15:29:25.928603  119973 configmap_cafile_content.go:202] "Starting controller" name="client-ca::kube-system::extension-apiserver-authentication::client-ca-file"
# I0531 15:29:25.928647  119973 configmap_cafile_content.go:202] "Starting controller" name="client-ca::kube-system::extension-apiserver-authentication::requestheader-client-ca-file"
# I0531 15:29:25.928672  119973 shared_informer.go:255] Waiting for caches to sync for client-ca::kube-system::extension-apiserver-authentication::client-ca-file
# I0531 15:29:25.928689  119973 shared_informer.go:255] Waiting for caches to sync for client-ca::kube-system::extension-apiserver-authentication::requestheader-client-ca-file
# I0531 15:29:25.928649  119973 shared_informer.go:255] Waiting for caches to sync for RequestHeaderAuthRequestController
# I0531 15:29:25.928824  119973 secure_serving.go:210] Serving securely on [::]:10258
# I0531 15:29:25.928885  119973 tlsconfig.go:240] "Starting DynamicServingCertificateController"
# I0531 15:29:25.929175  119973 leaderelection.go:248] attempting to acquire leader lease kube-system/cloud-controller-manager...
# I0531 15:29:26.029788  119973 shared_informer.go:262] Caches are synced for RequestHeaderAuthRequestController
# I0531 15:29:26.029828  119973 shared_informer.go:262] Caches are synced for client-ca::kube-system::extension-apiserver-authentication::requestheader-client-ca-file
# I0531 15:29:26.029801  119973 shared_informer.go:262] Caches are synced for client-ca::kube-system::extension-apiserver-authentication::client-ca-file
# I0531 15:29:26.128644  119973 leaderelection.go:258] successfully acquired lease kube-system/cloud-controller-manager
# I0531 15:29:26.128917  119973 event.go:294] "Event occurred" object="kube-system/cloud-controller-manager" fieldPath="" kind="Lease" apiVersion="coordination.k8s.io/v1" type="Normal" reason="LeaderElection" message="philxps_d44cea51-fd3d-4360-a6e8-3935242cd981 became leader"
# I0531 15:29:27.665258  119973 request.go:601] Waited for 1.01700277s due to client-side throttling, not priority and fairness, request: GET:https://4c3bdba6-c65b-4580-96c4-4825b97b0c4d.sks-ch-gva-2.exo.io:443/apis/authentication.k8s.io/v1
# E0531 15:29:28.396827  119973 controllermanager.go:463] unable to get all supported resources from server: unable to retrieve the complete list of server APIs: metrics.k8s.io/v1beta1: the server is currently unable to handle the request
# I0531 15:29:28.397120  119973 log.go:16] exoscale-ccm: using Exoscale actual API credentials (key + secret)
# I0531 15:29:28.397476  119973 controllermanager.go:291] Started "service"
# W0531 15:29:28.397495  119973 core.go:111] --configure-cloud-routes is set, but cloud provider does not support routes. Will not configure cloud provider routes.
# W0531 15:29:28.397509  119973 controllermanager.go:279] Skipping "route"
# I0531 15:29:28.397523  119973 controller.go:233] Starting service controller
# I0531 15:29:28.397539  119973 shared_informer.go:255] Waiting for caches to sync for service
# I0531 15:29:28.397726  119973 node_controller.go:118] Sending events to api server.
# I0531 15:29:28.397780  119973 controllermanager.go:291] Started "cloud-node"
# I0531 15:29:28.397880  119973 node_controller.go:157] Waiting for informer caches to sync
# I0531 15:29:28.397971  119973 node_lifecycle_controller.go:77] Sending events to api server
# I0531 15:29:28.397995  119973 controllermanager.go:291] Started "cloud-node-lifecycle"
# I0531 15:29:28.498266  119973 shared_informer.go:262] Caches are synced for service
```

NOTE: if you want to enable automatic CSR validation from the CCM, you will have to `export EXOSCALE_SKS_AGENT_RUNNERS=node-csr-validation`
before running it. This will add something like in logs:

```bash
# [output]
# I0531 15:34:53.867454  120931 log.go:16] exoscale-ccm: sks-agent: CSR csr-7mzx2 approved
# I0531 15:34:54.154227  120931 log.go:16] exoscale-ccm: sks-agent: CSR csr-zg5zl approved
```

## Cleaning up resources

Cleaning resources from your Exoscale account and removing generated assets is as simple as running `terraform destroy` in the
`terraform-kubeadm` or `terraform-sks` directory.
