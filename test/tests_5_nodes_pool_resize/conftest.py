import pytest

from helpers import tfNodes


## Fixtures (override)

# Nodes (up/down-sizing)
# TODO <-> BUG[58727]: CCM: NLB service's instance pool ID is not udated when Kubernetes manifest's is
# @pytest.fixture(scope="package", params=[1, 0, 2, 1])
@pytest.fixture(scope="package", params=[2, 1])
def tf_nodes_pool_resize(request, test, tf_control_plane, tf_nodes, ccm, logger):
    # Initialize and apply the Terraform configuration
    tf = tfNodes(test, tf_control_plane, request.param, logger)

    # Yield
    yield tf.output()

    # Teardown
    # -> tf_nodes
