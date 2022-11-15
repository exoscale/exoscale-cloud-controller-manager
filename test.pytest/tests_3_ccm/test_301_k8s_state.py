import pytest


@pytest.mark.ccm
def test_cni_started(test, cni_started, logger):
    assert test["state"]["cni"]["started"] is True


@pytest.mark.ccm
def test_k8s_nodes(test, tf_nodes, logger):
    nodes = test["state"]["k8s"]["nodes"]
    logger.info(
        "[K8s] Available nodes: "
        + ", ".join([f"{k} ({v['selfie']})" for k, v in nodes.items()])
    )
    assert (
        len(nodes)
        == test["state"]["nodes"]["pool"]["size"]
        + test["state"]["nodes"]["external"]["quantity"]
    )


@pytest.mark.ccm
def test_k8s_node_csrs(test, tf_nodes, logger):
    csrs = test["state"]["k8s"]["csrs"]
    logger.info(
        "[K8s] Node CSRs: "
        + ", ".join([f"{k} ({v['selfie']})" for k, v in csrs.items()])
    )
    csrs_pending = {k: v for k, v in csrs.items() if not v["approved"]}
    for k, v in csrs_pending.items():
        logger.debug(f"[K8s] Pending CSR: {k} <-> {v}")
    assert len(csrs_pending) >= test["state"]["nodes"]["all"]["count_delta"]
