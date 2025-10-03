package manager

import (
	"context"
	"fmt"
	"os"
	"time"

	exoscale "github.com/exoscale/egoscale/v3"
	"github.com/exoscale/egoscale/v3/credentials"
)

type Config struct {
	Zone              string
	APIKey            string
	APISecret         string
	TestID            string
	KubernetesVersion string
	NodepoolSize      int64
	InstanceType      string
	TemplateID        string
	DiskSize          int64
	Timeouts          Timeouts
}

type Timeouts struct {
	ClusterCreate         time.Duration
	ClusterDelete         time.Duration
	NodepoolCreate        time.Duration
	NodepoolDelete        time.Duration
	NodepoolResize        time.Duration
	NodeReady             time.Duration
	CCMStart              time.Duration
	CSRApproval           time.Duration
	NodeDeletion          time.Duration
	NLBCreate             time.Duration
	NLBServiceStart       time.Duration
	NLBHealthcheckSuccess time.Duration
	InstanceCreate        time.Duration
	InstanceDelete        time.Duration
}

func DefaultTimeouts() Timeouts {
	return Timeouts{
		ClusterCreate:         2 * time.Minute,
		ClusterDelete:         2 * time.Minute,
		NodepoolCreate:        5 * time.Minute,
		NodepoolDelete:        5 * time.Minute,
		NodepoolResize:        5 * time.Minute,
		NodeReady:             5 * time.Minute,
		CCMStart:              2 * time.Minute,
		CSRApproval:           1 * time.Minute,
		NodeDeletion:          1 * time.Minute,
		NLBCreate:             2 * time.Minute,
		NLBServiceStart:       3 * time.Minute,
		NLBHealthcheckSuccess: 2 * time.Minute,
		InstanceCreate:        5 * time.Minute,
		InstanceDelete:        3 * time.Minute,
	}
}

func NewConfigFromEnv() (*Config, error) {
	apiKey := os.Getenv("EXOSCALE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("EXOSCALE_API_KEY environment variable is required")
	}

	apiSecret := os.Getenv("EXOSCALE_API_SECRET")
	if apiSecret == "" {
		return nil, fmt.Errorf("EXOSCALE_API_SECRET environment variable is required")
	}

	zone := os.Getenv("EXOSCALE_ZONE")
	if zone == "" {
		zone = "ch-gva-2"
	}

	testID := os.Getenv("TEST_ID")
	if testID == "" {
		testID = fmt.Sprintf("test-ccm-%d", time.Now().Unix())
	}

	client, err := (&Config{APIKey: apiKey, APISecret: apiSecret}).NewExoscaleClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	k8sVersion := os.Getenv("KUBERNETES_VERSION")
	if k8sVersion == "" {
		k8sVersion, err = getLatestKubernetesVersion(client)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest Kubernetes version: %w", err)
		}
	}

	templateVariant := os.Getenv("TEMPLATE_VARIANT")
	if templateVariant == "" {
		templateVariant = "standard"
	}

	templateID, err := getNodepoolTemplateID(client, k8sVersion, templateVariant)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodepool template: %w", err)
	}

	config := &Config{
		Zone:              zone,
		APIKey:            apiKey,
		APISecret:         apiSecret,
		TestID:            testID,
		KubernetesVersion: k8sVersion,
		NodepoolSize:      1,
		InstanceType:      "standard.medium",
		TemplateID:        templateID,
		DiskSize:          50,
		Timeouts:          DefaultTimeouts(),
	}

	return config, nil
}

func getLatestKubernetesVersion(client *exoscale.Client) (string, error) {
	ctx := context.Background()

	versions, err := client.ListSKSClusterVersions(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list SKS versions: %w", err)
	}

	if len(versions.SKSClusterVersions) == 0 {
		return "", fmt.Errorf("no Kubernetes versions available")
	}

	var versionList []string
	for _, v := range versions.SKSClusterVersions {
		versionList = append(versionList, v)
	}

	return versionList[len(versionList)-1], nil
}

func getNodepoolTemplateID(client *exoscale.Client, version, variant string) (string, error) {
	ctx := context.Background()

	templateVariant := exoscale.GetActiveNodepoolTemplateVariantStandard
	if variant == "nvidia" {
		templateVariant = exoscale.GetActiveNodepoolTemplateVariantNvidia
	} else if variant != "standard" {
		return "", fmt.Errorf("unknown template variant: %s", variant)
	}

	template, err := client.GetActiveNodepoolTemplate(ctx, version, templateVariant)
	if err != nil {
		return "", fmt.Errorf("failed to get active nodepool template: %w", err)
	}

	return template.ActiveTemplate.String(), nil
}

func (c *Config) NewExoscaleClient() (*exoscale.Client, error) {
	creds := credentials.NewStaticCredentials(c.APIKey, c.APISecret)
	client, err := exoscale.NewClient(creds)
	if err != nil {
		return nil, fmt.Errorf("failed to create Exoscale client: %w", err)
	}
	return client, nil
}
