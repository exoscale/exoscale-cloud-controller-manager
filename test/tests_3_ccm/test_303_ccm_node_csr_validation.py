import pytest

from helpers import ioMatch


@pytest.mark.ccm
def test_ccm_node_csrs_approved(test, tf_nodes, ccm, logger):
    csrs = test["state"]["k8s"]["csrs"]
    csrs_pending = {k: v for k, v in csrs.items() if not v["approved"]}
    csrs_approved = list()
    csrs_invalid = list()
    while len(csrs_pending) > 0:
        (lines, match, unmatch) = ioMatch(
            ccm,
            matches=[
                "re:/exoscale-ccm: sks-agent: CSR (\\S+) (approved|doesn't match)/i"
            ],
            timeout=test["timeout"]["ccm"]["csr_approve"],
            logger=logger,
        )
        assert lines > 0
        assert match is not None
        assert unmatch is None
        event = match[2]
        csr = match[1]

        if event == "approved":
            csrs_approved.append(csr)
        else:
            csrs_invalid.append(csr)
        if csr in csrs_pending:
            csrs_pending.pop(csr)
        else:
            logger.warning(f"[K8s] Unexpected CSR: {csr}")

    logger.info("[CCM] Approved CSRs: " + ", ".join(csrs_approved))
    logger.debug("[CCM] Invalid (ignored) CSRs: " + ", ".join(csrs_invalid))
    assert len(csrs_pending) == 0
