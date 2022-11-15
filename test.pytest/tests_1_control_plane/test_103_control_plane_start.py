import pytest


@pytest.mark.control_plane
def test_cni_started(test, cni_started, logger):
    assert test["state"]["cni"]["started"] is True
