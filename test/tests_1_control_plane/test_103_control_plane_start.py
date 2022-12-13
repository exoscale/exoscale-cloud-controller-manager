import json
import pytest

from helpers import kubectl


@pytest.mark.control_plane
def test_k8s_version(test, tf_control_plane, logger):
    (iExit, sStdOut, sStdErr) = kubectl(
        [
            "--output=json",
            "version",
        ],
        kubeconfig=tf_control_plane["kubeconfig_admin"],
    )
    assert iExit == 0
    output = json.loads(sStdOut)
    version = output["serverVersion"]["gitVersion"]
    logger.info(f"[K8s] Kubernetes version: {version}")

    version_major_minor = "%s.%s" % (
        output["serverVersion"]["major"],
        output["serverVersion"]["minor"],
    )
    assert version_major_minor in ["1.24", "1.25", "1.26"]


@pytest.mark.control_plane
def test_cni_started(test, cni_started, logger):
    assert test["state"]["cni"]["started"] is True
