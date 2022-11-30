import pytest


@pytest.mark.nodes_pool_resize
def test_cni_started(test, cni_started, logger):
    assert test["state"]["cni"]["started"] is True
