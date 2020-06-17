# Deploy k8s cluster with kubeadm for Exoscale cloud controller

## WIP documentation

## Init K8S Master

Follow the instructions in the k8s documentation
https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/create-cluster-kubeadm/

When reaching the control-plane node initialization step (kubeadm init <args>), replace arguments with `--config=kubeadm-config.yml`:

```Shell
sudo kubeadm init --config=kubeadm-config-master.yml
```

## Join Your Nodes

in `kubeadm-config-node.yml` add your Kubeadm token credentials.

```Shell
sudo kubeadm join --config=kubeadm-config-node.yml
```

## Instance Pool K8S Nodes

If you are using Instance Pool with a custom template with all installed K8S components (docker, kubeadm).

You can use this following Cloud-Init to let Instances joins automagically the K8S cluster, when scaling up.

- [node-join-cloud-init.yaml](./node-join-cloud-init.yaml)

If you scaling down the Instance Pool Nodes will be removed automatically from the K8S cluster.