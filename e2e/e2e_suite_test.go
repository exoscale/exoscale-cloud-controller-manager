package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	exoscale "github.com/exoscale/egoscale/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/exoscale/exoscale-cloud-controller-manager/e2e/manager"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Exoscale Cloud Controller Manager E2E Suite")
}

var (
	suite          *TestSuite
	ctx            context.Context
	cancel         context.CancelFunc
	skipCleanup    bool
	kubeconfigPath string
)

type TestSuite struct {
	Config      *manager.Config
	Client      *exoscale.Client
	ClusterMgr  *manager.ClusterManager
	NodepoolMgr *manager.NodepoolManager
	InstanceMgr *manager.InstanceManager
	K8sClient   *manager.KubernetesClient
	CCMMgr      *manager.CCMManager
}

func (s *TestSuite) GetManifestReplacements() map[string]string {
	replacements := map[string]string{
		"${exoscale_zone}": s.Config.Zone,
	}

	if s.NodepoolMgr != nil {
		nodepool := s.NodepoolMgr.GetNodepool()
		if nodepool != nil && nodepool.InstancePool != nil {
			replacements["${exoscale_instance_pool_id}"] = nodepool.InstancePool.ID.String()
		}
	}

	return replacements
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Minute)
	skipCleanup = os.Getenv("E2E_SKIP_CLEANUP") != ""

	By("Loading configuration from environment")
	config, err := manager.NewConfigFromEnv()
	Expect(err).NotTo(HaveOccurred())
	config.TestID = fmt.Sprintf("e2e-%d", time.Now().UnixNano())

	By("Creating Exoscale API client")
	client, err := config.NewExoscaleClient()
	Expect(err).NotTo(HaveOccurred())

	By(fmt.Sprintf("Creating SKS cluster (test ID: %s)", config.TestID))
	clusterMgr := manager.NewClusterManager(client, config)
	err = clusterMgr.CreateCluster(ctx)
	Expect(err).NotTo(HaveOccurred())

	GinkgoWriter.Printf("Cluster created: %s (version %s)\n", clusterMgr.GetClusterID(), config.KubernetesVersion)

	By("Waiting for cluster to be running")
	err = clusterMgr.WaitForClusterRunning(ctx)
	Expect(err).NotTo(HaveOccurred())

	By("Generating kubeconfig")
	kubeconfig, err := clusterMgr.GetKubeconfig(ctx, "admin", []string{"system:masters"})
	Expect(err).NotTo(HaveOccurred())

	kubeconfigPath = fmt.Sprintf("./kubeconfig-%s", config.TestID)
	err = os.WriteFile(kubeconfigPath, kubeconfig, 0600)
	Expect(err).NotTo(HaveOccurred())

	By("Creating nodepool")
	nodepoolMgr, err := manager.NewNodepoolManager(client, config, clusterMgr.GetClusterID())
	Expect(err).NotTo(HaveOccurred())

	err = nodepoolMgr.CreateNodepool(ctx, config.NodepoolSize)
	Expect(err).NotTo(HaveOccurred())

	err = nodepoolMgr.WaitForNodepoolRunning(ctx)
	Expect(err).NotTo(HaveOccurred())

	GinkgoWriter.Printf("Kubeconfig written to: %s\n", kubeconfigPath)

	By("Creating Kubernetes client")
	k8sClient, err := manager.NewKubernetesClient(kubeconfig, config)
	Expect(err).NotTo(HaveOccurred())

	By("Starting Cloud Controller Manager")
	ccmMgr := manager.NewCCMManager(config, kubeconfigPath)
	err = ccmMgr.Start(ctx)
	Expect(err).NotTo(HaveOccurred())

	By(fmt.Sprintf("Waiting for %d nodes to be ready", config.NodepoolSize))
	err = k8sClient.WaitForNodesReady(ctx, int(config.NodepoolSize))
	Expect(err).NotTo(HaveOccurred())

	By("Creating static compute instance with nodepool security groups")
	instanceMgr := manager.NewInstanceManager(client, config, clusterMgr)

	securityGroups, err := nodepoolMgr.GetInstancePoolSecurityGroups(ctx)
	Expect(err).NotTo(HaveOccurred())

	var sgIDs []exoscale.SecurityGroup
	for _, sg := range securityGroups {
		sgIDs = append(sgIDs, exoscale.SecurityGroup{ID: sg.ID})
	}

	err = instanceMgr.CreateInstance(ctx, sgIDs)
	Expect(err).NotTo(HaveOccurred())

	err = instanceMgr.WaitForInstanceRunning(ctx)
	Expect(err).NotTo(HaveOccurred())

	GinkgoWriter.Printf("Instance created: %s\n", instanceMgr.GetInstanceID())

	suite = &TestSuite{
		Config:      config,
		Client:      client,
		ClusterMgr:  clusterMgr,
		NodepoolMgr: nodepoolMgr,
		InstanceMgr: instanceMgr,
		K8sClient:   k8sClient,
		CCMMgr:      ccmMgr,
	}

	GinkgoWriter.Println("Test suite setup complete!")
})

var _ = AfterSuite(func() {
	if skipCleanup {
		GinkgoWriter.Println("\n=== Skipping cleanup (E2E_SKIP_CLEANUP is set) ===")
		return
	}

	if suite == nil {
		return
	}

	cleanupCtx := context.Background()
	GinkgoWriter.Println("\n=== Cleaning up test resources ===")

	if suite.CCMMgr != nil {
		_ = suite.CCMMgr.Stop()
	}

	if suite.InstanceMgr != nil {
		GinkgoWriter.Println("Deleting static instance (if not already deleted)...")
		if err := suite.InstanceMgr.DeleteInstance(cleanupCtx); err != nil {
			GinkgoWriter.Printf("Note: instance cleanup: %v\n", err)
		}
	}

	if suite.NodepoolMgr != nil {
		GinkgoWriter.Println("Deleting nodepool...")
		_ = suite.NodepoolMgr.DeleteNodepool(cleanupCtx)
	}

	if suite.ClusterMgr != nil {
		GinkgoWriter.Println("Deleting cluster...")
		_ = suite.ClusterMgr.DeleteCluster(cleanupCtx)
	}

	GinkgoWriter.Println("Cleaning up files...")
	if kubeconfigPath != "" {
		_ = os.Remove(kubeconfigPath)
	}

	if cancel != nil {
		cancel()
	}

	GinkgoWriter.Println("=== Cleanup complete ===")
})
