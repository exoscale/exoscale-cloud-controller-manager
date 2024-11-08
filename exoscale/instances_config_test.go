package exoscale

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/types"
)

var (
	// Overrides
	testInstanceOverrideType             = "testInstanceType"
	testInstanceOverrideAddress_internal = "192.0.2.42"
	testInstanceOverrideAddress_external = "203.0.113.42"
	testInstanceOverrideExternalName     = "testOverrideInstanceExternal"
	testInstanceOverrideExternalID       = "testInstanceExternalID"
	testInstanceOverrideExternalType     = "testInstanceTypeExternal"
	testInstanceOverrideExternalRegion   = "testInstanceRegionExternal"
	testInstanceOverrideRegexpName       = "/^testOverrideInstanceRegexp.*/"
	testInstanceOverrideRegexpNodeName   = "testOverrideInstanceRegexpF00b@r"
	testInstanceOverrideRegexpInstanceID = fmt.Sprintf("external-%x", sha256.Sum256([]byte(testInstanceOverrideRegexpName)))
	testInstanceOverrideRegexpProviderID = fmt.Sprintf("%s://%s", ProviderName, testInstanceOverrideRegexpInstanceID)

	// YAML
	testConfigYAML_instances = fmt.Sprintf(`---
global:
  zone: "%s"
  apiKey: "%s"
  apiSecret: "%s"
instances:
  overrides:
    - name: "testOverrideInstanceType"
      type: "%s"
    - name: "testOverrideInstanceAddresses"
      addresses:
        - type: InternalIP
          address: "%s"
        - type: ExternalIP
          address: "%s"
    - name: "%s"
      external: true
      externalID: "%s"
      type: "%s"
      region: "%s"
    - name: "%s"
      external: true
`,
		testZone, testAPIKey, testAPISecret,
		testInstanceOverrideType,
		testInstanceOverrideAddress_internal, testInstanceOverrideAddress_external,
		testInstanceOverrideExternalName, testInstanceOverrideExternalID, testInstanceOverrideExternalType, testInstanceOverrideExternalRegion,
		testInstanceOverrideRegexpName,
	)
)

func (ts *exoscaleCCMTestSuite) Test_readExoscaleConfig_instances() {
	os.Unsetenv("EXOSCALE_ZONE")
	os.Unsetenv("EXOSCALE_API_KEY")
	os.Unsetenv("EXOSCALE_API_SECRET")

	cfg, err := readExoscaleConfig(strings.NewReader(testConfigYAML_instances))

	// Global
	ts.Require().NoError(err)
	ts.Require().Equal(testZone, cfg.Global.Zone)
	ts.Require().Equal(testAPIKey, cfg.Global.APIKey)
	ts.Require().Equal(testAPISecret, cfg.Global.APISecret)

	// Overrides
	ts.Require().Equal(4, len(cfg.Instances.Overrides))

	// empty
	{
		override := cfg.Instances.getInstanceOverride("")
		ts.Require().Nil(override)
	}
	{
		override := cfg.Instances.getInstanceOverrideByProviderID("")
		ts.Require().Nil(override)
	}

	// invalid
	{
		override := cfg.Instances.getInstanceOverride("invalidNodeName")
		ts.Require().Nil(override)
	}
	{
		override := cfg.Instances.getInstanceOverrideByProviderID("invalidProviderID")
		ts.Require().Nil(override)
	}

	// testOverrideInstanceType
	{
		override := cfg.Instances.getInstanceOverride("testOverrideInstanceType")
		ts.Require().NotNil(override)
		ts.Require().Equal(false, override.External)
		ts.Require().Equal("", override.ExternalID)
		ts.Require().Equal(testInstanceOverrideType, override.Type)
		ts.Require().Equal(0, len(override.Addresses))
		ts.Require().Equal("", override.Region)
	}

	// testOverrideInstanceAddresses
	{
		override := cfg.Instances.getInstanceOverride("testOverrideInstanceAddresses")
		ts.Require().NotNil(override)
		ts.Require().Equal(false, override.External)
		ts.Require().Equal("", override.ExternalID)
		ts.Require().Equal("", override.Type)
		ts.Require().Equal(2, len(override.Addresses))
		ts.Require().Equal("InternalIP", override.Addresses[0].Type)
		ts.Require().Equal(testInstanceOverrideAddress_internal, override.Addresses[0].Address)
		ts.Require().Equal("ExternalIP", override.Addresses[1].Type)
		ts.Require().Equal(testInstanceOverrideAddress_external, override.Addresses[1].Address)
		ts.Require().Equal("", override.Region)
	}

	// testOverrideInstanceExternal
	{
		override := cfg.Instances.getInstanceOverride(types.NodeName(testInstanceOverrideExternalName))
		ts.Require().NotNil(override)
		ts.Require().Equal(true, override.External)
		ts.Require().Equal(testInstanceOverrideExternalID, override.ExternalID)
		ts.Require().Equal(testInstanceOverrideExternalType, override.Type)
		ts.Require().Equal(0, len(override.Addresses))
		ts.Require().Equal(testInstanceOverrideExternalRegion, override.Region)
	}
	{
		override := cfg.Instances.getInstanceOverrideByProviderID(testInstanceOverrideExternalID)
		ts.Require().NotNil(override)
		ts.Require().Equal(true, override.External)
		ts.Require().Equal(testInstanceOverrideExternalID, override.ExternalID)
		ts.Require().Equal(testInstanceOverrideExternalType, override.Type)
		ts.Require().Equal(0, len(override.Addresses))
		ts.Require().Equal(testInstanceOverrideExternalRegion, override.Region)
	}

	// testOverrideInstanceRegexp
	{
		override := cfg.Instances.getInstanceOverride(types.NodeName(testInstanceOverrideRegexpNodeName))
		ts.Require().NotNil(override)
		ts.Require().Equal(true, override.External)
		ts.Require().Equal("", override.ExternalID)
		ts.Require().Equal("", override.Type)
		ts.Require().Equal(0, len(override.Addresses))
		ts.Require().Equal("", override.Region)
	}
	{
		override := cfg.Instances.getInstanceOverrideByProviderID(testInstanceOverrideRegexpProviderID)
		ts.Require().NotNil(override)
		ts.Require().Equal(true, override.External)
		ts.Require().Equal("", override.ExternalID)
		ts.Require().Equal("", override.Type)
		ts.Require().Equal(0, len(override.Addresses))
		ts.Require().Equal("", override.Region)
	}
}
