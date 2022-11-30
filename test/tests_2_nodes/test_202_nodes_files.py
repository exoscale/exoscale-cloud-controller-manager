import os.path

import pytest


@pytest.mark.nodes
def test_k8s_manifests(tf_nodes):
    for manifest in ["hello_external", "hello_ingress"]:
        path = tf_nodes["manifest_" + manifest]
        assert "/output/" in path
        assert os.path.exists(path)
