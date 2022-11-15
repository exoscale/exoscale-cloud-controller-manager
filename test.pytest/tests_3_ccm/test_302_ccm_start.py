import json

import pytest

from helpers import kubectl, ioMatch


@pytest.mark.ccm
def test_ccm_rbac(test, tf_control_plane, ccm_rbac, logger):
    # ClusterRole
    (iExit, sStdOut, sStdErr) = kubectl(
        [
            "--output=json",
            "get",
            "clusterrole/system:cloud-controller-manager",
        ],
        kubeconfig=tf_control_plane["kubeconfig_admin"],
    )
    assert iExit == 0
    manifest = json.loads(sStdOut)
    logger.debug(f"[K8s] Asserting CCM RBAC (ClusterRole) manifest:\n{manifest}")

    assert manifest["kind"] == "ClusterRole"
    assert manifest["metadata"]["labels"]["exoscale/manager"] == "exoscale"

    # ClusterRoleBinding
    (iExit, sStdOut, sStdErr) = kubectl(
        [
            "--output=json",
            "get",
            "clusterrolebinding/system:cloud-controller-manager",
        ],
        kubeconfig=tf_control_plane["kubeconfig_admin"],
    )
    assert iExit == 0
    manifest = json.loads(sStdOut)
    logger.debug(f"[K8s] Asserting CCM RBAC (ClusterRoleBinding) manifest:\n{manifest}")

    assert manifest["kind"] == "ClusterRoleBinding"
    assert manifest["subjects"][0]["kind"] == "User"
    assert manifest["subjects"][0]["name"] == f"ccm-{tf_control_plane['cluster_id']}"


@pytest.mark.ccm
def test_ccm_start(test, ccm, logger):
    logger.info(
        "[CCM] Waiting for this CCM to become leader (this may take some time) ..."
    )
    (lines, match, unmatch) = ioMatch(
        ccm,
        matches=["successfully acquired lease kube-system/cloud-controller-manager"],
        unmatches=["error"],
        timeout=test["timeout"]["ccm"]["start"],
        logger=logger,
    )
    assert lines > 0
    if unmatch is not None:
        pytest.exit(unmatch)
    assert match is not None


@pytest.mark.ccm
def test_ccm_api_credentials_valid(test, ccm, logger):
    (lines, match, unmatch) = ioMatch(
        ccm,
        matches=["exoscale-ccm: Exoscale API credentials refreshed, now using valid"],
        timeout=test["timeout"]["ccm"]["refresh_api_credentials"],
        logger=logger,
    )
    assert lines > 0
    assert unmatch is None
    assert match is not None


@pytest.mark.ccm
def test_ccm_node_csr_agent(test, ccm, logger):
    (lines, match, unmatch) = ioMatch(
        ccm,
        matches=["exoscale-ccm: sks-agent: starting node-csr-validation runner"],
        logger=logger,
    )
    assert lines > 0
    assert unmatch is None
    assert match is not None

    (lines, match, unmatch) = ioMatch(
        ccm,
        matches=["exoscale-ccm: sks-agent: watching for pending CSRs"],
        logger=logger,
    )
    assert lines > 0
    assert unmatch is None
    assert match is not None


@pytest.mark.ccm
def test_ccm_started(test, ccm_started, logger):
    assert test["state"]["ccm"]["started"] is True
