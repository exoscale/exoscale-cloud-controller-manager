import json
import logging
import os

import pytest

from helpers import (
    TEST_CCM_TYPE,
    execEnvironment,
    execForeground,
    execBackground,
    tfControlPlane,
    tfNodes,
    ioMatch,
    kubectl,
)

## Logging

# tftest
logging.getLogger("tftest").setLevel(logging.ERROR)


## Fixtures

# Test (configuration and state tracking)
@pytest.fixture(scope="session")
def test():
    test_directory = os.path.dirname(os.path.abspath(__file__))
    return {
        "directory": test_directory,
        "type": TEST_CCM_TYPE,
        "terraform": {"directory": os.path.join(test_directory, "terraform")},
        # Timeouts, used throughout test functions
        "timeout": {
            "ccm": {
                "start": 120,
                "refresh_api_credentials": 30,
                "invalid_api_credentials": 30,
                "csr_approve": 60,
                "node_init": 60,
                "the_end": 60,
            },
            "node": {
                "start": 180,
                "delete": 60,
            },
            "nlb": {
                "start": 120,
                "service": {
                    "start": 180,
                    "healthcheck": {
                        "success": 120,
                    },
                },
            },
        },
        # Internal state tracking (throughout the test session)
        "state": {
            "cni": {
                "started": False,
            },
            "ccm": {
                "started": False,
            },
            "nodes": {
                "pool": {
                    "size_previous": -1,
                    "size": -1,
                },
                "external": {
                    "quantity_previous": -1,
                    "quantity": -1,
                },
                "all": {
                    "count_previous": -1,
                    "count": -1,
                    "count_delta": 0,
                },
            },
            "nlb": {
                "external": {
                    "name": None,
                    "id": None,
                    "ipv4": None,
                    "ipv6": None,
                    "services": {},  # "port": {"name": ..., "id": ...}
                },
                "k8s": {
                    "name": None,
                    "id": None,
                    "ipv4": None,
                    "ipv6": None,
                    "services": {},  # "port": {"name": ..., "id": ...}
                },
            },
            "k8s": {
                "nodes": {},  # <-> k8sGetNodes()
                "csrs": {},  # <-> k8sGetNodeCSRs()
            },
        },
    }


@pytest.fixture(scope="session")
def logger():
    return logging.getLogger("test-ccm")


# Terraform
@pytest.fixture(scope="session")
def tf_control_plane(test, logger):
    # Initialize and apply the Terraform configuration
    tf = tfControlPlane(test, logger)

    # Yield
    yield tf.output()

    # Teardown
    if not os.getenv("TEST_CCM_NO_TF_TEARDOWN"):
        logger.info(
            "[Terraform] Destroying the control-plane infrastructure (this may take some time) ..."
        )
        tf.destroy()


@pytest.fixture(scope="session")
def tf_nodes(test, tf_control_plane, logger):
    # Initialize and apply the Terraform configuration
    tf = tfNodes(test, tf_control_plane, 1, logger)

    # Yield
    yield tf.output()

    # Teardown
    if not os.getenv("TEST_CCM_NO_TF_TEARDOWN"):
        logger.info(
            "[Terraform] Destroying the nodes infrastructure (this may take some time) ..."
        )
        tf.destroy()


# Container Network Interface (CNI)
@pytest.fixture(scope="session")
def cni(test, tf_control_plane, logger):
    cni = tf_control_plane["kubernetes_cni"]
    manifest = f"cni-{cni}.yaml"
    if test["type"] in ["sks"]:
        logger.debug(
            f"[K8s] Chosen CNI manifest ({manifest}) is intrinsically applied; skipping"
        )
    else:
        logger.info(f"[K8s] Applying CNI manifest ({manifest}) ...")
        manifest = os.path.join(test["directory"], "resources", "manifests", manifest)
        # We MUST use 'kubectl create' given `kubectl apply' leads to "The CustomResourceDefinition
        # "installations.operator.tigera.io" is invalid: metadata.annotations: Too long: must have
        # at most 262144 bytes" when using 'calico' as CNI.
        # Hence, we MUST tolerate exit status being non-zero, given 'kubectl create' will fail to
        # "re-created" existing resources.
        kubectl(
            [
                "create",
                f"--filename={manifest}",
            ],
            kubeconfig=tf_control_plane["kubeconfig_admin"],
            pyexit=False,
        )

    # Yield
    yield manifest


# This fixture ought to be used at the start of each package (tests_* subdirectory)
# such to make sure the CNI started successfully (even if only a subset of
# tests packages/modules)
@pytest.fixture(scope="session")
def cni_started(test, tf_control_plane, cni, logger):
    if test["state"]["cni"]["started"]:
        logger.debug("[K8s] CNI already started; skipping")

    # Wait for CNI to start
    if test["type"] not in ["sks"]:
        node = tf_control_plane["control_plane_node"]
        timeout = test["timeout"]["node"]["start"]
        kubectl(
            [
                "--output=json",
                "wait",
                f"--timeout={timeout}s",
                "--for=condition=Ready",
                f"node/{node}",
            ],
            kubeconfig=tf_control_plane["kubeconfig_admin"],
            pyexit=True,
        )
        logger.info("[K8s] CNI started successfully")
    else:
        logger.info("[K8s] CNI started successfully")
    test["state"]["cni"]["started"] = True


