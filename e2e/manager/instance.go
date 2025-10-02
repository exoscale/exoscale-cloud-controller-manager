package manager

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	exoscale "github.com/exoscale/egoscale/v3"
)

type InstanceManager struct {
	client     *exoscale.Client
	config     *Config
	clusterMgr *ClusterManager
	instanceID exoscale.UUID
	instance   *exoscale.Instance
}

func NewInstanceManager(client *exoscale.Client, config *Config, clusterMgr *ClusterManager) *InstanceManager {
	return &InstanceManager{
		client:     client,
		config:     config,
		clusterMgr: clusterMgr,
	}
}

func (im *InstanceManager) CreateInstance(ctx context.Context, securityGroups []exoscale.SecurityGroup) error {
	instanceName := fmt.Sprintf("%s-static", im.config.TestID)

	templateID, err := exoscale.ParseUUID(im.config.TemplateID)
	if err != nil {
		return fmt.Errorf("invalid template ID: %w", err)
	}

	instanceTypes, err := im.client.ListInstanceTypes(ctx)
	if err != nil {
		return fmt.Errorf("failed to list instance types: %w", err)
	}

	instanceType, err := instanceTypes.FindInstanceTypeByIdOrFamilyAndSize(im.config.InstanceType)
	if err != nil {
		return fmt.Errorf("failed to find instance type %q: %w", im.config.InstanceType, err)
	}

	userData, err := im.buildUserData()
	if err != nil {
		return fmt.Errorf("failed to build user data: %w", err)
	}

	createReq := exoscale.CreateInstanceRequest{
		DiskSize:       im.config.DiskSize,
		InstanceType:   &instanceType,
		Name:           instanceName,
		Template:       &exoscale.Template{ID: templateID},
		UserData:       userData,
		SecurityGroups: securityGroups,
	}

	fmt.Printf("[CreateInstance] Creating instance '%s' with TestID: %s\n", instanceName, im.config.TestID)

	op, err := im.client.CreateInstance(ctx, createReq)
	if err != nil {
		return fmt.Errorf("failed to create instance: %w", err)
	}

	im.instanceID = op.Reference.ID
	fmt.Printf("[CreateInstance] Instance creation initiated, ID: %s\n", im.instanceID)
	return nil
}

func (im *InstanceManager) buildUserData() (string, error) {
	ctx := context.Background()

	kubeconfigBytes, err := im.clusterMgr.GetKubeconfig(ctx, "admin", []string{"system:masters"})
	if err != nil {
		return "", fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	kubeconfig := string(kubeconfigBytes)

	apiServerURL, err := extractServerFromKubeconfig(kubeconfig)
	if err != nil {
		return "", fmt.Errorf("failed to extract server URL: %w", err)
	}

	caCertBase64, err := extractCACertFromKubeconfig(kubeconfig)
	if err != nil {
		return "", fmt.Errorf("failed to extract CA cert: %w", err)
	}

	k8sClient, err := NewKubernetesClient(kubeconfigBytes, im.config)
	if err != nil {
		return "", fmt.Errorf("failed to create k8s client: %w", err)
	}

	bootstrapToken, err := k8sClient.GetBootstrapToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get bootstrap token: %w", err)
	}

	clusterDomain := "cluster.local"

	userDataToml := fmt.Sprintf(`[settings.kubernetes]
api-server = "%s"
bootstrap-token = "%s"
cluster-domain = "%s"
cloud-provider = "external"
cluster-certificate = "%s"

`, apiServerURL, bootstrapToken, clusterDomain, caCertBase64)

	userDataFile := fmt.Sprintf("./userdata-%s.toml", im.config.TestID)
	if err := os.WriteFile(userDataFile, []byte(userDataToml), 0600); err != nil {
		return "", fmt.Errorf("failed to write user data file: %w", err)
	}

	return base64.StdEncoding.EncodeToString([]byte(userDataToml)), nil
}

func extractFieldFromKubeconfig(kubeconfig, fieldPrefix, fieldName string) (string, error) {
	startIdx := -1
	for i := 0; i < len(kubeconfig)-len(fieldPrefix); i++ {
		if kubeconfig[i:i+len(fieldPrefix)] == fieldPrefix {
			startIdx = i + len(fieldPrefix)
			break
		}
	}
	if startIdx == -1 {
		return "", fmt.Errorf("%s not found in kubeconfig", fieldName)
	}

	endIdx := startIdx
	for endIdx < len(kubeconfig) && kubeconfig[endIdx] != '\n' && kubeconfig[endIdx] != '\r' {
		endIdx++
	}

	return kubeconfig[startIdx:endIdx], nil
}

func extractServerFromKubeconfig(kubeconfig string) (string, error) {
	return extractFieldFromKubeconfig(kubeconfig, "server: ", "server")
}

func extractCACertFromKubeconfig(kubeconfig string) (string, error) {
	return extractFieldFromKubeconfig(kubeconfig, "certificate-authority-data: ", "certificate-authority-data")
}

func (im *InstanceManager) DeleteInstance(ctx context.Context) error {
	var zeroUUID exoscale.UUID
	if im.instanceID == zeroUUID {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, im.config.Timeouts.InstanceDelete)
	defer cancel()

	op, err := im.client.DeleteInstance(ctx, im.instanceID)
	if err != nil {
		return fmt.Errorf("failed to delete instance: %w", err)
	}

	_, err = im.client.Wait(ctx, op, exoscale.OperationStateSuccess)
	if err != nil {
		return fmt.Errorf("failed to wait for instance deletion: %w", err)
	}

	return nil
}

func (im *InstanceManager) GetInstanceID() string {
	return im.instanceID.String()
}

func (im *InstanceManager) GetInstance() *exoscale.Instance {
	return im.instance
}

func (im *InstanceManager) WaitForInstanceRunning(ctx context.Context) error {
	fmt.Printf("[WaitForInstanceRunning] Waiting for instance %s to be running (timeout: %s)\n",
		im.instanceID, im.config.Timeouts.InstanceCreate)

	timeout := time.After(im.config.Timeouts.InstanceCreate)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for instance %s to be running", im.instanceID)
		case <-ticker.C:
			instance, err := im.client.GetInstance(ctx, im.instanceID)
			if err != nil {
				return fmt.Errorf("failed to get instance status: %w", err)
			}

			fmt.Printf("[WaitForInstanceRunning] Instance %s state: %s\n", im.instanceID, instance.State)

			if instance.State == exoscale.InstanceStateRunning {
				im.instance = instance
				fmt.Printf("[WaitForInstanceRunning] Instance %s is now running\n", im.instanceID)
				return nil
			}
		}
	}
}
