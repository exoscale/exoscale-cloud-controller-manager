import os

import pytest

from helpers import ioMatch


@pytest.mark.ccm
def test_ccm_api_credentials_invalid(
    test, tf_control_plane, ccm, ccm_api_credentials_invalid, logger
):
    # Exoscale API credentials (file)
    # We use a symlink such as to be able to **atomically** change it(s content)
    logger.info("[CCM] Using invalid API credentials ...")
    api_credentials_path = tf_control_plane["ccm_api_credentials"]
    try:
        os.unlink(api_credentials_path)
    except Exception:
        pass
    os.symlink(ccm_api_credentials_invalid, api_credentials_path)
    (lines, match, unmatch) = ioMatch(
        ccm,
        matches=["exoscale-ccm: failed to switch client zone"],
        timeout=test["timeout"]["ccm"]["refresh_api_credentials"],
        logger=logger,
    )
    assert lines > 0
    assert unmatch is None
    assert match is not None


@pytest.mark.ccm
def test_ccm_api_credentials_valid(
    test, tf_control_plane, ccm, ccm_api_credentials_valid, logger
):
    # Exoscale API credentials (file)
    # We use a symlink such as to be able to **atomically** change it(s content)
    logger.info("[CCM] Using valid API credentials ...")
    api_credentials_path = tf_control_plane["ccm_api_credentials"]
    try:
        os.unlink(api_credentials_path)
    except Exception:
        pass
    os.symlink(ccm_api_credentials_valid, api_credentials_path)
    (lines, match, unmatch) = ioMatch(
        ccm,
        matches=["exoscale-ccm: Exoscale API credentials refreshed, now using valid"],
        timeout=test["timeout"]["ccm"]["refresh_api_credentials"],
        logger=logger,
    )
    assert lines > 0
    assert unmatch is None
    assert match is not None
