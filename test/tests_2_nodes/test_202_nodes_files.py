import os.path

import pytest

from helpers import TEST_CCM_TYPE


@pytest.mark.nodes
@pytest.mark.skipif(
    TEST_CCM_TYPE not in ["kubeadm"],
    reason="This test may only be performed for 'kubeadm' type (<-> external node kubeconfig/username)",
)
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
