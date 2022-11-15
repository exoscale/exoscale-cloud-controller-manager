import pytest

from helpers import httpRequest

# Make sure to request (package-scoped, parameterized) 'tf_nodes_pool_resize' fixture,
# such as to trigger each test on each (nodes quantity) update


@pytest.mark.nodes_pool_resize
def test_http_hello_external(test, tf_nodes_pool_resize, nlb_hello_external, logger):
    if test["state"]["nodes"]["pool"]["size"] <= 0:
        pytest.skip("Nodes pool is empty")

    nlb_ipv4 = test["state"]["nlb"]["external"]["ipv4"]
    if nlb_ipv4 is None:
        pytest.skip("Nodes NLB tests ('nlb' marker) have not run")
    for scheme, port in [("http", 80)]:
        logger.debug(f"[NLB] Querying URL: {scheme}://{nlb_ipv4}:{port}")
        try:
            httpRequest(
                f"{scheme}://{nlb_ipv4}:{port}",
                tries=10,
                delay=test["timeout"]["nlb"]["service"]["start"] / 10,
            )
        except Exception as e:
            pytest.fail(str(e), pytrace=False)


@pytest.mark.nodes_pool_resize
def test_http_hello_ingress(test, tf_nodes_pool_resize, logger):
    if test["state"]["nodes"]["pool"]["size"] <= 0:
        pytest.skip("Nodes pool is empty")

    nlb_ipv4 = test["state"]["nlb"]["k8s"]["ipv4"]
    if nlb_ipv4 is None:
        pytest.skip(
            "NGINX Ingress NLB tests ('nlb' marker) have not run (or have failed)"
        )
    for scheme, port in [("http", 80), ("https", 443)]:
        logger.debug(f"[NLB] Querying URL: {scheme}://{nlb_ipv4}:{port}")
        try:
            httpRequest(
                f"{scheme}://{nlb_ipv4}:{port}",
                tries=10,
                delay=test["timeout"]["nlb"]["service"]["start"] / 10,
            )
        except Exception as e:
            pytest.fail(str(e), pytrace=False)
