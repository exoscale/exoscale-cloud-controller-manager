import re
from time import time, sleep

import pytest

from helpers import (
    TEST_CCM_TYPE,
    ioMatch,
    k8sGetNodes,
    reUUID,
    reIPv4,
    reIPv4_private,
    reIPv4_privnet,
    reIPv6,
)


@pytest.mark.ccm
def test_k8s_nodes_init(test, tf_nodes, ccm, logger):
    nodes_count_delta = test["state"]["nodes"]["all"]["count_delta"]
    nodes_initialized = list()
    reNode = re.compile(
        "Successfully initialized node (\\S+) with cloud provider", re.IGNORECASE
    )
    for _ in range(nodes_count_delta):
        (lines, match, unmatch) = ioMatch(
            ccm,
            matches=["re:/Successfully initialized node \\S+ with cloud provider/i"],
            timeout=test["timeout"]["ccm"]["csr_approve"],
            logger=logger,
        )
        assert lines > 0
        assert match is not None
        assert unmatch is None
        node = reNode.search(match)
        assert node is not None
        node = node[1]

        nodes_initialized.append(node)

    logger.info("[K8s] Initialized nodes: " + ", ".join(nodes_initialized))


@pytest.mark.ccm
def test_k8s_nodes_labels(test, tf_control_plane, tf_nodes, logger):
    nodes_expected = set(test["state"]["k8s"]["nodes"].keys())
    nodes_qualified = list()
    until = time() + test["timeout"]["ccm"]["node_qualify"]
    while time() <= until:
        nodes = k8sGetNodes(
            kubeconfig=tf_control_plane["kubeconfig_admin"],
        )
        for node, meta in nodes.items():
            if node in nodes_qualified:
                continue
            labels = meta["metadata"]["labels"]
            logger.debug(f"[K8s] Asserting Node: {node} <-> {labels}")
            try:
                if test["type"] not in ["sks"]:
                    # TODO: Investigate why those labels do not show up on SKS
                    if not node.endswith("external"):
                        assert "node.kubernetes.io/instance-type" in labels
                        assert labels["node.kubernetes.io/instance-type"] == "small"
                        assert "topology.kubernetes.io/region" in labels
                        assert (
                            labels["topology.kubernetes.io/region"]
                            == tf_nodes["exoscale_zone"]
                        )
                    else:
                        assert "node.kubernetes.io/instance-type" in labels
                        assert (
                            labels["node.kubernetes.io/instance-type"] == "externalType"
                        )
                        assert "topology.kubernetes.io/region" in labels
                        assert (
                            labels["topology.kubernetes.io/region"] == "externalRegion"
                        )
                if tf_nodes["nodepool_id"] != "n/a":
                    assert "node.exoscale.net/nodepool-id" in labels
                    assert (
                        labels["node.exoscale.net/nodepool-id"]
                        == tf_nodes["nodepool_id"]
                    )

                nodes_qualified.append(node)

            except AssertionError as e:
                logger.debug(f"[K8s] Node assertion failed: {node} <-> {str(e)}")

        if set(nodes_qualified) == nodes_expected:
            break

        sleep(1.0)

    assert set(nodes_qualified) == nodes_expected


@pytest.mark.ccm
@pytest.mark.xfail(
    TEST_CCM_TYPE == "sks",
    reason="TODO/BUG[58670]: CCM: error checking if node is shutdown: provider ID cannot be empty",
)
def test_k8s_nodes_spec(test, tf_control_plane, tf_nodes, logger):
    nodes_expected = set(test["state"]["k8s"]["nodes"].keys())
    nodes_qualified = list()
    until = time() + test["timeout"]["ccm"]["node_qualify"]
    while time() <= until:
        nodes = k8sGetNodes(
            kubeconfig=tf_control_plane["kubeconfig_admin"],
        )
        for node, meta in nodes.items():
            if node in nodes_qualified:
                continue
            spec = meta["spec"]
            logger.debug(f"[K8s] Asserting Node: {node} <-> {spec}")
            try:
                if not node.endswith("external"):
                    assert "providerID" in spec
                    assert spec["providerID"].startswith("exoscale://")
                    assert reUUID.match(spec["providerID"].split("/")[-1])

                nodes_qualified.append(node)

            except AssertionError as e:
                logger.debug(f"[K8s] Node assertion failed: {node} <-> {str(e)}")

        if set(nodes_qualified) == nodes_expected:
            break

        sleep(1.0)

    assert set(nodes_qualified) == nodes_expected


@pytest.mark.ccm
@pytest.mark.xfail(
    TEST_CCM_TYPE == "sks", reason="TODO: Investigate why this fails on SKS"
)
def test_k8s_nodes_addresses(test, tf_control_plane, tf_nodes, logger):
    nodes_expected = set(test["state"]["k8s"]["nodes"].keys())
    nodes_qualified = list()
    until = time() + test["timeout"]["ccm"]["node_qualify"]
    while time() <= until:
        nodes = k8sGetNodes(
            kubeconfig=tf_control_plane["kubeconfig_admin"],
        )
        for node, meta in nodes.items():
            if node in nodes_qualified:
                continue
            addresses = meta["addresses"]
            logger.debug(f"[K8s] Asserting Node: {node} <-> {addresses}")
            try:
                ipv4_internal = None
                ipv6_internal = None
                ipv4_external = None
                ipv6_external = None
                for ip in meta["addresses"]:
                    address = ip["address"]
                    type = ip["type"].lower()
                    if type == "internalip":
                        if reIPv4.match(address):
                            ipv4_internal = address
                        elif reIPv6.match(address):
                            ipv6_internal = address
                    elif type == "externalip":
                        if reIPv4.match(address):
                            ipv4_external = address
                        elif reIPv6.match(address):
                            ipv6_external = address
                if not node.endswith("external"):
                    if test["type"] in ["sks"]:
                        # TODO: Public IPv6
                        assert ipv6_external is None
                        # Public IPv4
                        assert ipv4_external is not None
                        assert not reIPv4_private.match(ipv4_external)
                        # TODO: PrivNet IPv4
                        assert ipv4_internal is None
                        # assert reIPv4_privnet.match(ipv4_internal)
                        # TODO: PrivNet IPv6
                        assert ipv6_internal is None
                    else:
                        # Public IPv6
                        assert ipv6_external is not None
                        # Public IPv4
                        assert ipv4_external is not None
                        assert not reIPv4_private.match(ipv4_external)
                        # PrivNet IPv4
                        assert ipv4_internal is not None
                        assert reIPv4_privnet.match(ipv4_internal)
                        # TODO: PrivNet IPv6
                        assert ipv6_internal is None
                else:
                    # External nodes addresses are untouched
                    # (unless explicitly configured in the the CCM cloud-config)
                    assert ipv6_external is None
                    assert ipv4_external is None
                    assert ipv4_internal is not None
                    assert ipv6_internal is None

                nodes_qualified.append(node)

            except AssertionError as e:
                logger.debug(f"[K8s] Node assertion failed: {node} <-> {str(e)}")

        if set(nodes_qualified) == nodes_expected:
            break

        sleep(1.0)

    assert set(nodes_qualified) == nodes_expected
