import pytest

from helpers import ioMatch


@pytest.mark.the_end
def test_ccm_tail(test, ccm, logger):
    logger.debug("[CCM] Dumping remaining CCM output ...")
    (lines, match, unmatch) = ioMatch(
        ccm,
        matches=[
            "The end is nigh..."
        ],  # Dummy, invalid match to read all remaining output lines
        timeout=test["timeout"]["ccm"]["the_end"],
        logger=logger,
    )
