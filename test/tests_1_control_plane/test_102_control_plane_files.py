import os.path

import pytest


@pytest.mark.control_plane
def test_kubernetes_cni(test, tf_control_plane):
    cni = tf_control_plane["kubernetes_cni"]
    path = os.path.join(test["directory"], "resources", "manifests", f"cni-{cni}.yaml")
    assert os.path.exists(path)


@pytest.mark.control_plane
def test_kubeconfig_admin(tf_control_plane):
    path = tf_control_plane["kubeconfig_admin"]
    assert "/output/" in path
    assert "kubeconfig" in path
    assert os.path.exists(path)


@pytest.mark.control_plane
def test_kubeconfig_ccm(tf_control_plane):
    path = tf_control_plane["kubeconfig_ccm"]
    assert "/output/" in path
    assert "kubeconfig" in path
    assert os.path.exists(path)


@pytest.mark.control_plane
def test_ccm_rbac(tf_control_plane):
    path = tf_control_plane["ccm_rbac"]
    assert "/output/" in path
    assert "rbac" in path
    assert os.path.exists(path)


@pytest.mark.control_plane
def test_ccm_cloud_config(tf_control_plane):
    path = tf_control_plane["ccm_cloud_config"]
    assert "/output/" in path
    assert "cloud-config" in path
    assert os.path.exists(path)


@pytest.mark.control_plane
def test_ccm_main(tf_control_plane):
    path = tf_control_plane["ccm_main"]
    assert "/cmd/" in path
    assert "/main.go" in path
    assert os.path.exists(path)
