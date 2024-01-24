package exoscale

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	k8scertv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	certificatesv1 "k8s.io/client-go/kubernetes/typed/certificates/v1"
	fakecertificatesv1 "k8s.io/client-go/kubernetes/typed/certificates/v1/fake"
	"k8s.io/utils/ptr"

	egoscale "github.com/exoscale/egoscale/v2"
)

func (ts *exoscaleCCMTestSuite) generateK8sCSR(nodeName string, nodeIPAddresses []string) []byte {

	// k8s node name are lowercase only
	k8sNodeName := strings.ToLower(nodeName)
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	ts.Require().NoError(err, "failed to generate RSA private key")

	ipAddresses := make([]net.IP, 0, len(nodeIPAddresses))
	for _, ip := range nodeIPAddresses {
		ipAddresses = append(ipAddresses, net.ParseIP(ip))
	}

	csrBytes, err := x509.CreateCertificateRequest(
		rand.Reader,
		&x509.CertificateRequest{
			SignatureAlgorithm: x509.SHA512WithRSA,
			Subject: pkix.Name{
				Organization: []string{"system:nodes"},
				CommonName:   "system:nodes:" + k8sNodeName,
			},
			DNSNames:    []string{k8sNodeName},
			IPAddresses: ipAddresses,
		},
		privateKey,
	)
	ts.Require().NoError(err, "failed to create CSR")

	csrBuf := bytes.NewBuffer(nil)
	ts.Require().NoError(pem.Encode(csrBuf, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes}))

	return csrBuf.Bytes()
}

type certificateSigningRequestMockWatcher struct {
	eventChan <-chan watch.Event
}

func (c certificateSigningRequestMockWatcher) ResultChan() <-chan watch.Event {
	return c.eventChan
}

func (c certificateSigningRequestMockWatcher) Stop() {
}

type certificateSigningRequestMock struct {
	fakecertificatesv1.FakeCertificateSigningRequests

	eventChan           <-chan watch.Event
	csrApprovalTestFunc func(certificateSigningRequestName string, certificateSigningRequest *k8scertv1.CertificateSigningRequest)
}

func (m *certificateSigningRequestMock) UpdateApproval(ctx context.Context, certificateSigningRequestName string, certificateSigningRequest *k8scertv1.CertificateSigningRequest, opts metav1.UpdateOptions) (result *k8scertv1.CertificateSigningRequest, err error) {
	m.csrApprovalTestFunc(certificateSigningRequestName, certificateSigningRequest)
	return m.FakeCertificateSigningRequests.UpdateApproval(ctx, certificateSigningRequestName, certificateSigningRequest, opts)
}

func (m *certificateSigningRequestMock) Watch(_ context.Context, _ metav1.ListOptions) (watch.Interface, error) {
	return &certificateSigningRequestMockWatcher{eventChan: m.eventChan}, nil
}

type certificatesV1Mock struct {
	*fakecertificatesv1.FakeCertificatesV1

	eventChan           <-chan watch.Event
	csrApprovalTestFunc func(certificateSigningRequestName string, certificateSigningRequest *k8scertv1.CertificateSigningRequest)
}

func (m *certificatesV1Mock) CertificateSigningRequests() certificatesv1.CertificateSigningRequestInterface {
	return &certificateSigningRequestMock{
		FakeCertificateSigningRequests: fakecertificatesv1.FakeCertificateSigningRequests{Fake: m.FakeCertificatesV1},
		eventChan:                      m.eventChan,
		csrApprovalTestFunc:            m.csrApprovalTestFunc,
	}
}

type k8sClientMock struct {
	*fakek8s.Clientset

	eventChan           <-chan watch.Event
	csrApprovalTestFunc func(certificateSigningRequestName string, certificateSigningRequest *k8scertv1.CertificateSigningRequest)
}

func (m *k8sClientMock) CertificatesV1() certificatesv1.CertificatesV1Interface {
	return &certificatesV1Mock{
		FakeCertificatesV1:  &fakecertificatesv1.FakeCertificatesV1{Fake: &m.Clientset.Fake},
		eventChan:           m.eventChan,
		csrApprovalTestFunc: m.csrApprovalTestFunc,
	}
}