# Cloud Controller Manager (CCM)
@pytest.fixture(scope="session")
def ccm_exe(test, tf_control_plane, logger):
    # Build the CCM executable
    # ('go run ...' doesn't return the **running process** PID, which we need for proper Teardown)
    ccm_main_path = tf_control_plane["ccm_main"]
    ccm_exe_path = tf_control_plane["ccm_exe"]
    if not os.path.exists(ccm_exe_path) or not os.getenv("TEST_CCM_NO_CCM_TEARDOWN"):
        logger.info("[CCM] Building the CCM executable ...")
        (iExit, sStdOut, sStdErr) = execForeground(
            [
                "go",
                "build",
                "-o",
                ccm_exe_path,
                ccm_main_path,
            ],
            pyexit=True,
        )
        if not os.path.exists(ccm_exe_path):
            pytest.exit("[CCM] Failed to 'go build' executable")
        logger.debug(f"[CCM] CCM executable succesfully built; path=={ccm_exe_path}")
    else:
        logger.info("[CCM] Using the existing CCM executable")

    # Yield
    yield ccm_exe_path

    # Teardown
    if not os.getenv("TEST_CCM_NO_CCM_TEARDOWN"):
        os.unlink(ccm_exe_path)


# Invalid credentials may be used to "hold" the CCM from doing its work (when necessary for the tests)
@pytest.fixture(scope="session")
def ccm_api_credentials_invalid(test, tf_control_plane, logger):
    # Exoscale API credentials (file)
    # We use an alternate filename such as to be able to **symlink** it to the actual one
    logger.info("[CCM] Creating invalid API credentials (file) ...")
    api_credentials = {
        "name": "invalid",
        "api_key": "EXO",
        "api_secret": "n/a",
    }
    api_credentials_path = tf_control_plane["ccm_api_credentials"].replace(
        ".json", "-invalid.json"
    )
    with os.fdopen(
        os.open(
            api_credentials_path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, mode=0o600
        ),
        "w",
    ) as api_credentials_file:
        json.dump(api_credentials, api_credentials_file)
    logger.debug(
        f"[CCM] Invalid API credentials successfully created; path={api_credentials_path}"
    )

    # Yield
    yield api_credentials_path

    # Teardown
    if not os.getenv("TEST_CCM_NO_CCM_TEARDOWN"):
        os.unlink(api_credentials_path)


@pytest.fixture(scope="session")
def ccm_api_credentials_valid(test, tf_control_plane, logger):
    # Exoscale API credentials (file)
    # We use an alternate filename such as to be able to **symlink** it to the actual one
    logger.info("[CCM] Creating valid API credentials (file) ...")
    api_key = os.getenv("EXOSCALE_API_KEY")
    api_secret = os.getenv("EXOSCALE_API_SECRET")
    api_credentials = {
        "name": "valid",
        "api_key": f"{api_key}",
        "api_secret": f"{api_secret}",
    }
    api_credentials_path = tf_control_plane["ccm_api_credentials"].replace(
        ".json", "-valid.json"
    )
    with os.fdopen(
        os.open(
            api_credentials_path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, mode=0o600
        ),
        "w",
    ) as api_credentials_file:
        json.dump(api_credentials, api_credentials_file)
    logger.debug(
        f"[CCM] Valid API credentials successfully created; path={api_credentials_path}"
    )

    # Yield
    yield api_credentials_path

    # Teardown
    if not os.getenv("TEST_CCM_NO_CCM_TEARDOWN"):
        os.unlink(api_credentials_path)


# As of 2022-11-18, the SKS orchestrator creates and **reconciliates** CCM-specific
# RBAC rules **even** when CCM is parameterized as disabled in the cluster
# (meaning this fixture should not be needed... but better safe than sorry)
@pytest.fixture(scope="session")
def ccm_rbac(test, tf_control_plane, logger):
    manifest = tf_control_plane["ccm_rbac"]

    # Apply the CCM-specific RBAC rules
    logger.info("[K8s] Applying CCM RBAC manifest ...")
    kubectl(
        [
            "apply",
            f"--filename={manifest}",
        ],
        kubeconfig=tf_control_plane["kubeconfig_admin"],
        pyexit=True,
    )

    # Yield
    yield manifest


