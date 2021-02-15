package exoscale

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	k8scertv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8swatch "k8s.io/apimachinery/pkg/watch"
)

// sksAgentNodeCSRValidationRequiredGroups describes the list of Kubernetes
// RBAC groups a Node must be member of in order to have its CSR validated.
var sksAgentNodeCSRValidationRequiredGroups = []string{
	"system:authenticated",
	"system:nodes",
}

// sksAgentRunnerNodeCSRValidation is a SKS agent runner performing automatic
// cluster Node CSR validation.
type sksAgentRunnerNodeCSRValidation struct {
	p *cloudProvider
}

func (r *sksAgentRunnerNodeCSRValidation) run(ctx context.Context) {
	// We set a relatively low watcher timeout to be able to break out of the
	// watch loop frequently in case we weren't able to validate a CSR for some
	// reason (e.g. the Exoscale client didn't have valid API credentials at
	// the time), this way we have a chance to do re-evaluate missed CSRs quickly
	// after.
	watchTimeoutSeconds := int64(120)

	for {
		watcher, err := r.p.kclient.
			CertificatesV1().
			CertificateSigningRequests().
			Watch(ctx, v1.ListOptions{
				Watch:          true,
				TimeoutSeconds: &watchTimeoutSeconds, // Default timeout: 20 minutes.
			})
		if err != nil {
			errorf("sks-agent: failed to list CSR resources: %s", err)
			time.Sleep(10 * time.Second) // Pause for a while before retrying, otherwise we'll spam error logs.
			continue
		}
		csrWatcher := k8swatch.Filter(watcher, func(in k8swatch.Event) (out k8swatch.Event, keep bool) {
			if in.Type != k8swatch.Added {
				return in, false
			}
			return in, true
		})

		debugf("sks-agent: watching for pending CSRs")

	watch:
		for {
			select {
			case <-ctx.Done():
				infof("sks-agent: context cancelled, terminating")
				return

			case event, ok := <-csrWatcher.ResultChan():
				if !ok {
					// Server timeout closed the watcher channel, loop again to re-create a new one.
					debugf("sks-agent: API server closed watcher channel")
					break watch
				}

				csr, ok := event.Object.DeepCopyObject().(*k8scertv1.CertificateSigningRequest)
				if !ok {
					errorf("sks-agent: expected event of type *CertificateSigningRequest, got %v",
						event.Object.GetObjectKind())
					continue
				}

				// The CSR has already been approved or denied.
				if len(csr.Status.Conditions) > 0 {
					continue
				}

				if !r.hasRequiredGroups(csr) {
					continue
				}

				debugf("sks-agent: checking pending CSR %s", csr.Name)

				parsedCSR, err := r.parseCSR(csr.Spec.Request)
				if err != nil {
					errorf("sks-agent: failed to parse CSR: %s", err)
					continue
				}

				if l := len(parsedCSR.DNSNames); l != 1 {
					errorf("sks-agent: expected 1 certificate Subject Alternate Name DNS Name value, got %d", l)
					continue
				}
				if l := len(parsedCSR.IPAddresses); l != 1 {
					errorf("sks-agent: expected 1 certificate Subject Alternate Name IP Address value, got %d", l)
					continue
				}

				instances, err := r.p.client.ListInstances(ctx, r.p.zone)
				if err != nil {
					errorf("sks-agent: failed to list Compute instances: %s", err)
					continue
				}

				for _, instance := range instances {
					if instance.Name == parsedCSR.DNSNames[0] {
						if instance.DefaultNic().IPAddress.Equal(parsedCSR.IPAddresses[0]) {
							ok = true
							break
						}

						errorf("sks-agent: CSR %q Node IP address doesn't match corresponding "+
							"Compute instance public interface IP address", csr.Name)
						continue
					}
				}
				if !ok {
					errorf("sks-agent: CSR %s doesn't match any Compute instance", csr.Name)
					continue
				}

				csr.Status.Conditions = append(csr.Status.Conditions, k8scertv1.CertificateSigningRequestCondition{
					Type:           k8scertv1.CertificateApproved,
					Status:         corev1.ConditionTrue,
					Reason:         "ExoscaleCloudControllerApproved",
					Message:        "This CSR was approved by the Exoscale Cloud Controller Manager",
					LastUpdateTime: metav1.Now(),
				})

				_, err = r.p.kclient.
					CertificatesV1().
					CertificateSigningRequests().
					UpdateApproval(ctx, csr.Name, csr, metav1.UpdateOptions{})
				if err != nil {
					errorf("sks-agent: failed to approve CSR %s: %s", err)
					continue
				}

				infof("sks-agent: CSR %s approved", csr.Name)
			}
		}
	}
}

func (r *sksAgentRunnerNodeCSRValidation) hasRequiredGroups(csr *k8scertv1.CertificateSigningRequest) bool {
	for _, expected := range sksAgentNodeCSRValidationRequiredGroups {
		var ok bool
		for _, actual := range csr.Spec.Groups {
			if expected == actual {
				ok = true
				break
			}
		}

		if !ok {
			return false
		}
	}

	return true
}

func (r *sksAgentRunnerNodeCSRValidation) parseCSR(pemData []byte) (*x509.CertificateRequest, error) {
	block, _ := pem.Decode(pemData)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, fmt.Errorf("PEM block type must be CERTIFICATE REQUEST")
	}

	return x509.ParseCertificateRequest(block.Bytes)
}
