import json
from time import sleep, time

import pytest

from helpers import kubectl, exocli, ioMatch, httpRequest


@pytest.mark.nlb
def test_k8s_hello_external(test, tf_control_plane, nlb_hello_external, logger):
    # Deployment
    (iExit, sStdOut, sStdErr) = kubectl(
        [
            "--output=json",
            "--namespace=default",
            "wait",
            "--timeout=600s",
            "--for=condition=Available",
            "deployment/hello-external",
        ],
        kubeconfig=tf_control_plane["kubeconfig_admin"],
    )
    assert iExit == 0
    manifest = json.loads(sStdOut)
    logger.debug(
        f"[K8s] Asserting NGINX-hello (external NLB) application (Deployment) manifest:\n{manifest}"
    )

    assert manifest["kind"] == "Deployment"
    assert manifest["metadata"]["labels"]["app.kubernetes.io/name"] == "hello-external"


@pytest.mark.nlb
def test_ccm_hello_external_services(test, ccm, nlb_hello_external, logger):
    nlb_name = test["state"]["nlb"]["external"]["name"]
    if nlb_name is None:
        pytest.skip("Nodes NLB preliminary test has not run (or has failed)")
    for port in [80]:
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
def test_cli_hello_external(
    test, tf_control_plane, tf_nodes, nlb_hello_external, logger
):
    exoscale_zone = tf_control_plane["exoscale_zone"]
    nlb_name = test["state"]["nlb"]["external"]["name"]
    nlb_id = test["state"]["nlb"]["external"]["id"]
    if nlb_name is None or nlb_id is None:
        pytest.skip("Nodes NLB preliminary test has not run (or has failed)")
    (iExit, sStdOut, sStdErr) = exocli(
        [
            "--output-format=json",
            f"--zone={exoscale_zone}",
            "compute",
            "load-balancer",
            "show",
            nlb_id,
        ],
    )
    assert iExit == 0
    output = json.loads(sStdOut)
    logger.debug(f"[CLI] Asserting NLB: {nlb_name} (ID:{nlb_id}) <-> {output}")

    assert output["ip_address"] == tf_nodes["external_nlb_ipv4"]
    assert output["state"] == "running"

    # State (update)
    test["state"]["nlb"]["external"]["ipv4"] = output["ip_address"]
    # test["state"]["nlb"]["external"]["ipv6"] = output["TODO"]


@pytest.mark.nlb
def test_cli_hello_external_services(
    test, tf_control_plane, tf_nodes, nlb_hello_external, logger
):
    exoscale_zone = tf_control_plane["exoscale_zone"]
    nlb_id = test["state"]["nlb"]["external"]["id"]
    if nlb_id is None:
        pytest.skip("Nodes NLB preliminary test has not run (or has failed)")
    ports_expected = set([80])
    ports_healthy = list()
    until = time() + test["timeout"]["nlb"]["service"]["healthcheck"]["success"]
    while time() <= until:
        for port in ports_expected:
            if port not in test["state"]["nlb"]["external"]["services"]:
                pytest.skip("Nodes NLB preliminary test has not run (or has failed)")
            service_name = test["state"]["nlb"]["external"]["services"][port]["name"]
            service_id = test["state"]["nlb"]["external"]["services"][port]["id"]
            (iExit, sStdOut, sStdErr) = exocli(
                [
                    "--output-format=json",
                    f"--zone={exoscale_zone}",
                    "compute",
                    "load-balancer",
                    "service",
                    "show",
                    nlb_id,
                    service_id,
                ],
            )
            logger.debug(
                f"XXXXXXX NLB service: {sStdErr} Service ID: {service_id}"
            )
            assert iExit == 0
            output = json.loads(sStdOut)
            logger.debug(
                f"Asserting NLB service: {service_name} (ID:{service_id}) <-> {output}"
            )

            healthchecks_ok = 0
            for healthcheck in output["healthcheck_status"]:
                if healthcheck["status"] == "success":
                    healthchecks_ok += 1
            if healthchecks_ok != test["state"]["nodes"]["pool"]["size"]:
                continue

            assert output["id"] == service_id
            assert output["name"] == service_name
            assert output["port"] == port
            assert output["protocol"] == "tcp"
            assert output["strategy"] == "round-robin"
            assert output["instance_pool_id"] == tf_nodes["instancepool_id"]
            assert output["state"] == "running"

            assert output["healthcheck"]["mode"] == "http"
            assert output["healthcheck"]["uri"] == "/"
            assert output["healthcheck"]["interval"] == 5000000000  # micro-second
            assert output["healthcheck"]["timeout"] == 2000000000  # micro-second
            assert output["healthcheck"]["retries"] == 2
            assert (
                len(output["healthcheck_status"])
                == test["state"]["nodes"]["pool"]["size"]
            )
            for healthcheck in output["healthcheck_status"]:
                assert healthcheck["status"] == "success"

            ports_healthy.append(port)

            # State (update)
            test["state"]["nlb"]["external"]["services"][port] = output

        if set(ports_healthy) == ports_expected:
            break

        sleep(1.0)

    assert set(ports_healthy) == ports_expected


@pytest.mark.nlb
def test_http_hello_external(test, nlb_hello_external, logger):
    nlb_ipv4 = test["state"]["nlb"]["external"]["ipv4"]
    if nlb_ipv4 is None:
        pytest.skip("Nodes NLB preliminary test has not run (or has failed)")
    for scheme, port in [("http", 80)]:
        logger.debug(f"[NLB] Querying URL: {scheme}://{nlb_ipv4}:{port}")
        try:
            httpRequest(f"{scheme}://{nlb_ipv4}:{port}")
        except Exception as e:
            pytest.fail(str(e), pytrace=False)
