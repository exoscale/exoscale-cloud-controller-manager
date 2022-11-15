import pytest

from helpers import k8sGetNodes

# Make sure to request (package-scoped, parameterized) 'tf_nodes_pool_resize' fixture,
# such as to trigger each test on each (nodes quantity) update


@pytest.mark.nodes_pool_resize
def test_k8s_nodes(test, tf_control_plane, tf_nodes_pool_resize, logger):
    nodes = k8sGetNodes(kubeconfig=tf_control_plane["kubeconfig_admin"])
    logger.info(
        "[K8s] Available nodes: "
        + ", ".join([f"{k} ({v['selfie']})" for k, v in nodes.items()])
    )
    assert len(nodes) == test["state"]["nodes"]["all"]["count"]
