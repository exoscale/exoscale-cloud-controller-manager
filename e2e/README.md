# Exoscale CCM E2E Tests

End-to-end tests for the Exoscale Cloud Controller Manager using Ginkgo framework.

## Prerequisites

- Exoscale API credentials (key and secret)
- Go 1.23+ with Ginkgo CLI installed

## Installation

Install the Ginkgo CLI:

```bash
go install github.com/onsi/ginkgo/v2/ginkgo@latest
```

## Running Tests

### Pretty Output (Recommended) 🎨

Run with Ginkgo CLI for beautiful, colored output:

```bash
# Run all tests with verbose output
~/go/bin/ginkgo -v

# Run with progress indicators
~/go/bin/ginkgo -v --progress

# Run specific tests by filtering
~/go/bin/ginkgo -v --focus="Network Load Balancers"

# Show what tests will run without executing them
~/go/bin/ginkgo --dry-run -v
```

### Standard Go Test

```bash
# Basic run
go test -v

# With timeout
go test -v -timeout 30m

# Skip cleanup for debugging
E2E_SKIP_CLEANUP=1 go test -v
```

## Environment Variables

| Variable              | Required |   Default  |
|-----------------------|----------|------------|
| `EXOSCALE_API_KEY`    | Yes      | -          |
| `EXOSCALE_API_SECRET` | Yes      | -          |
| `EXOSCALE_ZONE`       | No       | `ch-gva-2` |
| `KUBERNETES_VERSION`  | No       | latest     |
| `E2E_SKIP_CLEANUP`    | No       | -          |

## Test Organization

The test suite is organized hierarchically:

```
Exoscale Cloud Controller Manager
├── Infrastructure Setup
│   ├── SKS cluster creation
│   ├── Nodepool creation
│   ├── Static instance creation
│   └── Kubernetes client initialization
├── Kubernetes State
│   ├── Nodes availability
│   └── Node CSRs
├── Static Instance
│   ├── Running state
│   ├── Public IP
│   ├── Bootstrap token
│   └── Cluster join
│       ├── Provider ID
│       └── IP addresses
├── Cloud Controller Manager
│   ├── CSR approval for valid nodes
│   ├── CSR rejection for invalid IPv4 addresses
│   ├── CSR rejection for invalid IPv6 addresses
│   ├── Node initialization logging
│   ├── Invalid credentials detection
│   ├── Credentials refresh (valid)
│   └── Finalization (dump logs)
├── Kubernetes Nodes
│   └── Provider IDs and metadata
├── Network Load Balancers
│   ├── Simple LoadBalancer Service
│   ├── NGINX Ingress Controller
│   ├── Ingress with Hello App
│   └── UDP Echo Service with External NLB
└── Nodepool Scaling
    ├── Scale Up
    │   ├── Node count increase
    │   ├── CSR approval for new nodes
    │   ├── Node metadata
    │   └── LoadBalancer maintenance
    └── Scale Down
        └── Node count decrease
```

## Ginkgo Features

### Focus Tests

Run specific tests using labels or regex:

```bash
# Run only scaling tests
~/go/bin/ginkgo -v --focus="Nodepool Scaling"

# Run only NLB tests
~/go/bin/ginkgo -v --focus="Network Load Balancers"

# Skip certain tests
~/go/bin/ginkgo -v --skip="Scale Down"

# Run only CCM tests
~/go/bin/ginkgo -v --focus="Cloud Controller Manager"
```

### Generate Reports

```bash
# Generate JUnit XML report (for CI)
~/go/bin/ginkgo -v --junit-report=junit.xml

# Generate JSON report
~/go/bin/ginkgo -v --json-report=report.json
```

## Debugging

To keep resources for manual inspection:

```bash
E2E_SKIP_CLEANUP=1 ~/go/bin/ginkgo -v --focus="Static Instance"
```

This will leave the cluster, nodes, and resources running so you can inspect them.
