package manager

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"time"

	exoscale "github.com/exoscale/egoscale/v3"
)

type ClusterManager struct {
	client    *exoscale.Client
	config    *Config
	clusterID exoscale.UUID
	cluster   *exoscale.SKSCluster
}

func NewClusterManager(client *exoscale.Client, config *Config) *ClusterManager {
	return &ClusterManager{
		client: client,
		config: config,
	}
}

func (cm *ClusterManager) GetAvailableKubernetesVersions(ctx context.Context) ([]string, error) {
	versions, err := cm.client.ListSKSClusterVersions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list SKS versions: %w", err)
	}

	if len(versions.SKSClusterVersions) == 0 {
		return nil, fmt.Errorf("no Kubernetes versions available")
	}

	var versionList []string
	for _, v := range versions.SKSClusterVersions {
		versionList = append(versionList, v)
	}

	sort.Strings(versionList)
	return versionList, nil
}

func (cm *ClusterManager) GetLatestKubernetesVersion(ctx context.Context) (string, error) {
	versions, err := cm.GetAvailableKubernetesVersions(ctx)
	if err != nil {
		return "", err
	}

	return versions[len(versions)-1], nil
}

func (cm *ClusterManager) CreateCluster(ctx context.Context) error {
	version := cm.config.KubernetesVersion
	if version == "" {
		var err error
		version, err = cm.GetLatestKubernetesVersion(ctx)
		if err != nil {
			return fmt.Errorf("failed to get latest Kubernetes version: %w", err)
		}
		cm.config.KubernetesVersion = version
	}

	clusterName := fmt.Sprintf("%s-cluster", cm.config.TestID)

	createDefaultSG := true
	createReq := exoscale.CreateSKSClusterRequest{
		Name:                       clusterName,
		Version:                    version,
		Cni:                        exoscale.CreateSKSClusterRequestCniCalico,
		Level:                      exoscale.CreateSKSClusterRequestLevelPro,
		CreateDefaultSecurityGroup: &createDefaultSG,
	}

	ctx, cancel := context.WithTimeout(ctx, cm.config.Timeouts.ClusterCreate)
	defer cancel()

	op, err := cm.client.CreateSKSCluster(ctx, createReq)
	if err != nil {
		return fmt.Errorf("failed to create SKS cluster: %w", err)
	}

	_, err = cm.client.Wait(ctx, op, exoscale.OperationStateSuccess)
	if err != nil {
		return fmt.Errorf("failed to wait for cluster creation: %w", err)
	}

	cm.clusterID = op.Reference.ID

	cluster, err := cm.client.GetSKSCluster(ctx, cm.clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster details: %w", err)
	}

	cm.cluster = cluster
	return nil
}

func (cm *ClusterManager) GetKubeconfig(ctx context.Context, user string, groups []string) ([]byte, error) {
	var zeroUUID exoscale.UUID
	if cm.clusterID == zeroUUID {
		return nil, fmt.Errorf("cluster not created")
	}

	kubeconfigReq := exoscale.SKSKubeconfigRequest{
		User:   user,
		Groups: groups,
		Ttl:    86400,
	}

	resp, err := cm.client.GenerateSKSClusterKubeconfig(ctx, cm.clusterID, kubeconfigReq)
	if err != nil {
		return nil, fmt.Errorf("failed to generate kubeconfig: %w", err)
	}

	kubeconfig, err := base64.StdEncoding.DecodeString(resp.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to decode kubeconfig: %w", err)
	}

	return kubeconfig, nil
}

func (cm *ClusterManager) DeleteCluster(ctx context.Context) error {
	var zeroUUID exoscale.UUID
	if cm.clusterID == zeroUUID {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, cm.config.Timeouts.ClusterDelete)
	defer cancel()

	op, err := cm.client.DeleteSKSCluster(ctx, cm.clusterID)
	if err != nil {
		return fmt.Errorf("failed to delete cluster: %w", err)
	}

	_, err = cm.client.Wait(ctx, op, exoscale.OperationStateSuccess)
	if err != nil {
		return fmt.Errorf("failed to wait for cluster deletion: %w", err)
	}

	return nil
}

func (cm *ClusterManager) GetClusterID() string {
	return cm.clusterID.String()
}

func (cm *ClusterManager) GetCluster() *exoscale.SKSCluster {
	return cm.cluster
}

func (cm *ClusterManager) WaitForClusterRunning(ctx context.Context) error {
	timeout := time.After(cm.config.Timeouts.ClusterCreate)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for cluster to be running")
		case <-ticker.C:
			cluster, err := cm.client.GetSKSCluster(ctx, cm.clusterID)
			if err != nil {
				return fmt.Errorf("failed to get cluster status: %w", err)
			}

			if cluster.State == exoscale.SKSClusterStateRunning {
				cm.cluster = cluster
				return nil
			}
		}
	}
}
