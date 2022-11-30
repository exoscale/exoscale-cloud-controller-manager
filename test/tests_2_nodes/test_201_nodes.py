import pytest

from helpers import reUUID, reIPv4, reIPv6


@pytest.mark.nodes
def test_nodes_outputs(test, tf_nodes):
    # Test
    assert "test_id" in tf_nodes
    assert "test_name" in tf_nodes

    # Exoscale
    assert "exoscale_zone" in tf_nodes
    assert "exoscale_environment" in tf_nodes

    # Cluster
    assert "cluster_id" in tf_nodes
    if test["type"] not in ["kubeadm"]:
        assert reUUID.match(tf_nodes["cluster_id"])

    # Nodes

    # (pool)
    assert "nodepool_id" in tf_nodes
    if test["type"] not in ["kubeadm"]:
        assert reUUID.match(tf_nodes["nodepool_id"])
    assert "instancepool_id" in tf_nodes
    assert reUUID.match(tf_nodes["instancepool_id"])

    # (external)
    if test["type"] not in ["sks"]:
        assert "external_node_name" in tf_nodes
        assert "external_node_ipv4" in tf_nodes
        assert reIPv4.match(tf_nodes["external_node_ipv4"])
        assert "external_node_ipv6" in tf_nodes
        assert reIPv6.match(tf_nodes["external_node_ipv6"])

    # Load balancer (NLB)
    assert "external_nlb_id" in tf_nodes
    assert reUUID.match(tf_nodes["external_nlb_id"])
    assert "external_nlb_ipv4" in tf_nodes
    assert reIPv4.match(tf_nodes["external_nlb_ipv4"])
    # assert 'external_nlb_ipv6' in tf_nodes
    # assert reIPv6.match(tf_nodes['external_nlb_ipv6'])

    # Kubernetes manifests
    # (applications)
    for manifest in ["hello_external", "hello_ingress"]:
        assert "manifest_" + manifest in tf_nodes
