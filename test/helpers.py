import json
import os
import re
from pty import openpty
from subprocess import Popen, PIPE
from sys import stdout, stderr
from time import sleep, time

import pytest
import requests
import tftest
import urllib3


## Helpers

# Environment
TEST_CCM_TYPE = os.getenv("TEST_CCM_TYPE", "sks")
TEST_CCM_ZONE = os.getenv("TEST_CCM_ZONE", "ch-gva-2")
TEST_CCM_EXEC_TERRAFORM = os.getenv("TERRAFORM", "terraform")
TEST_CCM_EXEC_KUBECTL = os.getenv("KUBECTL", "kubectl")
TEST_CCM_EXEC_EXOCLI = os.getenv("EXOCLI", "exo")

# RegExps
reUUID = re.compile("^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$")
reIPv4 = re.compile("^[0-9]{1,3}(\\.[0-9]{1,3}){3}$")
reIPv4_private = re.compile(
    "^(10(\\.[0-9]{1,3}){3}|172\\.(1[6-9]|2[0-9]|3[0-1])(\\.[0-9]{1,3}){2}|192\\.168(\\.[0-9]{1,3}){2})$"
)
reIPv4_privnet = re.compile("^172\\.16\\.0\\.[0-9]{1,3}$")
reIPv6 = re.compile("^[0-9a-f]{1,4}(:([0-9a-f]{1,4}|:)){7}$")
reMatch_compiled = dict()


def reMatch(needle: str, haystack: str):
    if needle[:4] != "re:/":
        return (False, None)
    reHash = hash(needle)
    if reHash not in reMatch_compiled:
        flags = 0
        needle = needle.split("/")
        for flag in needle[-1]:
            if flag == "a":
                flags |= re.ASCII
            if flag == "i":
                flags |= re.IGNORECASE
        reMatch_compiled[reHash] = re.compile("/".join(needle[1:-1]), flags)
    return (True, reMatch_compiled[reHash].search(haystack))


# Shell
def execEnvironment(inherit: bool = True, add: dict = None, unset: list = None):
    if inherit:
        env = os.environ.copy()
    else:
        env = dict()
    if add is not None:
        for k, v in add.items():
            env[k] = v
    if unset is not None:
        for k in unset:
            env.pop(k, None)
    return env


def execForeground(
    commands: list, cwd: str = None, env: dict = None, pyexit: bool = False
):
    oPopen = Popen(commands, cwd=cwd, env=env, stdout=PIPE, stderr=PIPE)
    (bStdOut, bStdErr) = oPopen.communicate()
    iExit = oPopen.returncode
    if iExit and pyexit:
        commands = " ".join(commands)
        pytest.exit(f"Command failed: {commands}; exit={iExit}")
    return (
        iExit,
        bStdOut.decode(stdout.encoding),
        bStdErr.decode(stderr.encoding),
    )


def execBackground(commands: list, cwd: str = None, env: dict = None):
    (fdMaster, fdSlave) = openpty()
    oPopen = Popen(
        commands, cwd=cwd, env=env, stdin=PIPE, stdout=fdSlave, stderr=fdSlave
    )
    return (oPopen, fdMaster, fdSlave)


# I/O stream
def ioMatch(
    io_file,
    matches: list = None,
    unmatches: list = None,
    timeout: int = 10,
    logger=None,
):
    matches = matches or list()
    unmatches = unmatches or list()
    until = time() + timeout
    lines = 0
    while time() <= until:
        line = io_file.readline()
        if line:
            if logger:
                logger.debug(f"[I/O]: {line}")
            line_LC = line.lower()
            lines += 1
            for match in matches:
                (isRegExp, reMatchObject) = reMatch(match, line)
                if isRegExp:
                    if reMatchObject:
                        return (lines, reMatchObject, None)
                elif match.lower() in line_LC:
                    return (lines, line, None)
            for match in unmatches:
                (isRegExp, reMatchObject) = reMatch(match, line)
                if isRegExp:
                    if reMatchObject:
                        return (lines, None, reMatchObject)
                elif match.lower() in line_LC:
                    return (lines, None, line)
        sleep(0.01)
    return (lines, None, None)


