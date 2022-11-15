from helpers import reUUID, reIPv4

import pytest


@pytest.mark.control_plane
def test_control_plane_outputs(test, tf_control_plane):
    # Test
    assert "test_id" in tf_control_plane
    assert "test_name" in tf_control_plane

    # Exoscale
    assert "exoscale_zone" in tf_control_plane
    assert "exoscale_environment" in tf_control_plane

    # Cluster
    assert "cluster_id" in tf_control_plane
    if test["type"] not in ["kubeadm"]:
        assert reUUID.match(tf_control_plane["cluster_id"])
    assert "cluster_sg_id" in tf_control_plane
    assert reUUID.match(tf_control_plane["cluster_sg_id"])

    # Control plane
    assert "control_plane_node" in tf_control_plane
    assert "control_plane_ipv4" in tf_control_plane
    if test["type"] not in ["sks"]:
        assert reIPv4.match(tf_control_plane["control_plane_ipv4"])
    assert "control_plane_endpoint" in tf_control_plane

    # Nodes
    assert "nodes_bootstrap_token" in tf_control_plane
    assert "nodes_ssh_key_name" in tf_control_plane

    # Kubernetes configuration and credentials
    assert "kubernetes_cni" in tf_control_plane
    assert "kubeconfig_admin" in tf_control_plane
    assert "kubeconfig_ccm" in tf_control_plane

    # CCM configuration and credentials
    assert "ccm_rbac" in tf_control_plane
    assert "ccm_cloud_config" in tf_control_plane
    assert "ccm_api_credentials" in tf_control_plane
    # (source and executable)
    assert "ccm_main" in tf_control_plane
    assert "ccm_exe" in tf_control_plane
