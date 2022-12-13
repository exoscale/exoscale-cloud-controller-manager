import pytest


@pytest.mark.nlb
def test_cni_started(test, cni_started, logger):
    assert test["state"]["cni"]["started"] is True


@pytest.mark.nlb
def test_ccm_started(test, ccm_started, logger):
    assert test["state"]["ccm"]["started"] is True
