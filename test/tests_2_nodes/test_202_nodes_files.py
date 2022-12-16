import os.path

import pytest


@pytest.mark.nodes
def test_kubeconfig_external_node(tf_nodes):
    path = tf_nodes["external_node_kubeconfig"]
    assert "/output/" in path
    assert "kubeconfig" in path
    assert os.path.exists(path)


@pytest.mark.nodes
def test_k8s_manifests(tf_nodes):
    for manifest in ["hello_external", "hello_ingress"]:
        path = tf_nodes["manifest_" + manifest]
        assert "/output/" in path
        assert os.path.exists(path)
