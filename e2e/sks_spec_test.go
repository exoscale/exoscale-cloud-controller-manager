package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	exoscale "github.com/exoscale/egoscale/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

var _ = Describe("Exoscale Cloud Controller Manager", Ordered, func() {
	Describe("Infrastructure Setup", func() {
		It("should have created the SKS cluster", func() {
			Expect(suite.ClusterMgr).NotTo(BeNil())
			Expect(suite.ClusterMgr.GetClusterID()).NotTo(BeEmpty())
			GinkgoWriter.Printf("SKS cluster ready: %s (version %s)\n",
				suite.ClusterMgr.GetClusterID(), suite.Config.KubernetesVersion)
		})

		It("should have created the nodepool", func() {
			Expect(suite.NodepoolMgr).NotTo(BeNil())
		})

		It("should have created the static instance", func() {
			Expect(suite.InstanceMgr).NotTo(BeNil())
			Expect(suite.InstanceMgr.GetInstanceID()).NotTo(BeEmpty())
			GinkgoWriter.Printf("Static instance ready: %s\n", suite.InstanceMgr.GetInstanceID())
		})

		It("should have initialized the Kubernetes client", func() {
			Expect(suite.K8sClient).NotTo(BeNil())
		})
	})

	Describe("Kubernetes State", func() {
		It("should have all nodes available", func() {
			expectedNodeCount := int(suite.Config.NodepoolSize) + 1 // nodepool + static instance

			timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()

			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-timeoutCtx.Done():
					Fail(fmt.Sprintf("Timeout waiting for %d nodes to be available", expectedNodeCount))
				case <-ticker.C:
					nodes, err := suite.K8sClient.GetNodes(timeoutCtx)
					if err != nil {
						continue
					}

					if len(nodes) >= expectedNodeCount {
						GinkgoWriter.Printf("Available nodes: %d\n", len(nodes))
						return
					}
				}
			}
		})

		It("should have node CSRs", func() {
			csrs, err := suite.K8sClient.GetNodeCSRs(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(csrs).NotTo(BeEmpty())
			GinkgoWriter.Printf("Node CSRs found: %d\n", len(csrs))
		})
	})

	Describe("Static Instance", func() {
		var instance *exoscale.Instance
		var instanceIP string

		BeforeEach(func() {
			instance = suite.InstanceMgr.GetInstance()
			Expect(instance).NotTo(BeNil())
		})

		It("should be in running state", func() {
			Expect(instance.State).To(Equal(exoscale.InstanceStateRunning))
		})

		It("should have a public IP address", func() {
			Expect(instance.PublicIP.IsUnspecified()).To(BeFalse())
			instanceIP = instance.PublicIP.String()
			GinkgoWriter.Printf("Static instance: %s (IP: %s)\n",
				suite.InstanceMgr.GetInstanceID(), instanceIP)
		})

		It("should have a valid bootstrap token", func() {
			bootstrapToken, err := suite.K8sClient.GetBootstrapToken(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(bootstrapToken).NotTo(BeEmpty())
			Expect(bootstrapToken).To(ContainSubstring("."))
			GinkgoWriter.Printf("Bootstrap token retrieved: %s...\n", bootstrapToken[:6])
		})

		Context("when joined to the cluster", func() {
			var staticNode *corev1.Node

			It("should join the cluster as a node", func() {
				instance := suite.InstanceMgr.GetInstance()
				instanceIP := instance.PublicIP.String()

				var err error
				staticNode, err = suite.K8sClient.WaitForStaticNodeToJoin(ctx, instanceIP)
				Expect(err).NotTo(HaveOccurred())
				GinkgoWriter.Printf("Static node joined: %s (IP: %s)\n", staticNode.Name, instanceIP)
			})

			It("should have the correct provider ID", func() {
				node, err := suite.K8sClient.WaitForNodeProviderID(ctx, staticNode.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(node.Spec.ProviderID).NotTo(BeEmpty())
				Expect(node.Spec.ProviderID).To(ContainSubstring("exoscale://"))

				expectedProviderID := "exoscale://" + suite.InstanceMgr.GetInstanceID()
				Expect(node.Spec.ProviderID).To(Equal(expectedProviderID))
				GinkgoWriter.Printf("Static node provider ID: %s\n", node.Spec.ProviderID)
			})

			It("should have external and internal IP addresses", func() {
				node, err := suite.K8sClient.WaitForNodeProviderID(ctx, staticNode.Name)
				Expect(err).NotTo(HaveOccurred())

				var externalIPv4 string
				var internalIPv4 string

				for _, addr := range node.Status.Addresses {
					switch addr.Type {
					case corev1.NodeExternalIP:
						externalIPv4 = addr.Address
					case corev1.NodeInternalIP:
						internalIPv4 = addr.Address
					}
				}

				Expect(externalIPv4).NotTo(BeEmpty(), "Static node should have external IPv4")
				Expect(externalIPv4).To(Equal(instanceIP), "External IP should match instance IP")
				GinkgoWriter.Printf("Static node addresses - External: %s, Internal: %s\n",
					externalIPv4, internalIPv4)
			})
		})
	})

	Describe("Cloud Controller Manager", func() {
		It("should approve node CSRs", func() {
			approvedCSRs, err := suite.K8sClient.GetApprovedCSRs(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(approvedCSRs)).To(BeNumerically(">=", int(suite.Config.NodepoolSize)),
				"Expected at least %d CSRs to be approved", suite.Config.NodepoolSize)

			GinkgoWriter.Printf("Approved CSRs: %d\n", len(approvedCSRs))

			for _, csr := range approvedCSRs {
				foundApproved := false
				for _, condition := range csr.Status.Conditions {
					if condition.Reason == "ExoscaleCloudControllerApproved" {
						foundApproved = true
						break
					}
				}
				Expect(foundApproved).To(BeTrue(), "CSR %s should be approved by CCM", csr.Name)
			}
		})

		It("should reject CSRs with invalid IPv4 addresses", func() {
			instance := suite.InstanceMgr.GetInstance()
			Expect(instance).NotTo(BeNil())
			instanceName := instance.Name

			csrName := "csr-invalid-ipv4"

			// Generate a kubeconfig with node credentials
			nodeKubeconfig, err := suite.ClusterMgr.GetKubeconfig(ctx, fmt.Sprintf("system:node:%s", strings.ToLower(instanceName)), []string{"system:authenticated", "system:nodes"})
			Expect(err).NotTo(HaveOccurred())

			GinkgoWriter.Printf("Creating CSR with invalid IPv4: %s (for instance: %s)\n", csrName, instanceName)
			_, err = suite.K8sClient.CreateInvalidCSR(ctx, csrName, instanceName, []string{"192.0.2.42"}, nodeKubeconfig)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				_ = suite.K8sClient.DeleteCSR(context.Background(), csrName)
			}()

			logCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			found, err := suite.CCMMgr.WaitForLog(fmt.Sprintf("sks-agent: CSR %s Node IP addresses don't match", csrName), logCtx)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue(), "CCM should reject CSR with invalid IPv4")

			csr, err := suite.K8sClient.GetCSR(ctx, csrName)
			Expect(err).NotTo(HaveOccurred())
			Expect(suite.K8sClient.IsCSRApproved(csr)).To(BeFalse(), "CSR should not be approved")

			GinkgoWriter.Printf("CSR %s correctly rejected\n", csrName)
		})

		It("should reject CSRs with invalid IPv6 addresses", func() {
			instance := suite.InstanceMgr.GetInstance()
			Expect(instance).NotTo(BeNil())
			instanceName := instance.Name

			csrName := "csr-invalid-ipv6"

			// Generate a kubeconfig with node credentials
			nodeKubeconfig, err := suite.ClusterMgr.GetKubeconfig(ctx, fmt.Sprintf("system:node:%s", strings.ToLower(instanceName)), []string{"system:authenticated", "system:nodes"})
			Expect(err).NotTo(HaveOccurred())

			GinkgoWriter.Printf("Creating CSR with invalid IPv6: %s (for instance: %s)\n", csrName, instanceName)
			_, err = suite.K8sClient.CreateInvalidCSR(ctx, csrName, instanceName, []string{"2001:db8::dead:beef"}, nodeKubeconfig)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				_ = suite.K8sClient.DeleteCSR(context.Background(), csrName)
			}()

			logCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			found, err := suite.CCMMgr.WaitForLog(fmt.Sprintf("sks-agent: CSR %s Node IP addresses don't match", csrName), logCtx)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue(), "CCM should reject CSR with invalid IPv6")

			csr, err := suite.K8sClient.GetCSR(ctx, csrName)
			Expect(err).NotTo(HaveOccurred())
			Expect(suite.K8sClient.IsCSRApproved(csr)).To(BeFalse(), "CSR should not be approved")

			GinkgoWriter.Printf("CSR %s correctly rejected\n", csrName)
		})

		It("should initialize static node with cloud provider", func() {
			staticNodeName := fmt.Sprintf("%s-static", suite.Config.TestID)

			logCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()

			found, err := suite.CCMMgr.WaitForLog(fmt.Sprintf("Successfully initialized node %s with cloud provider", staticNodeName), logCtx)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue(), "CCM should log node initialization")

			GinkgoWriter.Printf("Static node %s initialized by CCM\n", staticNodeName)
		})

		It("should detect invalid credentials when refreshed", func() {
			GinkgoWriter.Println("Switching to invalid credentials...")
			err := suite.CCMMgr.WriteCredentialsFile("invalid-key", "invalid-secret", "invalid")
			Expect(err).NotTo(HaveOccurred())

			logCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
			defer cancel()

			found, err := suite.CCMMgr.WaitForLog("failed to switch client zone", logCtx)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue(), "CCM should detect invalid credentials")

			GinkgoWriter.Println("CCM detected invalid credentials")
		})

		It("should refresh to valid credentials successfully", func() {
			GinkgoWriter.Println("Switching back to valid credentials...")
			err := suite.CCMMgr.WriteCredentialsFile(suite.Config.APIKey, suite.Config.APISecret, "valid")
			Expect(err).NotTo(HaveOccurred())

			logCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
			defer cancel()

			found, err := suite.CCMMgr.WaitForLog("Exoscale API credentials refreshed, now using valid", logCtx)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue(), "CCM should successfully refresh to valid credentials")

			GinkgoWriter.Println("CCM refreshed to valid credentials")
		})
	})

	Describe("Kubernetes Nodes", func() {
		It("should have all nodes ready with provider IDs", func() {
			expectedNodeCount := int(suite.Config.NodepoolSize) + 1

			err := suite.K8sClient.WaitForNodesReady(ctx, expectedNodeCount)
			Expect(err).NotTo(HaveOccurred(), "Failed to wait for %d nodes to be ready", expectedNodeCount)

			err = suite.K8sClient.WaitForNodesWithProviderID(ctx, 1)
			Expect(err).NotTo(HaveOccurred(), "Failed to wait for static node to have provider ID")

			nodes, err := suite.K8sClient.GetReadyNodes(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(nodes).To(HaveLen(expectedNodeCount),
				"Expected %d ready nodes (nodepool + static instance)", expectedNodeCount)

			staticNodeName := fmt.Sprintf("%s-static", suite.Config.TestID)

			for _, node := range nodes {
				if node.Name == staticNodeName {
					Expect(node.Spec.ProviderID).NotTo(BeEmpty())
					Expect(node.Spec.ProviderID).To(ContainSubstring("exoscale://"))

					Expect(node.Labels).To(HaveKey("topology.kubernetes.io/region"))
					Expect(node.Labels["topology.kubernetes.io/region"]).To(Equal(suite.Config.Zone))
				}

				Expect(node.Status.Addresses).NotTo(BeEmpty())

				hasInternalIP := false
				for _, addr := range node.Status.Addresses {
					if addr.Type == corev1.NodeInternalIP {
						hasInternalIP = true
						break
					}
				}
				Expect(hasInternalIP).To(BeTrue(), "Node should have internal IP")
			}

			nodesWithProviderID, err := suite.K8sClient.CountNodesByProviderID(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(nodesWithProviderID).To(Equal(1),
				"Static node should have exoscale:// provider ID")
		})
	})

	Describe("Static Instance Cleanup", func() {
		It("should delete the static instance", func() {
			GinkgoWriter.Println("Deleting static instance...")
			err := suite.InstanceMgr.DeleteInstance(ctx)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("Static instance deleted: %s\n", suite.InstanceMgr.GetInstanceID())
		})

		It("should wait for static node to be removed from cluster", func() {
			staticNodeName := fmt.Sprintf("%s-static", suite.Config.TestID)

			timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()

			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-timeoutCtx.Done():
					Fail("Timeout waiting for static node to be removed")
				case <-ticker.C:
					nodes, err := suite.K8sClient.GetNodes(timeoutCtx)
					if err != nil {
						continue
					}

					nodeExists := false
					for _, node := range nodes {
						if node.Name == staticNodeName {
							nodeExists = true
							break
						}
					}

					if !nodeExists {
						GinkgoWriter.Printf("Static node %s removed from cluster\n", staticNodeName)
						return
					}
				}
			}
		})

		It("should have only nodepool nodes remaining", func() {
			expectedNodeCount := int(suite.Config.NodepoolSize)

			err := suite.K8sClient.WaitForNodesReady(ctx, expectedNodeCount)
			Expect(err).NotTo(HaveOccurred())

			nodes, err := suite.K8sClient.GetReadyNodes(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(nodes).To(HaveLen(expectedNodeCount),
				"Expected %d ready nodes (nodepool only)", expectedNodeCount)

			GinkgoWriter.Printf("Cluster has %d nodepool nodes remaining\n", len(nodes))
		})
	})

	Describe("Network Load Balancers", func() {
		Describe("Simple LoadBalancer Service", func() {
			var serviceName string
			var podName string

			BeforeEach(func() {
				podName = "nginx-hello-external"
				serviceName = "hello-external-lb"
			})

			AfterEach(func() {
				cleanupCtx := context.Background()
				_ = suite.K8sClient.Clientset().CoreV1().Services("default").Delete(cleanupCtx, serviceName, metav1.DeleteOptions{})
				_ = suite.K8sClient.Clientset().CoreV1().Pods("default").Delete(cleanupCtx, podName, metav1.DeleteOptions{})
			})

			It("should create an NLB with external IP", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: "default",
						Labels: map[string]string{
							"app": "hello-external",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "nginx",
								Image: "nginx:alpine",
								Ports: []corev1.ContainerPort{
									{ContainerPort: 80},
								},
							},
						},
					},
				}

				_, err := suite.K8sClient.Clientset().CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      serviceName,
						Namespace: "default",
						Annotations: map[string]string{
							"service.beta.kubernetes.io/exoscale-loadbalancer-name": fmt.Sprintf("%s-hello", suite.Config.TestID),
						},
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
						Selector: map[string]string{
							"app": "hello-external",
						},
						Ports: []corev1.ServicePort{
							{
								Port:       80,
								TargetPort: intstr.FromInt(80),
								Protocol:   corev1.ProtocolTCP,
							},
						},
					},
				}

				_, err = suite.K8sClient.Clientset().CoreV1().Services("default").Create(ctx, service, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				svc, err := suite.K8sClient.WaitForServiceLoadBalancer(ctx, "default", serviceName)
				Expect(err).NotTo(HaveOccurred())

				Expect(svc.Status.LoadBalancer.Ingress).NotTo(BeEmpty())
				externalIP := svc.Status.LoadBalancer.Ingress[0].IP
				Expect(externalIP).NotTo(BeEmpty(), "LoadBalancer should have external IP")

				GinkgoWriter.Printf("NLB created with external IP: %s\n", externalIP)
			})
		})

		Describe("NGINX Ingress Controller", Ordered, func() {
			const ingressManifestPath = "manifests/ingress-nginx.yaml"
			const helloManifestPath = "manifests/hello-ingress.yaml"

			BeforeAll(func() {
				GinkgoWriter.Println("Deploying NGINX Ingress Controller...")
				err := suite.K8sClient.ApplyManifestWithReplacements(ctx, kubeconfigPath, ingressManifestPath, suite.GetManifestReplacements())
				Expect(err).NotTo(HaveOccurred())

				GinkgoWriter.Println("Waiting for NGINX Ingress Controller deployment to be available...")
				err = suite.K8sClient.WaitForDeploymentAvailable(ctx, "ingress-nginx", "ingress-nginx-controller", 10*time.Minute)
				Expect(err).NotTo(HaveOccurred())

				GinkgoWriter.Println("Waiting for LoadBalancer service to get external IP...")
				svc, err := suite.K8sClient.WaitForServiceLoadBalancer(ctx, "ingress-nginx", "ingress-nginx-controller")
				Expect(err).NotTo(HaveOccurred())
				Expect(svc.Status.LoadBalancer.Ingress).NotTo(BeEmpty())

				externalIP := svc.Status.LoadBalancer.Ingress[0].IP
				GinkgoWriter.Printf("NGINX Ingress Controller external IP: %s\n", externalIP)
			})

			AfterAll(func() {
				_ = suite.K8sClient.DeleteManifest(context.Background(), kubeconfigPath, helloManifestPath)
				_ = suite.K8sClient.DeleteManifest(context.Background(), kubeconfigPath, ingressManifestPath)
			})

			It("should have an external IP", func() {
				svc, err := suite.K8sClient.Clientset().CoreV1().Services("ingress-nginx").Get(ctx, "ingress-nginx-controller", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(svc.Status.LoadBalancer.Ingress).NotTo(BeEmpty())
				Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))

				externalIP := svc.Status.LoadBalancer.Ingress[0].IP
				Expect(externalIP).NotTo(BeEmpty())
			})

			It("should route traffic to hello app via ingress", func() {
				defer func() {
					GinkgoWriter.Println("Cleaning up hello-ingress application...")
					_ = suite.K8sClient.DeleteManifest(context.Background(), kubeconfigPath, helloManifestPath)
				}()

				GinkgoWriter.Println("Deploying hello-ingress application...")
				err := suite.K8sClient.ApplyManifestWithReplacements(ctx, kubeconfigPath, helloManifestPath, suite.GetManifestReplacements())
				Expect(err).NotTo(HaveOccurred())

				GinkgoWriter.Println("Waiting for hello-ingress deployment to be available...")
				err = suite.K8sClient.WaitForDeploymentAvailable(ctx, "default", "hello-ingress", 10*time.Minute)
				Expect(err).NotTo(HaveOccurred())

				GinkgoWriter.Println("Checking hello-ingress service...")
				helloSvc, err := suite.K8sClient.Clientset().CoreV1().Services("default").Get(ctx, "hello-ingress", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				GinkgoWriter.Printf("Service hello-ingress ClusterIP: %s\n", helloSvc.Spec.ClusterIP)

				GinkgoWriter.Println("Checking hello-ingress pod status...")
				pods, err := suite.K8sClient.Clientset().CoreV1().Pods("default").List(ctx, metav1.ListOptions{
					LabelSelector: "app.kubernetes.io/name=hello-ingress",
				})
				Expect(err).NotTo(HaveOccurred())
				for _, pod := range pods.Items {
					GinkgoWriter.Printf("Pod %s: Phase=%s, Ready=%v\n", pod.Name, pod.Status.Phase, isPodReady(&pod))
				}

				GinkgoWriter.Println("Checking ingress resource status...")
				ingress, err := suite.K8sClient.Clientset().NetworkingV1().Ingresses("default").Get(ctx, "hello-ingress", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				GinkgoWriter.Printf("Ingress status: %+v\n", ingress.Status)

				svc, err := suite.K8sClient.Clientset().CoreV1().Services("ingress-nginx").Get(ctx, "ingress-nginx-controller", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(svc.Status.LoadBalancer.Ingress).NotTo(BeEmpty())

				nlbIP := svc.Status.LoadBalancer.Ingress[0].IP
				GinkgoWriter.Printf("Testing HTTP endpoint: http://%s\n", nlbIP)

				timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
				defer cancel()

				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()

				attemptCount := 0
				var lastErr error

				for {
					select {
					case <-timeoutCtx.Done():
						GinkgoWriter.Printf("Last error after %d attempts: %v\n", attemptCount, lastErr)
						Fail(fmt.Sprintf("Timeout waiting for Ingress routing to be ready (last error: %v)", lastErr))
					case <-ticker.C:
						attemptCount++
						lastErr = suite.K8sClient.TestHTTPEndpoint(timeoutCtx, fmt.Sprintf("http://%s", nlbIP))
						if lastErr == nil {
							GinkgoWriter.Printf("HTTP endpoint is accessible after %d attempts\n", attemptCount)
							return
						}
						GinkgoWriter.Printf("Attempt %d failed: %v\n", attemptCount, lastErr)
					}
				}
			})
		})

		Describe("UDP Echo Service with External NLB", func() {
			const manifestPath = "manifests/udp-echo.yaml"
			var nlbID exoscale.UUID

			BeforeEach(func() {
				nlbName := fmt.Sprintf("%s-udp-nlb", suite.Config.TestID)
				GinkgoWriter.Printf("Creating external NLB: %s\n", nlbName)

				createNLBReq := exoscale.CreateLoadBalancerRequest{
					Name: nlbName,
				}

				nlbOp, err := suite.Client.CreateLoadBalancer(ctx, createNLBReq)
				Expect(err).NotTo(HaveOccurred())

				opResult, err := suite.Client.Wait(ctx, nlbOp, exoscale.OperationStateSuccess)
				Expect(err).NotTo(HaveOccurred())

				nlbID = opResult.Reference.ID
				GinkgoWriter.Printf("Created NLB with ID: %s\n", nlbID)
			})

			AfterEach(func() {
				cleanupCtx := context.Background()
				_ = suite.K8sClient.DeleteManifest(cleanupCtx, kubeconfigPath, manifestPath)

				GinkgoWriter.Println("Deleting external NLB...")
				deleteOp, err := suite.Client.DeleteLoadBalancer(cleanupCtx, nlbID)
				if err == nil {
					_, _ = suite.Client.Wait(cleanupCtx, deleteOp, exoscale.OperationStateSuccess)
				}
			})

			It("should create UDP service with external NLB", func() {
				replacements := suite.GetManifestReplacements()
				replacements["${exoscale_nlb_id}"] = nlbID.String()

				GinkgoWriter.Println("Applying UDP echo application manifest...")
				err := suite.K8sClient.ApplyManifestWithReplacements(ctx, kubeconfigPath, manifestPath, replacements)
				Expect(err).NotTo(HaveOccurred())

				GinkgoWriter.Println("Waiting for udp-echo deployment to be available...")
				err = suite.K8sClient.WaitForDeploymentAvailable(ctx, "default", "udp-echo", 10*time.Minute)
				Expect(err).NotTo(HaveOccurred())

				GinkgoWriter.Println("Waiting for UDP echo LoadBalancer service to get external IP...")
				svc, err := suite.K8sClient.WaitForServiceLoadBalancer(ctx, "default", "udp-echo-service")
				Expect(err).NotTo(HaveOccurred())
				Expect(svc.Status.LoadBalancer.Ingress).NotTo(BeEmpty())

				externalIP := svc.Status.LoadBalancer.Ingress[0].IP
				GinkgoWriter.Printf("UDP echo service external IP: %s\n", externalIP)

				Expect(externalIP).NotTo(BeEmpty())
				Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
				Expect(svc.Spec.Ports[0].Protocol).To(Equal(corev1.ProtocolUDP))
			})
		})
	})

	Describe("Nodepool Scaling", func() {
		var initialNodeCount int

		BeforeEach(func() {
			expectedNodeCount := int(suite.Config.NodepoolSize)
			err := suite.K8sClient.WaitForNodesReady(ctx, expectedNodeCount)
			Expect(err).NotTo(HaveOccurred())

			nodes, err := suite.K8sClient.GetReadyNodes(ctx)
			Expect(err).NotTo(HaveOccurred())
			initialNodeCount = len(nodes)

			GinkgoWriter.Printf("Initial ready nodes: %d\n", initialNodeCount)
		})

		Describe("Scale Up", func() {
			var newSize int64
			var scaleUpDone bool

			BeforeEach(func() {
				if scaleUpDone {
					return
				}

				newSize = int64(initialNodeCount + 1)
				GinkgoWriter.Printf("Scaling node pool from %d to %d\n", initialNodeCount, newSize)

				err := suite.NodepoolMgr.ResizeNodepool(ctx, newSize)
				Expect(err).NotTo(HaveOccurred())
				scaleUpDone = true
			})

			It("should increase node count", func() {
				err := suite.K8sClient.WaitForNodesReady(ctx, int(newSize))
				Expect(err).NotTo(HaveOccurred())

				finalNodes, err := suite.K8sClient.GetReadyNodes(ctx)
				Expect(err).NotTo(HaveOccurred())

				Expect(finalNodes).To(HaveLen(int(newSize)),
					"Node count should be %d after scaling up", newSize)

				GinkgoWriter.Printf("Successfully scaled up to %d nodes\n", len(finalNodes))
			})

			It("should approve CSRs for new nodes", func() {
				nodes, err := suite.K8sClient.GetReadyNodes(ctx)
				Expect(err).NotTo(HaveOccurred())
				nodeCount := len(nodes)

				timeoutCtx, cancel := context.WithTimeout(ctx, suite.Config.Timeouts.CSRApproval)
				defer cancel()

				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()

				var approvedCSRs []certificatesv1.CertificateSigningRequest
				for {
					select {
					case <-timeoutCtx.Done():
						Fail("Timeout waiting for CSRs to be approved")
					case <-ticker.C:
						approvedCSRs, err = suite.K8sClient.GetApprovedCSRs(timeoutCtx)
						if err != nil {
							continue
						}

						if len(approvedCSRs) >= nodeCount {
							for _, csr := range approvedCSRs {
								foundApproved := false
								for _, condition := range csr.Status.Conditions {
									if condition.Reason == "ExoscaleCloudControllerApproved" {
										foundApproved = true
										break
									}
								}
								Expect(foundApproved).To(BeTrue(), "CSR %s should be approved by CCM", csr.Name)
							}
							GinkgoWriter.Printf("All %d CSRs properly approved after scale up\n", len(approvedCSRs))
							return
						}
					}
				}
			})

			It("should have proper metadata on all nodes", func() {
				nodes, err := suite.K8sClient.GetReadyNodes(ctx)
				Expect(err).NotTo(HaveOccurred())

				GinkgoWriter.Printf("All %d nodes have proper metadata after scale up\n", len(nodes))
			})

			It("should maintain LoadBalancer services after scale up", func() {
				services, err := suite.K8sClient.GetServices(ctx, "default")
				Expect(err).NotTo(HaveOccurred())

				var lbServices []corev1.Service
				for _, svc := range services {
					if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
						lbServices = append(lbServices, svc)
					}
				}

				GinkgoWriter.Printf("Found %d LoadBalancer services, verifying they still work after scale up\n", len(lbServices))

				for _, svc := range lbServices {
					Expect(svc.Status.LoadBalancer.Ingress).NotTo(BeEmpty(),
						"Service %s should have external IP after scale up", svc.Name)
				}
			})
		})

		Describe("Scale Down", func() {
			It("should decrease node count", func() {
				currentNodes, err := suite.K8sClient.GetReadyNodes(ctx)
				Expect(err).NotTo(HaveOccurred())
				currentNodepoolSize := len(currentNodes)

				if currentNodepoolSize <= 1 {
					Skip("Cannot scale down from 1 nodepool node")
				}

				newNodepoolSize := int64(currentNodepoolSize - 1)
				GinkgoWriter.Printf("Scaling node pool from %d to %d\n",
					currentNodepoolSize, newNodepoolSize)

				err = suite.NodepoolMgr.ResizeNodepool(ctx, newNodepoolSize)
				Expect(err).NotTo(HaveOccurred())

				eventualCtx, cancel := context.WithTimeout(ctx, suite.Config.Timeouts.NodeDeletion)
				defer cancel()

				ticker := time.NewTicker(10 * time.Second)
				defer ticker.Stop()

				for {
					select {
					case <-eventualCtx.Done():
						Fail("Timeout waiting for node count to decrease")
					case <-ticker.C:
						nodes, err := suite.K8sClient.GetNodes(eventualCtx)
						if err == nil && len(nodes) <= int(newNodepoolSize) {
							GinkgoWriter.Printf("Successfully scaled down to %d nodes\n", len(nodes))
							Expect(len(nodes)).To(BeNumerically("<=", int(newNodepoolSize)))
							return
						}
					}
				}
			})
		})
	})

	Describe("Test Finalization", func() {
		It("should dump remaining CCM logs for debugging", func() {
			GinkgoWriter.Println("\n=== CCM Log Summary ===")
			logs := suite.CCMMgr.GetLogs()

			if len(logs) == 0 {
				GinkgoWriter.Println("No additional CCM logs to display")
				return
			}

			GinkgoWriter.Printf("Total CCM log lines captured: %d\n", len(logs))

			recentLogs := logs
			if len(logs) > 50 {
				recentLogs = logs[len(logs)-50:]
				GinkgoWriter.Printf("Showing last 50 lines (of %d total):\n", len(logs))
			}

			GinkgoWriter.Println("--- Recent CCM Logs ---")
			for _, line := range recentLogs {
				GinkgoWriter.Println(line)
			}
			GinkgoWriter.Println("=== End CCM Log Summary ===\n")
		})
	})
})
