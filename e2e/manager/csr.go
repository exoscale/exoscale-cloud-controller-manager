package manager

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net"
	"strings"

	certificatesv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (k *KubernetesClient) CreateInvalidCSR(ctx context.Context, name, nodeName string, invalidIPs []string, nodeKubeconfig []byte) (*certificatesv1.CertificateSigningRequest, error) {
	nodeClient, err := NewKubernetesClient(nodeKubeconfig, k.config)
	if err != nil {
		return nil, fmt.Errorf("failed to create node client: %w", err)
	}

	k8sNodeName := strings.ToLower(nodeName)

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	subject := pkix.Name{
		Organization: []string{"system:nodes"},
		CommonName:   fmt.Sprintf("system:node:%s", k8sNodeName),
	}

	var ipAddresses []net.IP
	for _, ipStr := range invalidIPs {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			return nil, fmt.Errorf("invalid IP address: %s", ipStr)
		}
		ipAddresses = append(ipAddresses, ip)
	}

	template := x509.CertificateRequest{
		Subject:            subject,
		SignatureAlgorithm: x509.ECDSAWithSHA256,
		DNSNames:           []string{k8sNodeName},
		IPAddresses:        ipAddresses,
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &template, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate request: %w", err)
	}

	csrPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrDER,
	})

	k8sCSR := &certificatesv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: certificatesv1.CertificateSigningRequestSpec{
			Request:    csrPEM,
			SignerName: "kubernetes.io/kubelet-serving",
			Usages: []certificatesv1.KeyUsage{
				certificatesv1.UsageDigitalSignature,
				certificatesv1.UsageKeyEncipherment,
				certificatesv1.UsageServerAuth,
			},
		},
	}

	return nodeClient.clientset.CertificatesV1().CertificateSigningRequests().Create(ctx, k8sCSR, metav1.CreateOptions{})
}

func (k *KubernetesClient) DeleteCSR(ctx context.Context, name string) error {
	return k.clientset.CertificatesV1().CertificateSigningRequests().Delete(ctx, name, metav1.DeleteOptions{})
}

func (k *KubernetesClient) GetCSR(ctx context.Context, name string) (*certificatesv1.CertificateSigningRequest, error) {
	return k.clientset.CertificatesV1().CertificateSigningRequests().Get(ctx, name, metav1.GetOptions{})
}

func (k *KubernetesClient) IsCSRApproved(csr *certificatesv1.CertificateSigningRequest) bool {
	for _, condition := range csr.Status.Conditions {
		if condition.Type == certificatesv1.CertificateApproved {
			return true
		}
	}
	return false
}
