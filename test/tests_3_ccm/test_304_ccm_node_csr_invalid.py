import json
from base64 import b64encode
from ipaddress import IPv4Address, IPv6Address

import pytest
from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.x509.oid import NameOID

from helpers import TEST_CCM_TYPE, ioMatch, kubectl


## Helpers

# Kubernetes CSR manifest
# REF: https://kubernetes.io/docs/reference/access-authn-authz/certificate-signing-requests/ (signerName: kubernetes.io/kubelet-serving)
K8S_CSR_MANIFEST = """---
kind: CertificateSigningRequest
apiVersion: certificates.k8s.io/v1
metadata:
  name: {}
spec:
  request: {}
  signerName: kubernetes.io/kubelet-serving
  usages:
    - digital signature
    - key encipherment
    - server auth
"""


def k8s_csr_manifest(name: str, node: str, dns_sans: list = None, ip_sans: list = None):
    dns_sans = dns_sans or list()
    ip_sans = ip_sans or list()

    # Private key (NIST P-256)
    key = ec.generate_private_key(ec.SECP256R1)

    # Subject
    subject = x509.Name(
        [
            x509.NameAttribute(NameOID.ORGANIZATION_NAME, "system:nodes"),
            x509.NameAttribute(NameOID.COMMON_NAME, f"system:node:{node}"),
        ]
    )

    # Extensions
    # (SANs)
    sans = list()
    for dns_san in dns_sans:
        sans.append(x509.DNSName(dns_san))
    for ip_san in ip_sans:
        ip = IPv6Address(ip_san) if ":" in ip_san else IPv4Address(ip_san)
        sans.append(x509.IPAddress(ip))

    # Certificate Signing Request (CSR)
    csr = (
        x509.CertificateSigningRequestBuilder()
        .subject_name(subject)
        .add_extension(
            x509.SubjectAlternativeName(sans),
            critical=False,
        )
        .sign(key, hashes.SHA256())
    )

    return K8S_CSR_MANIFEST.format(
        name,
        b64encode(csr.public_bytes(serialization.Encoding.PEM)).decode("utf-8"),
    )


## Fixtures

# Invalid CSRs
@pytest.fixture(
    scope="function",
    params=[
        {"name": "csr-invalid-ipv4", "ip": "192.0.2.42"},
        {"name": "csr-invalid-ipv6", "ip": "2001:db8::dead:beef"},
    ],
)
def k8s_csr_invalid_ip_address(request, test, tf_control_plane, tf_nodes, ccm, logger):
    csr_node = tf_nodes["external_node_name"]
    csr_name = request.param["name"]
    csr_ip = request.param["ip"]
    manifest = k8s_csr_manifest(
        name=csr_name,
        node=csr_node,
        dns_sans=[csr_node],
        ip_sans=[csr_ip],
    )
    logger.debug(f"[K8s] Creating CSR: {csr_name} ...\n{manifest}")
    (iExit, sStdOut, sStdErr) = kubectl(
        [
            "--output=json",
            "--filename=-",
            "apply",
        ],
        input=manifest,
        kubeconfig=tf_nodes["external_node_kubeconfig"],
    )
    assert iExit == 0
    output = json.loads(sStdOut)
    logger.debug(f"[K8s] Created CSR: {csr_name}\n{output}")

    csr = output["metadata"]["name"]
    assert csr == csr_name

    # Yield
    yield csr

    # Teardown
    logger.debug(f"[K8s] Deleting CSR: {csr_name} ...")
    (iExit, sStdOut, sStdErr) = kubectl(
        [
            "delete",
            "certificatesigningrequest",
            csr,
        ],
        kubeconfig=tf_control_plane["kubeconfig_admin"],
    )


## Tests
@pytest.mark.ccm
@pytest.mark.skipif(
    TEST_CCM_TYPE not in ["kubeadm"],
    reason="This test may only be performed for 'kubeadm' type (<-> external node kubeconfig/username)",
)
def test_ccm_node_csrs_invalid(test, ccm, k8s_csr_invalid_ip_address, logger):
    csr = k8s_csr_invalid_ip_address
    (lines, match, unmatch) = ioMatch(
        ccm,
        matches=[
            f"exoscale-ccm: sks-agent: CSR {csr} Node IP addresses don't match corresponding Compute instance IP addresses"
        ],
        unmatches=[f"exoscale-ccm: sks-agent: CSR {csr} approved"],
        timeout=test["timeout"]["ccm"]["csr_approve"],
        logger=logger,
    )
    assert lines > 0
    assert match is not None
    assert unmatch is None

    logger.info(f"[CCM] Ignored invalid CSR: {csr}")
