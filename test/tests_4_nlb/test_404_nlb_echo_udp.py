import json
from time import sleep, time
import socket

import pytest

from helpers import kubectl, exocli, ioMatch


@pytest.mark.nlb
def test_k8s_udp_echo_external(test, tf_control_plane, nlb_udp_echo_external, logger):
    # Deployment
    (iExit, sStdOut, sStdErr) = kubectl(
        [
            "--output=json",
            "--namespace=default",
            "wait",
            "--timeout=600s",
            "--for=condition=Available",
            "deployment/udp-echo",
        ],
        kubeconfig=tf_control_plane["kubeconfig_admin"],
    )
    assert iExit == 0
    manifest = json.loads(sStdOut)
    logger.debug(
        f"[K8s] Asserting UDP Echo (external NLB) application (Deployment) manifest:\n{manifest}"
    )

    assert manifest["kind"] == "Deployment"
    assert manifest["metadata"]["labels"]["app.kubernetes.io/name"] == "udp-echo"


@pytest.mark.nlb
def test_udp_echo_service_creation(test, ccm, nlb_udp_echo_external, logger):
    nlb_name = test["state"]["nlb"]["external"]["name"]
    if nlb_name is None:
        pytest.skip("Nodes NLB preliminary test has not run (or has failed)")
    for port in [8080]:
        (lines, match, unmatch) = ioMatch(
            ccm,
            matches=[
                f"re:/NLB service {nlb_name}/(\\S+-{port}) created successfully \\(ID: ([^)]+)\\)/i"
            ],
            timeout=test["timeout"]["nlb"]["service"]["start"],
            logger=logger,
        )
        assert lines > 0
        assert unmatch is None
        assert match is not None
        service_name = match[1]
        service_id = match[2]

        # State (update)
        test["state"]["nlb"]["external"]["services"][port] = {
            "name": service_name,
            "id": service_id,
        }
        logger.info(f"[CCM] Created NLB service: {service_name} (ID:{service_id})")


@pytest.mark.nlb
def test_udp_echo_external_response(test, tf_nodes, nlb_udp_echo_external, logger):
    nlb_ipv4 = test["state"]["nlb"]["external"]["ipv4"]
    if nlb_ipv4 is None:
        pytest.skip("Nodes NLB preliminary test has not run (or has failed)")

    udp_port = 8080
    logger.debug(f"[NLB] Testing UDP Echo server on {nlb_ipv4}:{udp_port}")

    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as sock:
        sock.settimeout(10.0)
        try:
            # Send data
            message = "Hello UDP"
            sock.sendto(message.encode(), (nlb_ipv4, udp_port))

            # Receive response
            data, _ = sock.recvfrom(1024)
            logger.debug(f"[UDP Echo] Received: {data.decode()}")
            assert data.decode() == "Echo", "Unexpected response from UDP Echo server"
        except socket.timeout:
            pytest.fail("Timeout: No response from UDP Echo server", pytrace=False)
