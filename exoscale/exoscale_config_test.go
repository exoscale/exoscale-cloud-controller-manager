package exoscale

import (
	"fmt"
	"os"
	"strings"
)

var (
	// Global
	testZone               = "ch-gva-2"
	testAPIKey             = new(exoscaleCCMTestSuite).randomString(10)
	testAPISecret          = new(exoscaleCCMTestSuite).randomString(10)
	testAPICredentialsFile = new(exoscaleCCMTestSuite).randomString(10)
	testAPIEndpoint        = "test"

	// Config
	testConfig_empty   = cloudConfig{}
	testConfig_typical = cloudConfig{
		Global: globalConfig{
			Zone:      testZone,
			APIKey:    testAPIKey,
			APISecret: testAPISecret,
		},
		Instances: instancesConfig{
			Overrides: []instancesOverrideConfig{{
				Name:     testInstanceOverrideRegexpName,
				External: true,
				Type:     testInstanceOverrideExternalType,
				Addresses: []instancesOverrideAddressConfig{
					{Type: "InternalIP", Address: testInstanceOverrideAddress_internal},
					{Type: "ExternalIP", Address: testInstanceOverrideAddress_external},
				},
				Region: testInstanceOverrideExternalRegion,
			}},
		},
		LoadBalancer: loadBalancerConfig{},
	}

	// YAML
	testConfigYAML_empty    = "---"
	testConfigYAML_disabled = `---
instances:
  disabled: true
loadBalancer:
  disabled: true
`
	testConfigYAML_credsFile = fmt.Sprintf(`---
global:
  apiCredentialsFile: "%s"
`, testAPICredentialsFile)
	testConfigYAML_typical = fmt.Sprintf(`---
global:
  zone: "%s"
  apiKey: "%s"
  apiSecret: "%s"
  apiEndpoint: "%s"
`, testZone, testAPIKey, testAPISecret, testAPIEndpoint)
)

func (ts *exoscaleCCMTestSuite) Test_readExoscaleConfig_empty() {
	os.Unsetenv("EXOSCALE_ZONE")
	os.Unsetenv("EXOSCALE_API_KEY")
	os.Unsetenv("EXOSCALE_API_SECRET")
	os.Unsetenv("EXOSCALE_API_ENVIRONMENT")

	cfg, err := readExoscaleConfig(strings.NewReader(testConfigYAML_empty))
	ts.Require().NoError(err)
	ts.Require().Equal("", cfg.Global.Zone)
	ts.Require().Equal("", cfg.Global.APIKey)
	ts.Require().Equal("", cfg.Global.APISecret)
	ts.Require().Equal("", cfg.Global.APICredentialsFile)
	ts.Require().Equal(false, cfg.Instances.Disabled)
	ts.Require().Equal(false, cfg.LoadBalancer.Disabled)
}

func (ts *exoscaleCCMTestSuite) Test_readExoscaleConfig_env_credsFile() {
	os.Setenv("EXOSCALE_API_CREDENTIALS_FILE", testAPICredentialsFile)
	defer func() {
		os.Unsetenv("EXOSCALE_API_CREDENTIALS_FILE")
	}()

	cfg, err := readExoscaleConfig(strings.NewReader(testConfigYAML_empty))
	ts.Require().NoError(err)
	ts.Require().Equal("", cfg.Global.APIKey)
	ts.Require().Equal("", cfg.Global.APISecret)
	ts.Require().Equal(testAPICredentialsFile, cfg.Global.APICredentialsFile)
}

func (ts *exoscaleCCMTestSuite) Test_readExoscaleConfig_env_typical() {
	os.Setenv("EXOSCALE_ZONE", testZone)
	os.Setenv("EXOSCALE_API_KEY", testAPIKey)
	os.Setenv("EXOSCALE_API_SECRET", testAPISecret)
	os.Setenv("EXOSCALE_API_ENDPOINT", testAPIEndpoint)
	defer func() {
		os.Unsetenv("EXOSCALE_ZONE")
		os.Unsetenv("EXOSCALE_API_KEY")
		os.Unsetenv("EXOSCALE_API_SECRET")
		os.Unsetenv("EXOSCALE_API_ENDPOINT")
	}()

	cfg, err := readExoscaleConfig(strings.NewReader(testConfigYAML_empty))
	ts.Require().NoError(err)
	ts.Require().Equal(testZone, cfg.Global.Zone)
	ts.Require().Equal(testAPIKey, cfg.Global.APIKey)
	ts.Require().Equal(testAPISecret, cfg.Global.APISecret)
	ts.Require().Equal("", cfg.Global.APICredentialsFile)
}

func (ts *exoscaleCCMTestSuite) Test_readExoscaleConfig_disabled() {
	cfg, err := readExoscaleConfig(strings.NewReader(testConfigYAML_disabled))
	ts.Require().NoError(err)
	ts.Require().Equal(true, cfg.Instances.Disabled)
	ts.Require().Equal(true, cfg.LoadBalancer.Disabled)
}

func (ts *exoscaleCCMTestSuite) Test_readExoscaleConfig_credsFile() {
	os.Unsetenv("EXOSCALE_API_CREDENTIALS_FILE")

	cfg, err := readExoscaleConfig(strings.NewReader(testConfigYAML_credsFile))
	ts.Require().NoError(err)
	ts.Require().Equal("", cfg.Global.APIKey)
	ts.Require().Equal("", cfg.Global.APISecret)
	ts.Require().Equal(testAPICredentialsFile, cfg.Global.APICredentialsFile)
}

func (ts *exoscaleCCMTestSuite) Test_readExoscaleConfig_typical() {
	os.Unsetenv("EXOSCALE_ZONE")
	os.Unsetenv("EXOSCALE_API_KEY")
	os.Unsetenv("EXOSCALE_API_SECRET")
	os.Unsetenv("EXOSCALE_API_ENDPOINT")

	cfg, err := readExoscaleConfig(strings.NewReader(testConfigYAML_typical))
	ts.Require().NoError(err)
	ts.Require().Equal(testZone, cfg.Global.Zone)
	ts.Require().Equal(testAPIKey, cfg.Global.APIKey)
	ts.Require().Equal(testAPISecret, cfg.Global.APISecret)
	ts.Require().Equal("", cfg.Global.APICredentialsFile)
	ts.Require().Equal(false, cfg.Instances.Disabled)
	ts.Require().Equal(false, cfg.LoadBalancer.Disabled)
}
