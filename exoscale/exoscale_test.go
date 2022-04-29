package exoscale

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	testSeededRand = rand.New(rand.NewSource(time.Now().UnixNano()))
)

type exoscaleCCMTestSuite struct {
	suite.Suite

	p *cloudProvider
}

func (ts *exoscaleCCMTestSuite) SetupTest() {
	ts.p = &cloudProvider{
		cfg:     &testConfig_typical,
		ctx:     context.Background(),
		client:  new(exoscaleClientMock),
		kclient: fake.NewSimpleClientset(),
		zone:    testZone,
	}

	ts.p.instances = &instances{p: ts.p, cfg: &testConfig_typical.Instances}
	ts.p.loadBalancer = &loadBalancer{p: ts.p, cfg: &testConfig_typical.LoadBalancer}
	ts.p.zones = &zones{p: ts.p}
}

func (ts *exoscaleCCMTestSuite) TearDownTest() {
}

func (ts *exoscaleCCMTestSuite) randomID() string {
	id, err := uuid.NewV4()
	if err != nil {
		ts.T().Fatalf("unable to generate a new UUID: %s", err)
	}
	return id.String()
}

func (ts *exoscaleCCMTestSuite) randomStringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[testSeededRand.Intn(len(charset))]
	}
	return string(b)
}

func (ts *exoscaleCCMTestSuite) randomString(length int) string {
	const defaultCharset = "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	return ts.randomStringWithCharset(length, defaultCharset)
}

func TestSuiteExoscaleCCM(t *testing.T) {
	suite.Run(t, new(exoscaleCCMTestSuite))
}
