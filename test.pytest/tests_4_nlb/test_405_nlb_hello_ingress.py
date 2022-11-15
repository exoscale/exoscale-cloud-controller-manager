import json

import pytest

from helpers import kubectl, httpRequest


@pytest.mark.nlb
def test_k8s_hello_ingress(test, tf_control_plane, nlb_hello_ingress, logger):
    # Deployment
    (iExit, sStdOut, sStdErr) = kubectl(
        [
            "--output=json",
            "--namespace=default",
            "wait",
            "--timeout=600s",
            "--for=condition=Available",
            "deployment/hello-ingress",
        ],
        kubeconfig=tf_control_plane["kubeconfig_admin"],
    )
    assert iExit == 0
    manifest = json.loads(sStdOut)
    logger.debug(
        f"[K8s] Asserting NGINX-hello (Ingress-ed) application (Deployment) manifest:\n{manifest}"
    )

    assert manifest["kind"] == "Deployment"
    assert manifest["metadata"]["labels"]["app.kubernetes.io/name"] == "hello-ingress"


@pytest.mark.nlb
def test_http_hello_ingress(test, nlb_hello_ingress, logger):
    nlb_ipv4 = test["state"]["nlb"]["k8s"]["ipv4"]
    if nlb_ipv4 is None:
        pytest.skip("NGINX Ingress NLB preliminary test has not run (or has failed)")
    for scheme, port in [("http", 80), ("https", 443)]:
        logger.debug(f"[NLB] Querying URL: {scheme}://{nlb_ipv4}:{port}")
        try:
            httpRequest(f"{scheme}://{nlb_ipv4}:{port}")
        except Exception as e:
            pytest.fail(str(e), pytrace=False)
