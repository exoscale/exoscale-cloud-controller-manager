package manager

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type KubernetesClient struct {
	clientset *kubernetes.Clientset
	config    *Config
}

func NewKubernetesClient(kubeconfig []byte, config *Config) (*KubernetesClient, error) {
	clientConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create REST config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &KubernetesClient{
		clientset: clientset,
		config:    config,
	}, nil
}

func (kc *KubernetesClient) Clientset() *kubernetes.Clientset {
	return kc.clientset
}

func retryKubeAPICall[T any](fn func() (T, error)) (T, error) {
	var result T
	var err error

	for i := 0; i < 3; i++ {
		result, err = fn()
		if err == nil {
			return result, nil
		}
		if i < 2 {
			time.Sleep(time.Second)
		}
	}
	return result, err
}

func (kc *KubernetesClient) GetNodes(ctx context.Context) ([]corev1.Node, error) {
	nodeList, err := retryKubeAPICall(func() (*corev1.NodeList, error) {
		return kc.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	return nodeList.Items, nil
}

func (kc *KubernetesClient) GetReadyNodes(ctx context.Context) ([]corev1.Node, error) {
	nodes, err := kc.GetNodes(ctx)
	if err != nil {
		return nil, err
	}

	var readyNodes []corev1.Node
	for _, node := range nodes {
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				readyNodes = append(readyNodes, node)
				break
			}
		}
	}

	return readyNodes, nil
}

func (kc *KubernetesClient) WaitForNodesReady(ctx context.Context, expectedCount int) error {
	timeout := time.After(kc.config.Timeouts.NodeReady)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for %d nodes to be ready", expectedCount)
		case <-ticker.C:
			readyNodes, err := kc.GetReadyNodes(ctx)
			if err != nil {
				continue
			}

			if len(readyNodes) >= expectedCount {
				return nil
			}
		}
	}
}

func (kc *KubernetesClient) GetNodeCSRs(ctx context.Context) ([]certificatesv1.CertificateSigningRequest, error) {
	csrList, err := retryKubeAPICall(func() (*certificatesv1.CertificateSigningRequestList, error) {
		return kc.clientset.CertificatesV1().CertificateSigningRequests().List(ctx, metav1.ListOptions{})
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list CSRs: %w", err)
	}

	var nodeCSRs []certificatesv1.CertificateSigningRequest
	for _, csr := range csrList.Items {
		if strings.HasPrefix(csr.Spec.Username, "system:node:") {
			nodeCSRs = append(nodeCSRs, csr)
		}
	}

	return nodeCSRs, nil
}

func (kc *KubernetesClient) GetApprovedCSRs(ctx context.Context) ([]certificatesv1.CertificateSigningRequest, error) {
	csrs, err := kc.GetNodeCSRs(ctx)
	if err != nil {
		return nil, err
	}

	var approvedCSRs []certificatesv1.CertificateSigningRequest
	for _, csr := range csrs {
		for _, condition := range csr.Status.Conditions {
			if condition.Type == certificatesv1.CertificateApproved &&
				condition.Reason == "ExoscaleCloudControllerApproved" &&
				len(csr.Status.Certificate) > 0 {
				approvedCSRs = append(approvedCSRs, csr)
				break
			}
		}
	}

	return approvedCSRs, nil
}

func (kc *KubernetesClient) WaitForCSRsApproved(ctx context.Context, expectedCount int) error {
	timeout := time.After(kc.config.Timeouts.CSRApproval)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for %d CSRs to be approved", expectedCount)
		case <-ticker.C:
			approvedCSRs, err := kc.GetApprovedCSRs(ctx)
			if err != nil {
				continue
			}

			if len(approvedCSRs) >= expectedCount {
				return nil
			}
		}
	}
}

func (kc *KubernetesClient) CountNodesByProviderID(ctx context.Context) (int, error) {
	nodes, err := kc.GetNodes(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, node := range nodes {
		if strings.HasPrefix(node.Spec.ProviderID, "exoscale://") {
			count++
		}
	}

	return count, nil
}

func (kc *KubernetesClient) WaitForNodesWithProviderID(ctx context.Context, expectedCount int) error {
	timeout := time.After(kc.config.Timeouts.NodeReady)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for %d nodes to have provider ID", expectedCount)
		case <-ticker.C:
			count, err := kc.CountNodesByProviderID(ctx)
			if err != nil {
				continue
			}

			if count >= expectedCount {
				return nil
			}
		}
	}
}

