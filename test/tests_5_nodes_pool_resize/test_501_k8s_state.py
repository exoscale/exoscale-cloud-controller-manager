import pytest

# Make sure to request (package-scoped, parameterized) 'tf_nodes_pool_resize' fixture,
# such as to trigger each test on each (nodes quantity) update


@pytest.mark.nodes_pool_resize
def test_state_change(test, tf_nodes_pool_resize, logger):
    logger.debug(
        f"[Test] Nodes quantity: {test['state']['nodes']['pool']['size_previous']} -> {test['state']['nodes']['pool']['size']}"
    )


@pytest.mark.nodes_pool_resize
def test_k8s_nodes(test, tf_nodes_pool_resize, logger):
    nodes = test["state"]["k8s"]["nodes"]
    logger.info(
        "[K8s] Available nodes: "
        + ", ".join([f"{k} ({v['selfie']})" for k, v in nodes.items()])
    )


@pytest.mark.nodes_pool_resize
def test_k8s_node_csrs(test, tf_nodes_pool_resize, logger):
    csrs = test["state"]["k8s"]["csrs"]
    logger.info(
        "[K8s] Node CSRs: "
        + ", ".join([f"{k} ({v['selfie']})" for k, v in csrs.items()])
    )
    assert len(csrs) >= test["state"]["nodes"]["all"]["count_delta"]
