package exoscale

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"testing"
	"time"

	egoscale "github.com/exoscale/egoscale/v2"
	"github.com/gofrs/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/suite"
)

var (
	testZone        = "ch-gva-2"
	testAPIEndpoint = fmt.Sprintf("https://api-%s.exoscale.com/v2.alpha", testZone)
	testSeededRand  = rand.New(rand.NewSource(time.Now().UnixNano()))
)

type exoscaleCCMTestSuite struct {
	suite.Suite

	provider *cloudProvider
}

func (ts *exoscaleCCMTestSuite) SetupTest() {
	httpmock.Activate()

	exo, err := egoscale.NewClient("x", "x", egoscale.ClientOptWithPollInterval(10*time.Millisecond))
	if err != nil {
		ts.T().Fatal(err)
	}

	ts.provider = &cloudProvider{
		client: &exoscaleClient{
			exo:     exo,
			RWMutex: &sync.RWMutex{},
		},
		zone: testZone,
	}
	ts.provider.instances = &instances{p: ts.provider}
	ts.provider.loadBalancer = &loadBalancer{p: ts.provider}
	ts.provider.zones = &zones{p: ts.provider}
}

func (ts *exoscaleCCMTestSuite) TearDownTest() {
	ts.provider = nil

	httpmock.DeactivateAndReset()
}

func (ts *exoscaleCCMTestSuite) mockAPIRequest(method, url string, body interface{}) {
	httpmock.RegisterResponder(
		method,
		testAPIEndpoint+url,
		httpmock.NewJsonResponderOrPanic(http.StatusOK, body),
	)
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