func (kc *KubernetesClient) GetNodeByName(ctx context.Context, name string) (*corev1.Node, error) {
	node, err := retryKubeAPICall(func() (*corev1.Node, error) {
		return kc.clientset.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get node %s: %w", name, err)
	}

	return node, nil
}

func (kc *KubernetesClient) GetBootstrapToken(ctx context.Context) (string, error) {
	secrets, err := retryKubeAPICall(func() (*corev1.SecretList, error) {
		return kc.clientset.CoreV1().Secrets("kube-system").List(ctx, metav1.ListOptions{})
	})
	if err != nil {
		return "", fmt.Errorf("failed to list secrets in kube-system: %w", err)
	}

	for _, secret := range secrets.Items {
		if secret.Type == "bootstrap.kubernetes.io/token" {
			tokenID, hasID := secret.Data["token-id"]
			tokenSecret, hasSecret := secret.Data["token-secret"]
			if hasID && hasSecret {
				return fmt.Sprintf("%s.%s", string(tokenID), string(tokenSecret)), nil
			}
		}
	}

	return "", fmt.Errorf("no bootstrap token found in kube-system namespace")
}

func (kc *KubernetesClient) GetServices(ctx context.Context, namespace string) ([]corev1.Service, error) {
	serviceList, err := retryKubeAPICall(func() (*corev1.ServiceList, error) {
		return kc.clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	return serviceList.Items, nil
}

func (kc *KubernetesClient) WaitForStaticNodeToJoin(ctx context.Context, instanceIP string) (*corev1.Node, error) {
	timeout := time.After(kc.config.Timeouts.NodeReady)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for static node with IP %s to join", instanceIP)
		case <-ticker.C:
			nodes, err := kc.GetNodes(ctx)
			if err != nil {
				continue
			}

			for _, node := range nodes {
				for _, addr := range node.Status.Addresses {
					if (addr.Type == corev1.NodeExternalIP || addr.Type == corev1.NodeInternalIP) &&
						addr.Address == instanceIP {
						return &node, nil
					}
				}
			}
		}
	}
}

func (kc *KubernetesClient) WaitForNodeProviderID(ctx context.Context, nodeName string) (*corev1.Node, error) {
	timeout := time.After(kc.config.Timeouts.CCMStart)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for node %s to have provider ID set", nodeName)
		case <-ticker.C:
			node, err := kc.GetNodeByName(ctx, nodeName)
			if err != nil {
				continue
			}

			if node.Spec.ProviderID != "" {
				return node, nil
			}
		}
	}
}

func (kc *KubernetesClient) WaitForServiceLoadBalancer(ctx context.Context, namespace, name string) (*corev1.Service, error) {
	timeout := time.After(kc.config.Timeouts.NLBServiceStart)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for service %s/%s to get external IP", namespace, name)
		case <-ticker.C:
			svc, err := retryKubeAPICall(func() (*corev1.Service, error) {
				return kc.clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
			})
			if err != nil {
				continue
			}

			if svc.Spec.Type == corev1.ServiceTypeLoadBalancer &&
				len(svc.Status.LoadBalancer.Ingress) > 0 &&
				svc.Status.LoadBalancer.Ingress[0].IP != "" {
				return svc, nil
			}
		}
	}
}

func (kc *KubernetesClient) ApplyManifest(ctx context.Context, kubeconfigPath, manifestPath string) error {
	logFile, err := os.OpenFile("kubectl.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open kubectl.log: %w", err)
	}
	defer logFile.Close()

	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", manifestPath, "--kubeconfig", kubeconfigPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	return cmd.Run()
}

func (kc *KubernetesClient) ApplyManifestWithReplacements(ctx context.Context, kubeconfigPath, manifestPath string, replacements map[string]string) error {
	if len(replacements) == 0 {
		return kc.ApplyManifest(ctx, kubeconfigPath, manifestPath)
	}

	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	manifestContent := string(content)
	for key, value := range replacements {
		manifestContent = strings.ReplaceAll(manifestContent, key, value)
	}

	tmpFile, err := os.CreateTemp("", "manifest-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(manifestContent); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp manifest: %w", err)
	}
	tmpFile.Close()

	logFile, err := os.OpenFile("kubectl.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open kubectl.log: %w", err)
	}
	defer logFile.Close()

	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", tmpFile.Name(), "--kubeconfig", kubeconfigPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	return cmd.Run()
}

func (kc *KubernetesClient) DeleteManifest(ctx context.Context, kubeconfigPath, manifestPath string) error {
	logFile, err := os.OpenFile("kubectl.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open kubectl.log: %w", err)
	}
	defer logFile.Close()

	cmd := exec.CommandContext(ctx, "kubectl", "delete", "-f", manifestPath, "--kubeconfig", kubeconfigPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	return cmd.Run()
}

func (kc *KubernetesClient) WaitForDeploymentAvailable(ctx context.Context, namespace, name string, timeout time.Duration) error {
	timeoutCh := time.After(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCh:
			return fmt.Errorf("timeout waiting for deployment %s/%s to be available", namespace, name)
		case <-ticker.C:
			deployment, err := retryKubeAPICall(func() (*appsv1.Deployment, error) {
				return kc.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
			})
			if err != nil {
				continue
			}

			for _, condition := range deployment.Status.Conditions {
				if condition.Type == appsv1.DeploymentAvailable && condition.Status == corev1.ConditionTrue {
					return nil
				}
			}
		}
	}
}

func (kc *KubernetesClient) TestHTTPEndpoint(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	return nil
}
