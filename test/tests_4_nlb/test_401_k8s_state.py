import pytest


@pytest.mark.nlb
def test_cni_started(test, cni_started, logger):
    assert test["state"]["cni"]["started"] is True
