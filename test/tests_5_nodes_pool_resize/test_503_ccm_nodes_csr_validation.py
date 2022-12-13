import pytest

from helpers import ioMatch


# Make sure to request (package-scoped, parameterized) 'tf_nodes_pool_resize' fixture,
# such as to trigger each test on each (nodes quantity) update


@pytest.mark.nodes_pool_resize
def test_ccm_node_csrs_approved(test, tf_nodes_pool_resize, ccm, logger):
    nodes_count_delta = test["state"]["nodes"]["all"]["count_delta"]
    if nodes_count_delta <= 0:
        pytest.skip("No new node is expected to be approved")
    csrs = test["state"]["k8s"]["csrs"]
    csrs_pending = {k: v for k, v in csrs.items() if not v["approved"]}
    csrs_approved = list()
    for _ in range(nodes_count_delta):
        (lines, match, unmatch) = ioMatch(
            ccm,
            matches=["re:/exoscale-ccm: sks-agent: CSR (\\S+) approved/i"],
            timeout=test["timeout"]["ccm"]["csr_approve"],
            logger=logger,
        )
        assert lines > 0
        assert match is not None
        assert unmatch is None
        csr = match[1]

        csrs_approved.append(csr)
        if csr in csrs_pending:
            csrs_pending.pop(csr)

    logger.info("[K8s] Approved CSRs: " + ", ".join(csrs_approved))
    assert len(csrs_pending) == 0