func (ts *exoscaleCCMTestSuite) Test_sksAgentRunnerNodeCSRValidation_hasRequiredGroups() {
	type args struct {
		csr *k8scertv1.CertificateSigningRequest
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "missing required groups",
			args: args{
				csr: &k8scertv1.CertificateSigningRequest{
					Spec: k8scertv1.CertificateSigningRequestSpec{
						Groups: []string{},
					},
				},
			},
			want: false,
		},
		{
			name: "ok",
			args: args{
				csr: &k8scertv1.CertificateSigningRequest{
					Spec: k8scertv1.CertificateSigningRequestSpec{
						Groups: []string{"system:authenticated", "system:nodes"},
					},
				},
			},
			want: true,
		},
	}

	nodeCSRValidationRunner := &sksAgentRunnerNodeCSRValidation{p: ts.p}
	for _, tt := range tests {
		ts.T().Run(tt.name, func(t *testing.T) {
			if got := nodeCSRValidationRunner.hasRequiredGroups(tt.args.csr); got != tt.want {
				t.Errorf("hasRequiredGroups() = %v, want %v", got, tt.want)
			}
		})
	}
}

func (ts *exoscaleCCMTestSuite) Test_sksAgentRunnerNodeCSRValidation_parseCSR() {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	ts.Require().NoError(err)

	expected := &x509.CertificateRequest{
		SignatureAlgorithm: x509.SHA512WithRSA,
		Subject: pkix.Name{
			Organization: []string{"system:nodes"},
			CommonName:   "system:nodes:" + testInstanceName,
		},
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, expected, privateKey)
	ts.Require().NoError(err)

	csrBuf := bytes.NewBuffer(nil)
	ts.Require().NoError(pem.Encode(csrBuf, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes}))

	nodeCSRValidationRunner := &sksAgentRunnerNodeCSRValidation{p: ts.p}
	actual, err := nodeCSRValidationRunner.parseCSR(csrBuf.Bytes())
	ts.Require().NoError(err)
	ts.Require().Equal(expected.Subject.Organization, actual.Subject.Organization)
	ts.Require().Equal(expected.Subject.CommonName, actual.Subject.CommonName)
}

func (ts *exoscaleCCMTestSuite) Test_sksAgentRunnerNodeCSRValidation_run() {
	// Guarding the CSR approval result with a mutex is required as multiple goroutines
	// are involved, resulting in a data race during tests.
	type csrValidationResult struct {
		sync.RWMutex
		approved bool
	}

	var (
		csrName = "csr-" + strings.ToLower(ts.randomString(5))
		result  = csrValidationResult{RWMutex: sync.RWMutex{}}

		k8sEventChan = make(chan watch.Event)
		k8sCSR       = &k8scertv1.CertificateSigningRequest{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "certificates.k8s.io/v1",
				Kind:       "CertificateSigningRequest",
			},
			ObjectMeta: metav1.ObjectMeta{Name: csrName},
			Spec: k8scertv1.CertificateSigningRequestSpec{
				Request:    ts.generateK8sCSR(testInstanceName, []string{testInstancePublicIPv4, testInstancePublicIPv6}),
				SignerName: "kubernetes.io/kubelet-serving",
				Groups:     []string{"system:authenticated", "system:nodes"},
			},
		}
	)

	ts.p.kclient = &k8sClientMock{
		eventChan: k8sEventChan,
		Clientset: fakek8s.NewSimpleClientset(k8sCSR),
		csrApprovalTestFunc: func(name string, csr *k8scertv1.CertificateSigningRequest) {
			ts.Require().Equal(csrName, name)
			ts.Require().Equal(corev1.ConditionTrue, csr.Status.Conditions[0].Status)
			ts.Require().Equal(sksAgentNodeCSRValidationApprovalReason, csr.Status.Conditions[0].Reason)
			ts.Require().Equal(sksAgentNodeCSRValidationApprovalMessage, csr.Status.Conditions[0].Message)

			result.Lock()
			defer result.Unlock()
			result.approved = true
		},
	}

	ts.p.client.(*exoscaleClientMock).
		On("ListInstances", mock.Anything, ts.p.zone, mock.Anything).
		Return(
			[]*egoscale.Instance{{
				Name:            &testInstanceName,
				PublicIPAddress: &testInstancePublicIPv4P,
				IPv6Address:     &testInstancePublicIPv6P,
				IPv6Enabled:     ptr.To(true),
			}},
			nil,
		)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nodeCSRValidationRunner := &sksAgentRunnerNodeCSRValidation{p: ts.p}
	go nodeCSRValidationRunner.run(ctx)

	time.Sleep(1 * time.Second)
	k8sEventChan <- watch.Event{
		Type:   watch.Added,
		Object: k8sCSR,
	}

	ts.Require().Eventually(
		func() bool {
			result.RLock()
			defer result.RUnlock()
			return result.approved
		},
		3*time.Second,
		time.Second,
		"CSR has not been approved before timeout",
	)
}
