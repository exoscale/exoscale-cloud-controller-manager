import pytest

from helpers import tfNodes


## Fixtures (override)

# Nodes (0-sized)
# (make sure we start up/down-sizing nodes with a clean slate)
@pytest.fixture(scope="module")
def tf_nodes_reset(test, tf_control_plane, tf_nodes, ccm, logger):
    # Initialize and apply the Terraform configuration
    tf = tfNodes(test, tf_control_plane, 0, logger)

    # Yield
    yield tf.output()

    # Teardown
    # -> tf_nodes


## Tests
@pytest.mark.nodes_pool_resize
def test_cni_started(test, cni_started, logger):
    assert test["state"]["cni"]["started"] is True


@pytest.mark.nodes_pool_resize
def test_ccm_started(test, ccm_started, logger):
    assert test["state"]["ccm"]["started"] is True


# TODO <-> BUG[58727]: CCM: NLB service's instance pool ID is not udated when Kubernetes manifest's is
# @pytest.mark.nodes_pool_resize
# def test_nodes_reset(test, tf_nodes_reset, logger):
#     assert len(test["state"]["k8s"]["nodes"]) == 0
