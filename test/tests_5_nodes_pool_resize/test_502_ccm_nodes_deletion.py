import pytest

from helpers import TEST_CCM_TYPE, ioMatch

# Make sure to request (package-scoped, parameterized) 'tf_nodes_pool_resize' fixture,
# such as to trigger each test on each (nodes quantity) update


@pytest.mark.nodes_pool_resize
@pytest.mark.xfail(
    TEST_CCM_TYPE == "sks",
    reason="TODO/BUG[58670]: CCM: provider ID cannot be empty",
)
def test_ccm_node_deletion(test, tf_nodes_pool_resize, ccm, logger):
    nodes_count_delta = test["state"]["nodes"]["all"]["count_delta"]
    if nodes_count_delta >= 0:
        pytest.skip("No existing node is expected to be deleted")
    nodes = list()
    for _ in range(-nodes_count_delta):
        (lines, match, unmatch) = ioMatch(
            ccm,
            matches=[
                "re:/deleting node since it is no longer present in cloud provider: (\\S+)/i"
            ],
            timeout=test["timeout"]["node"]["delete"],
            logger=logger,
        )
        assert lines > 0
        assert match is not None
        assert unmatch is None
        node = match[1]

        nodes.append(node)

    logger.info("[K8s] Deleted nodes: " + ", ".join(nodes))