# Terraform
def tfControlPlane(test, logger):
    # Initialize and apply the Terraform configuration
    logger.info(
        "[Terraform] Creating the control-plane infrastructure (this may take some time) ..."
    )
    tf = tftest.TerraformTest(
        tfdir=os.path.join(test["type"], "control-plane"),
        basedir=test["terraform"]["directory"],
        binary=TEST_CCM_EXEC_TERRAFORM,
    )
    # (no clean-up; give the user the possibility to teardown manually if needs be)
    tf.setup(cleanup_on_exit=False)
    tf_vars = {
        "exoscale_zone": TEST_CCM_ZONE,
    }
    tf.apply(tf_vars=tf_vars)
    outputs = tf.output()

    # State
    nodes = k8sGetNodes(
        kubeconfig=outputs["kubeconfig_admin"],
        pyexit=True,
    )
    nodes_pool_size = 0
    nodes_external_quantity = 0
    for node in nodes:
        if node.endswith("external"):
            nodes_external_quantity += 1
        else:
            nodes_pool_size += 1
    test["state"]["nodes"]["pool"]["size"] = nodes_pool_size
    test["state"]["nodes"]["pool"]["size_previous"] = test["state"]["nodes"]["pool"][
        "size"
    ]
    test["state"]["nodes"]["external"]["quantity"] = nodes_external_quantity
    test["state"]["nodes"]["external"]["quantity_previous"] = test["state"]["nodes"][
        "external"
    ]["quantity"]
    test["state"]["nodes"]["all"]["count"] = nodes_pool_size + nodes_external_quantity
    test["state"]["nodes"]["all"]["count_previous"] = test["state"]["nodes"]["all"][
        "count"
    ]
    test["state"]["nodes"]["all"]["count_delta"] = 0

    return tf


def tfNodes(test, tf_control_plane, pool_size, logger):
    # Initialize and apply the Terraform configuration
    logger.info(
        f"[Terraform] Creating the {pool_size}-node(s) pool infrastructure (this may take some time) ..."
    )
    tf = tftest.TerraformTest(
        tfdir=os.path.join(test["type"], "nodes"),
        basedir=test["terraform"]["directory"],
        binary=TEST_CCM_EXEC_TERRAFORM,
    )
    # (no clean-up; give the user the possibility to teardown manually if needs be)
    tf.setup(cleanup_on_exit=False)
    tf_vars = {
        "exoscale_zone": TEST_CCM_ZONE,
        "test_id": tf_control_plane["test_id"],
        "test_cluster_id": tf_control_plane["cluster_id"],
        "test_cluster_sg_id": tf_control_plane["cluster_sg_id"],
        "test_control_plane_endpoint": tf_control_plane["control_plane_endpoint"],
        "test_nodes_pool_size": pool_size,
        "test_nodes_bootstrap_token": tf_control_plane["nodes_bootstrap_token"],
        "test_nodes_ssh_key_name": tf_control_plane["nodes_ssh_key_name"],
    }
    tf.apply(tf_vars=tf_vars)
    outputs = tf.output()

    # State
    test["state"]["nodes"]["pool"]["size_previous"] = test["state"]["nodes"]["pool"][
        "size"
    ]
    test["state"]["nodes"]["pool"]["size"] = pool_size
    test["state"]["nodes"]["external"]["quantity_previous"] = test["state"]["nodes"][
        "external"
    ]["quantity"]
    test["state"]["nodes"]["external"]["quantity"] = (
        1 if test["type"] not in ["sks"] else 0
    )
    test["state"]["nodes"]["all"]["count_previous"] = (
        test["state"]["nodes"]["pool"]["size_previous"]
        + test["state"]["nodes"]["external"]["quantity_previous"]
    )
    test["state"]["nodes"]["all"]["count"] = (
        test["state"]["nodes"]["pool"]["size"]
        + test["state"]["nodes"]["external"]["quantity"]
    )
    test["state"]["nodes"]["all"]["count_delta"] = (
        test["state"]["nodes"]["all"]["count"]
        - test["state"]["nodes"]["all"]["count_previous"]
    )

    # (wait for kubelet to do its job)
    nodes_count = test["state"]["nodes"]["all"]["count"]
    nodes_count_delta = test["state"]["nodes"]["all"]["count_delta"]
    nodes = k8sWaitForNodes(
        quantity=nodes_count,
        timeout=test["timeout"]["node"]["start"],
        kubeconfig=tf_control_plane["kubeconfig_admin"],
        pyexit=True,
    )
    if nodes_count_delta > 0 and len(nodes) != nodes_count:
        logger.warning(
            f"[Terraform] Registered/CSR-ed Kubernetes Nodes does not match expected quantity: {nodes_count} <-> {nodes}"
        )

    # State
    test["state"]["k8s"]["nodes"] = k8sGetNodes(
        kubeconfig=tf_control_plane["kubeconfig_admin"],
        pyexit=True,
    )
    test["state"]["k8s"]["csrs"] = k8sGetNodeCSRs(
        valid=True,
        kubeconfig=tf_control_plane["kubeconfig_admin"],
        pyexit=True,
    )
    test["state"]["nlb"]["external"]["name"] = outputs["test_name"]
    test["state"]["nlb"]["external"]["id"] = outputs["external_nlb_id"]
    test["state"]["nlb"]["external"]["ipv4"] = outputs["external_nlb_ipv4"]
    # test["state"]["nlb"]["external"]["ipv6"] = outputs["external_nlb_ipv6"]

    return tf


