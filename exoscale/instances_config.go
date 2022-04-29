package exoscale

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/types"
)

// Instances configuration (<-> cloud-config file)
type instancesConfig struct {
	Disabled     bool // if true, disables this controller
	Overrides    []instancesOverrideConfig
	ExternalOnly bool `yaml:"externalOnly"` // if true, ignore Exoscale API (use only static overrides, if any)
}

type instancesOverrideConfig struct {
	Name       string // considered a regexp if '/.../'
	External   bool
	ExternalID string `yaml:"externalID"`
	Type       string
	Addresses  []instancesOverrideAddressConfig
	Region     string
}

type instancesOverrideAddressConfig struct {
	Type    string // Hostname, ExternalIP, InternalIP, ExternalDNS, InternalDNS (v1.NodeAddressType)
	Address string
}

// Return statically-configured instance override
func (c *instancesConfig) getInstanceOverride(nodeName types.NodeName) *instancesOverrideConfig {
	var config *instancesOverrideConfig

	// first try an exact match on name
	for _, candidate := range c.Overrides {
		if candidate.Name != "" && nodeName == types.NodeName(candidate.Name) {
			config = &candidate
			break
		}
	}

	// then regexp match on "name"
	if config == nil {
		for _, candidate := range c.Overrides {
			if strings.HasPrefix(candidate.Name, "/") && strings.HasSuffix(candidate.Name, "/") {
				if match, _ := regexp.Match(strings.Trim(candidate.Name, "/"), []byte(nodeName)); match {
					config = &candidate
					break
				}
			}
		}
	}

	return config
}

func (c *instancesConfig) getInstanceOverrideByProviderID(providerID string) *instancesOverrideConfig {
	var config *instancesOverrideConfig
	instanceID := strings.TrimPrefix(providerID, providerPrefix)

	// first try an exact match on externalID
	for _, candidate := range c.Overrides {
		if candidate.ExternalID != "" && instanceID == candidate.ExternalID {
			config = &candidate
			break
		}
	}

	// then try a match on the internally-built, name-based one
	if config == nil {
		for _, candidate := range c.Overrides {
			if candidate.ExternalID == "" && instanceID == fmt.Sprintf("external-%x", sha256.Sum256([]byte(candidate.Name))) {
				config = &candidate
				break
			}
		}
	}

	return config
}
