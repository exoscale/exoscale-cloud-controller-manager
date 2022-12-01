# CCM Acceptance Tests

Exoscale Cloud Controller Manager (CCM) acceptance tests are required to pass before each new
[release](https://github.com/exoscale/exoscale-cloud-controller-manager/releases/).

They leverage [pytest](https://pytest.org) and [tftest](https://github.com/GoogleCloudPlatform/terraform-python-testing-helper/releases) to:

* instantiate a full, independent test infrastructure, using [Terraform](terraform.io/)

* install and configure [Kubernetes](https://kubernetes.io/) resources

* tests the various aspect of the CCM:
  - Exoscale Instances / Nodes (Pool) validation (as Kubernetes Nodes)
  - Kubernetes (nodes) Certificate Signing Requests (CSRs) automated validation
  - Exoscale Network Load-Balancer (NLB) instantiation and services configuration
    (along Kubernetes Services and Ingresses)
  - etc.

The tests come in two "types", which may be chosen with the `TEST_CCM_TYPE` environment variable:

* `sks` (default): using an [Exoscale SKS](https://www.exoscale.com/sks/) cluster and nodepool

* `kubeadm`: using a "stock" Kubernetes cluster instantiated with [kubeadm](https://kubernetes.io/docs/reference/setup-tools/kubeadm/)
  along an Exoscale instance pool (as worker nodes)


## Install the required dependencies

### terraform

Donwload and install `terraform` from its [release/download](https://releases.hashicorp.com/terraform/) page.

You may point to an out-of-`PATH` version using the ad-hoc environment variable:

``` bash
export TERRAFORM='/path/to/terraform'
```

### kubectl

Donwload and install `kubectl` from its [release/download](https://github.com/kubernetes/kubernetes/releases/) page.

You may point to an out-of-`PATH` version using the ad-hoc environment variable:

``` bash
export KUBECTL='/path/to/kubectl'
```

### Exoscale CLI

Donwload and install `exo` (CLI) from its [release/download](https://github.com/exoscale/cli/releases/) page.

You may point to an out-of-`PATH` version using the ad-hoc environment variable:

``` bash
export EXOCLI='/path/to/exo'
```

### pytest

``` bash
pip install --upgrade -r python-requirements.txt
```


## Run the tests

Run the tests with minimal verbosity (for successful tests reporting in Pull-Requests):

``` bash
# Set your Exoscale (API) credentials
export EXOSCALE_API_KEY='EXO...'
export EXOSCALE_API_SECRET='...'

# Run the tests
pytest
```

For development or troubleshooting, consider increasing verbosity:

``` bash
# Output function calls
pytest -v

# Output function calls and INFO-level logs
pytest -v --log-cli-level=INFO

# Output function calls and DEBUG-level logs
pytest -v --log-cli-level=DEBUG
```

Or use the ad-hoc `./run` helper:

``` bash
USAGE: run [<options>] [-- [<pytest arguments> ...]]

SYNOPSIS:
  Run the Exoscale CCM acceptance tests.

OPTIONS:

  -t, --type {sks|kubeadm}
    Tests type (may be specified multiple times; defaults: all)

  -l, --level <level>
    Verbosity level:
    - default:    be brief (please use in Pull-Requests)
    - 1-line (1): show individual tests (1 per line)
    - info (I):   peak into the tests process
    - debug (D):  deep-dive into the tests process
```


## Troubleshoot the tests

Consider targeting your tests:

``` bash
# Include only control plane, nodes and CCM tests

# (using packages <-> subdirectories targeting)
pytest \
  tests_1_control_plane \
  tests_2_nodes \
  tests_3_ccm

# (using markers)
pytest \
  -m 'control_plane or nodes or ccm'

# Exclude environment and NLB tests
# (using markers)
pytest \
  -m 'not environment and not nlb'

# List available markers
pytest --markers | grep test-ccm:serie
# [output]
#@pytest.mark.environment: [test-ccm:serie] Validate the test environment itself (tests_0_environment/test_0xx.py)
#@pytest.mark.control_plane: [test-ccm:serie] Test the Terraformed control plane (tests_1_control_plane/test_1xx.py)
#@pytest.mark.nodes: [test-ccm:serie] Test the Terraformed nodes (tests_2_nodes/test_2xx.py)
#@pytest.mark.ccm: [test-ccm:serie] Test the CCM itself (tests_3_ccm/test_3xx.py)
#@pytest.mark.nlb: [test-ccm:serie] Test CCM-managed NLB and Kubernetes (LoadBalancer) integration  (tests_4_nlb/test_4xx.py)
#@pytest.mark.nodes_pool_resize: [test-ccm:serie] Test CCM-managed nodes pool up-/down-sizing (tests_5_nodes_pool_resize/test_5xx.py)
#@pytest.mark.the_end: [test-ccm:serie] Extraneous finalization outputs (test_9xx.py)

# Advanced targeting (<-> scope)
pytest \
  tests_0_environment \                                      # package (subdirectory)
  tests_1_control_plane/test_101_control_plane.py \          # module (file)
  tests_2_nodes/test_202_nodes_files.py::test_k8s_manifests  # function
```

Some enviroment variables are also available to fine-tune the tests behavior:

* `export TEST_CCM_NO_TF_TEARDOWN=yes`: do not "tear down" (destroy) the Terraform-ed infrastructure

* `export TEST_CCM_NO_CCM_TEARDOWN=yes`: do not "tear down" (destroy) the CCM executable and resources

* `export TEST_CCM_NO_NLB_TEARDOWN=yes`: do not "tear down" (destroy) the NLB resources

You may need also to get familiar with [Kubernetes Cloud Controler](https://kubernetes.io/docs/concepts/architecture/cloud-controller/)
concepts in order to keep track with the CCM output (logs).


## When tests turn awry

Example given here for `sks` in the `ch-gva-2` zone:

### Manually setup the tests environment

Start by Terraform-ing the Kubernetes control plane and nodes:

``` bash
# Create the control plane
pushd ./terraform/sks/control-plane
terraform init
terraform apply
popd

# Create nodes
pushd ./terraform/sks/nodes
terraform init
terraform apply
popd
```

Then load the shell environment:

``` bash
# Load environment variables (KUBECONFIG, etc.) and aliases (go-run-ccm, etc.)
source ./terraform/sks/control-plane/output/shell.env
```

### Introspect the tests environment

* using Kubernetes `kubectl`:

``` bash
# Nodes
kubectl get nodes -o wide
kubectl get nodes -o json \
  | jq '.items[]|.metadata.name, (.status.conditions[]|select(.type=="Ready")).status, .status.addresses'

# CSRs
kubectl get csr
kubectl get csr -o json \
  | jq '.items[]|select(.spec.username|test("test-ccm"))|.metadata.name, .spec.username, (.status.conditions[]|select(.type=="Approved")).status'

# Services
kubectl get services -A
```

* using Exoscale CLI (`exo`):

``` bash
# SKS Cluster
exo -O json -z ch-gva-2 compute sks list \
  | jq '.[]|select(.name|test("^test-ccm"))'
exo -O json -z ch-gva-2 compute sks show ${sks_cluster_id} \
  | jq

# SKS Nodepool
exo -O json -z ch-gva-2 compute sks nodepool list \
  | jq '.[]|select(.name|test("^test-ccm"))'
exo -O json -z ch-gva-2 compute sks nodepool show ${sks_cluster_id} ${sks_nodepool_id} \
| jq

# Instance Pool (<-> SKS Nodepool)
exo -O json -z ch-gva-2 compute instance-pool list \
  | jq '.[]|select(.name|test("^nodepool-test-ccm"))'
exo -O json -z ch-gva-2 compute instance-pool show ${instance_pool_id} \
  | jq

# Instances
exo -O json -z ch-gva-2 compute instance list \
  | jq '.[]|select(.name|test("^test-ccm"))'
exo -O json -z ch-gva-2 compute instance show ${instance_id} \
  | jq

# Load-Balancer (NLB)
exo -O json -z ch-gva-2 compute load-balancer list | jq
exo -O json -z ch-gva-2 compute load-balancer show ${nlb_id} | jq

# NLB Services
exo -O json -z ch-gva-2 compute load-balancer show ${nlb_id} ${nlb_service_id} \
  | jq
```

* using the ad-hoc `./show` helper:

``` bash
USAGE: show <scope> <item(s)> [-- [<kubectl/exo arguments> ...]]

SYNOPSIS:
  Query and display various tests-relevant troubleshooting data.

SCOPES/ITEMS:

  k8s
    nodes, csrs, services

  exo (cli)
    [sks_]cluster[s], [sks_]nodepool[s], instancepool[s], instance[s],
    loadbalancer[s] (nlb[s]), service
```

* logging into the control plane or worker nodes (only for `kubeadm` test type):

``` bash
# Log into node (obtain its IP address using "instances" commands above)
ssh -i terraform/kubeadm/control-plane/output/ssh.id_ed25519 ubuntu@<node-ip-address>
```

### Clean-up the tests environment

When certain no resources (especially Terraformed) linger around:

``` bash
# Clean-up tests files and directories
./cleanup

# Clean-up the shell environment (variables and aliases)
source ./cleanup
```


## Notes and gotchas

* Exoscale CCM requires the `providerID: exoscale://<instance-uuid>` to be set in the kubelet
  [config.yaml](https://kubernetes.io/docs/reference/config-api/kubelet-config.v1beta1/#kubelet-config-k8s-io-v1beta1-KubeletConfiguration)
  _before_ it starts. Please note that once a node/kubelet has been successfully registered, nothing
  short of (kubectl-)_deleting_ the node and restarting kubelet will make Kubernetes update the node
  `spec`.

* As of Kubernets 1.25.4, the `--cloud-provider=external` _must_ still be specified in the kubelet
  [command-line options](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/)
  (although deprecated)
