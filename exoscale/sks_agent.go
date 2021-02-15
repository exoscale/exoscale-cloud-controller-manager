package exoscale

import (
	"context"
	"fmt"
)

const sksAgentNodeCSRValidation = "node-csr-validation"

// sksAgentRunner represents an SKS agent runner interface.
type sksAgentRunner interface {
	// run represents the runner execution loop, which will be running in a
	// goroutine during cloud provider startup. The runner loop is expected
	// to watch the provided context for cancellation, and shut down if
	// signaled by ctx.Done().
	run(context.Context)
}

func (p *cloudProvider) runSKSAgent(runners []string) error {
	for _, r := range runners {
		debugf("sks-agent: starting %s runner", r)

		switch r {
		case sksAgentNodeCSRValidation:
			var runner sksAgentRunner = &sksAgentRunnerNodeCSRValidation{p: p}
			go runner.run(p.ctx)

		default:
			return fmt.Errorf("unsupported runner %q", r)
		}
	}

	return nil
}