# Kubernetes (kubectl)
def kubectl(commands: list, kubeconfig: str = None, pyexit: bool = False):
    kubeconfig = os.getenv("KUBECONFIG", kubeconfig)
    if kubeconfig is not None:
        env = execEnvironment(add={"KUBECONFIG": kubeconfig})
    else:
        env = None
    kubectl = [TEST_CCM_EXEC_KUBECTL]
    kubectl.extend(commands)
    return execForeground(
        kubectl,
        env=env,
        pyexit=pyexit,
    )


def k8sGetNodes(kubeconfig: str = None, pyexit: bool = False):
    (iExit, sStdOut, sStdErr) = kubectl(
        [
            "--output=json",
            "get",
            "--selector=!node-role.kubernetes.io/control-plane",
            "nodes",
        ],
        kubeconfig=kubeconfig,
        pyexit=pyexit,
    )
    if iExit:
        return None

    nodes = dict()
    output = json.loads(sStdOut)
    for item in output["items"]:
        name = item["metadata"]["name"]
        ready = False
        for condition in item["status"].get("conditions", []):
            if condition["type"] == "Ready":
                if condition["status"] == "True":
                    ready = True
                break
        selfie = "✔" if ready else "✘"
        nodes[name] = {
            "selfie": selfie,
            "ready": ready,
            "metadata": item["metadata"],
            "spec": item["spec"],
            "addresses": item["status"]["addresses"],
            "info": item["status"]["nodeInfo"],
        }
    return nodes


def k8sGetNodeCSRs(valid: bool = False, kubeconfig: str = None, pyexit: bool = False):
    (iExit, sStdOut, sStdErr) = kubectl(
        [
            "--output=json",
            "get",
            "certificatesigningrequests",
        ],
        kubeconfig=kubeconfig,
        pyexit=pyexit,
    )
    if iExit:
        return None

    csrs = dict()
    output = json.loads(sStdOut)
    for item in output["items"]:
        if item["spec"]["username"].startswith("system:node:test-ccm"):
            approved = False
            for condition in item["status"].get("conditions", []):
                if (
                    condition["reason"] == "ExoscaleCloudControllerApproved"
                    and condition["type"] == "Approved"
                ):
                    if (
                        condition["status"] == "True"
                        and "certificate" in item["status"]
                    ):
                        approved = True
                    break
            selfie = "✔" if approved else "✘"
            csrs[item["metadata"]["name"]] = {
                "selfie": selfie,
                "approved": approved,
                "node": item["spec"]["username"][12:],
            }

    if valid:
        nodes = k8sGetNodes(kubeconfig=kubeconfig, pyexit=pyexit)
        csrs_invalid = list()
        for csr, csr_meta in csrs.items():
            if csr_meta["node"] not in nodes:
                csrs_invalid.append(csr)
        for csr in csrs_invalid:
            csrs.pop(csr)
    return csrs


def k8sWaitForNodes(
    quantity: int, timeout: int, kubeconfig: str = None, pyexit: bool = False
):
    nodes_ok = list()
    until = time() + timeout
    while time() <= until:
        nodes = k8sGetNodes(kubeconfig=kubeconfig, pyexit=pyexit)
        if len(nodes) <= quantity:
            csrs = k8sGetNodeCSRs(kubeconfig=kubeconfig, pyexit=pyexit)
            for node, node_meta in nodes.items():
                if node in nodes_ok:
                    continue
                if node_meta["ready"]:
                    for _, csr_meta in csrs.items():
                        if csr_meta["node"] == node:
                            nodes_ok.append(node)
                            break
            if len(nodes_ok) >= quantity:
                break
        sleep(1.0)
    return set(nodes_ok)


# Exoscale (CLI)
def exocli(commands: list, pyexit: bool = False):
    exocli = [TEST_CCM_EXEC_EXOCLI]
    exocli.extend(commands)
    return execForeground(
        exocli,
        pyexit=pyexit,
    )


# HTTP requests
def httpRequest(
    url: str,
    expect: list = None,
    method: str = "GET",
    timeout: float = 1.0,
    tries: int = 10,
    delay: float = 3.0,
):
    # REF: https://urllib3.readthedocs.io/en/latest/advanced-usage.html#ssl-warnings
    urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)
    expect = expect or [200]
    while tries > 0:
        tries -= 1
        try:
            response = requests.request(
                method=method,
                url=url,
                timeout=timeout,
                verify=False,
            )
            if response.status_code in expect:
                return response
        except (
            requests.exceptions.ConnectionError,
            requests.exceptions.HTTPError,
        ) as e:
            if not tries:
                raise e
        sleep(delay)
    return None


# MAIN
if __name__ == "__main__":
    nodes = k8sGetNodes()
    print(nodes)
    csrs = k8sGetNodeCSRs()
    print(csrs)
    nodes_ok = k8sWaitForNodes(len(nodes), timeout=1)
    print(nodes_ok)