@pytest.fixture(scope="session")
def ccm(
    test,
    tf_control_plane,
    ccm_exe,
    ccm_rbac,
    ccm_api_credentials_invalid,
    ccm_api_credentials_valid,
    logger,
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

    # Launch the CCM as a local daemon
    logger.info("[CCM] Launching the CCM daemon ...")
    kubeconfig = tf_control_plane["kubeconfig_ccm"]
    cloud_config = tf_control_plane["ccm_cloud_config"]
    (oPopen, fdMaster, fdSlave) = execBackground(
        [
            ccm_exe,
            f"--kubeconfig={kubeconfig}",
            f"--authentication-kubeconfig={kubeconfig}",
            f"--authorization-kubeconfig={kubeconfig}",
            f"--cloud-config={cloud_config}",
            "--leader-elect=true",
            "--allow-untagged-cloud",
            "--v=3",
        ],
        env=execEnvironment(
            add={"EXOSCALE_SKS_AGENT_RUNNERS": "node-csr-validation"},
            unset=[
                "EXOSCALE_API_KEY",
                "EXOSCALE_API_SECRET",
            ],  # Make sure we use the API credentials file
        ),
    )
    logger.info(f"[CCM] CCM daemon running; pid={oPopen.pid}")

    # Yield (the **non-blocking** stdout+stderr file descriptor)
    os.set_blocking(fdMaster, False)
    with os.fdopen(fdMaster) as output:
        yield output

    # Teardown
    logger.info(f"[CCM] Terminating CCM dameon; pid={oPopen.pid} ...")
    oPopen.terminate()
    if not os.getenv("TEST_CCM_NO_CCM_TEARDOWN"):
        os.unlink(api_credentials_path)


# This fixture ought to be used at the start of each package (tests_* subdirectory)
# such to make sure the CCM daemon started successfully (even if only a subset of
# tests packages/modules)
@pytest.fixture(scope="session")
def ccm_started(test, ccm, logger):
    if test["state"]["ccm"]["started"]:
        logger.debug("[CCM] CCM daemon already started; skipping")

    # We have no guarantee on the order in which expected reflectors start
    # (and appear in the logs)
    reflectors_expected = set(["Node", "Service"])
    reflectors_regexp = "|".join(reflectors_expected)
    reflectors_started = list()
    try:
        for reflector in reflectors_expected:
            (lines, match, unmatch) = ioMatch(
                ccm,
                matches=[
                    f"re:/Listing and watching \\*v1\\.({reflectors_regexp}) from k8s\\.io/i"
                ],
                unmatches=["error"],
                timeout=test["timeout"]["ccm"]["start"],
                logger=logger,
            )
            assert lines > 0
            assert unmatch is None
            assert match is not None
            reflector = match[1]
            reflectors_started.append(reflector)
        assert set(reflectors_started) == reflectors_expected
    except AssertionError:
        pytest.exit("CCM daemon did not start properly")

    logger.info("[CCM] CCM daemon started successfully")
    test["state"]["ccm"]["started"] = True


# Load-Balancer (NLB)
@pytest.fixture(scope="session")
def nlb_hello_external(test, tf_control_plane, tf_nodes, ccm, logger):
    manifest = tf_nodes["manifest_hello_external"]

    # Create NGINX-hello application (via "external", nodes-specific NLB)
    logger.info("[K8s] Applying NGINX-hello (external NLB) application manifest ...")
    kubectl(
        [
            "apply",
            f"--filename={manifest}",
        ],
        kubeconfig=tf_control_plane["kubeconfig_admin"],
        pyexit=True,
    )

    # Yield
    yield manifest

    # Teardown
    if not os.getenv("TEST_CCM_NO_NLB_TEARDOWN"):
        logger.info(
            "[K8s] Deleting NGINX-hello (external NLB) application manifest ..."
        )
        kubectl(
            [
                "delete",
                f"--filename={manifest}",
            ],
            kubeconfig=tf_control_plane["kubeconfig_admin"],
        )


@pytest.fixture(scope="session")
def nlb_ingress_nginx(test, tf_control_plane, ccm, logger):
    manifest = os.path.join(
        test["directory"], "resources", "manifests", "ingress-nginx.yaml"
    )

    # Create NGINX Ingress
    logger.info("[K8s] Applying NGINX Ingress manifest ...")
    kubectl(
        [
            "apply",
            f"--filename={manifest}",
        ],
        kubeconfig=tf_control_plane["kubeconfig_admin"],
        pyexit=True,
    )

    # Yield
    yield manifest

    # Teardown
    if not os.getenv("TEST_CCM_NO_NLB_TEARDOWN"):
        logger.info("[K8s] Deleting NGINX Ingress manifest ...")
        kubectl(
            [
                "delete",
                f"--filename={manifest}",
            ],
            kubeconfig=tf_control_plane["kubeconfig_admin"],
        )


@pytest.fixture(scope="session")
def nlb_hello_ingress(test, tf_control_plane, tf_nodes, nlb_ingress_nginx, logger):
    manifest = tf_nodes["manifest_hello_ingress"]

    # Create NGINX-hello application (via Kubernetes NGINX Ingress)
    logger.info("[K8s] Applying NGINX-hello (Ingress-ed) application manifest ...")
    kubectl(
        [
            "apply",
            f"--filename={manifest}",
        ],
        kubeconfig=tf_control_plane["kubeconfig_admin"],
        pyexit=True,
    )

    # Yield
    yield manifest

    # Teardown
    if not os.getenv("TEST_CCM_NO_NLB_TEARDOWN"):
        logger.info("[K8s] Deleting NGINX-hello (Ingress-ed) application manifest ...")
        kubectl(
            [
                "delete",
                f"--filename={manifest}",
            ],
            kubeconfig=tf_control_plane["kubeconfig_admin"],
        )
