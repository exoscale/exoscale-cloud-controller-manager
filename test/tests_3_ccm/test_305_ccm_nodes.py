import pytest

from helpers import TEST_CCM_TYPE, ioMatch


@pytest.mark.ccm
@pytest.mark.xfail(
    TEST_CCM_TYPE == "sks",
    reason="TODO/BUG[58670]: CCM: provider ID cannot be empty",
)
def test_ccm_nodes_init(test, tf_nodes, ccm, logger):
    nodes_count_delta = test["state"]["nodes"]["all"]["count_delta"]
    nodes_initialized = list()
    for _ in range(nodes_count_delta):
        (lines, match, unmatch) = ioMatch(
            ccm,
            matches=["re:/Successfully initialized node (\\S+) with cloud provider/i"],
            timeout=test["timeout"]["ccm"]["node_init"],
            logger=logger,
        )
        assert lines > 0
        assert match is not None
        node = match[1]

        nodes_initialized.append(node)

    logger.info("[CCM] Initialized nodes: " + ", ".join(nodes_initialized))
