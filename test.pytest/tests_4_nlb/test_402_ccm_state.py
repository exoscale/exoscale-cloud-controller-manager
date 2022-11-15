import pytest


@pytest.mark.nlb
def test_ccm_started(test, ccm_started, logger):
    assert test["state"]["ccm"]["started"] is True
