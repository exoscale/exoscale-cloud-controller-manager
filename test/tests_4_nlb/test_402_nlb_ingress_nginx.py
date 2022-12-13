import json
from time import sleep, time

import pytest

from helpers import kubectl, exocli, ioMatch, reIPv4


@pytest.mark.nlb
def test_k8s_ingress_nginx(test, tf_control_plane, nlb_ingress_nginx, logger):
    (iExit, sStdOut, sStdErr) = kubectl(
        [
            "--output=json",
            "--namespace=ingress-nginx",
            "wait",
            "--timeout=600s",
            "--for=condition=Available",
            "deployment/ingress-nginx-controller",
        ],
        kubeconfig=tf_control_plane["kubeconfig_admin"],
    )
    assert iExit == 0
    manifest = json.loads(sStdOut)
    logger.debug(f"[K8s] Asserting NGINX Ingress (Deployment) manifest:\n{manifest}")

    assert manifest["kind"] == "Deployment"
    assert manifest["metadata"]["labels"]["app.kubernetes.io/name"] == "ingress-nginx"


@pytest.mark.nlb
def test_ccm_ingress_nginx(test, ccm, nlb_ingress_nginx, logger):
    (lines, match, unmatch) = ioMatch(
        ccm,
        matches=['re:/NLB "(k8s-[-0-9a-f]+)" created successfully \\(ID: ([^)]+)\\)/i'],
        timeout=test["timeout"]["nlb"]["start"],
        logger=logger,
    )
    assert lines > 0
    assert unmatch is None
    assert match is not None
    nlb_name = match[1]
    nlb_id = match[2]

    # State (update)
    test["state"]["nlb"]["k8s"]["id"] = nlb_id
    test["state"]["nlb"]["k8s"]["name"] = nlb_name
    logger.info(f"[CCM] Created NLB: {nlb_name} (ID:{nlb_id})")


@pytest.mark.nlb
def test_cli_ingress_nginx(test, tf_control_plane, nlb_ingress_nginx, logger):
    exoscale_zone = tf_control_plane["exoscale_zone"]
    nlb_name = test["state"]["nlb"]["k8s"]["name"]
    nlb_id = test["state"]["nlb"]["k8s"]["id"]
    if nlb_name is None or nlb_id is None:
        pytest.skip("NGINX Ingress NLB preliminary test has not run (or has failed)")
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

    assert reIPv4.match(output["ip_address"])
    assert output["state"] == "running"

    # State (update)
    test["state"]["nlb"]["k8s"]["ipv4"] = output["ip_address"]
    # test["state"]["nlb"]["k8s"]["ipv6"] = output["TODO"]


@pytest.mark.nlb
def test_ccm_ingress_nginx_services(test, ccm, nlb_ingress_nginx, logger):
    nlb_name = test["state"]["nlb"]["k8s"]["name"]
    if nlb_name is None:
        pytest.skip("NGINX Ingress NLB preliminary test has not run (or has failed)")
    ports_expected = set([80, 443])
    ports_regexp = "|".join(map(str, ports_expected))
    ports_configured = list()
    for port in ports_expected:
        (lines, match, unmatch) = ioMatch(
            ccm,
            matches=[
                f"re:/NLB service {nlb_name}/(\\S+-({ports_regexp})) created successfully \\(ID: ([^)]+)\\)/i"
            ],
            timeout=test["timeout"]["nlb"]["service"]["start"],
            logger=logger,
        )
        assert lines > 0
        assert unmatch is None
        assert match is not None
        service_name = match[1]
        service_id = match[3]

        ports_configured.append(port)

        # State (update)
        test["state"]["nlb"]["k8s"]["services"][port] = {
            "name": service_name,
            "id": service_id,
        }
        logger.info(f"[CCM] Created NLB service: {service_name} (ID:{service_id})")

    assert set(ports_configured) == ports_expected


@pytest.mark.nlb
def test_ccm_ingress_nginx_loadbalancer(
    test, tf_control_plane, ccm, nlb_ingress_nginx, logger
):
    (lines, match, unmatch) = ioMatch(
        ccm,
        matches=[
            "Ensuring load balancer for service ingress-nginx/ingress-nginx-controller"
        ],
        logger=logger,
    )
    assert lines > 0
    assert unmatch is None
    assert match is not None


@pytest.mark.nlb
def test_k8s_ingress_nginx_loadbalancer(
    test, tf_control_plane, nlb_ingress_nginx, logger
):
    nlb_ipv4 = test["state"]["nlb"]["k8s"]["ipv4"]
    if nlb_ipv4 is None:
        pytest.skip("NGINX Ingress NLB preliminary test has not run (or has failed)")
    (iExit, sStdOut, sStdErr) = kubectl(
        [
            "--output=json",
            "--namespace=ingress-nginx",
            "get",
            "service/ingress-nginx-controller",
        ],
        kubeconfig=tf_control_plane["kubeconfig_admin"],
    )
    assert iExit == 0
    manifest = json.loads(sStdOut)
    logger.debug(f"[K8s] Asserting NGINX Ingress (Service) manifest:\n{manifest}")

    assert manifest["kind"] == "Service"
    assert manifest["metadata"]["labels"]["app.kubernetes.io/name"] == "ingress-nginx"
    assert manifest["status"]["loadBalancer"]["ingress"][0]["ip"] == nlb_ipv4


@pytest.mark.nlb
def test_cli_ingress_nginx_services(
    test, tf_control_plane, tf_nodes, nlb_ingress_nginx, logger
):
    exoscale_zone = tf_control_plane["exoscale_zone"]
    nlb_id = test["state"]["nlb"]["k8s"]["id"]
    if nlb_id is None:
        pytest.skip("NGINX Ingress NLB preliminary test has not run (or has failed)")
    ports_expected = set([80, 443])
    ports_healthy = list()
    until = time() + test["timeout"]["nlb"]["service"]["healthcheck"]["success"]
    while time() <= until:
        for port in ports_expected:
            if port not in test["state"]["nlb"]["k8s"]["services"]:
                pytest.skip(
                    "NGINX Ingress NLB preliminary test has not run (or has failed)"
                )
            service_name = test["state"]["nlb"]["k8s"]["services"][port]["name"]
            service_id = test["state"]["nlb"]["k8s"]["services"][port]["id"]
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
            assert iExit == 0
            output = json.loads(sStdOut)
            logger.debug(
                f"[CLI] Asserting NLB service: {service_name} (ID:{service_id}) <-> {output}"
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

            assert output["healthcheck"]["mode"] == "tcp"
            assert output["healthcheck"]["interval"] == 10000000000  # micro-second
            assert output["healthcheck"]["timeout"] == 5000000000  # micro-second
            assert output["healthcheck"]["retries"] == 1
            assert (
                len(output["healthcheck_status"])
                == test["state"]["nodes"]["pool"]["size"]
            )
            for healthcheck in output["healthcheck_status"]:
                assert healthcheck["status"] == "success"

            ports_healthy.append(port)

            # State (update)
            test["state"]["nlb"]["k8s"]["services"][port] = output

        if set(ports_healthy) == ports_expected:
            break

        sleep(1.0)

    assert set(ports_healthy) == ports_expected
