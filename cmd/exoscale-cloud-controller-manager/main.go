package main

import (
	"math/rand"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/wait"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/app"
	"k8s.io/cloud-provider/app/config"
	"k8s.io/cloud-provider/names"
	"k8s.io/cloud-provider/options"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	_ "k8s.io/component-base/metrics/prometheus/clientgo" // load all the prometheus client-go plugins
	_ "k8s.io/component-base/metrics/prometheus/version"  // for version metric registration
	"k8s.io/klog/v2"

	"github.com/exoscale/exoscale-cloud-controller-manager/exoscale"
)

func main() {
	rand.New(rand.NewSource(time.Now().UnixNano()))

	ccmOptions, err := options.NewCloudControllerManagerOptions()
	if err != nil {
		klog.Fatalf("unable to initialize command options: %v", err)
	}

	fss := cliflag.NamedFlagSets{}
	command := app.NewCloudControllerManagerCommand(ccmOptions, cloudInitializer, app.DefaultInitFuncConstructors, names.CCMControllerAliases(), fss, wait.NeverStop)

	// From https://github.com/kubernetes/cloud-provider/blob/master/sample/basic_main.go:
	// TODO: once we switch everything over to Cobra commands, we can go back to calling
	// utilflag.InitFlags() (by removing its pflag.Parse() call). For now, we have to set the
	// normalize func and add the go flag set by hand.
	// Here is an sample
	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	// utilflag.InitFlags()
	logs.InitLogs()
	defer logs.FlushLogs()

	command.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Name == "cloud-provider" {
			if err := flag.Value.Set(exoscale.ProviderName); err != nil {
				klog.Fatalf("unable to set cloud-provider flag value: %s", err)
			}
			return
		}
	})

	if err := command.Execute(); err != nil {
		klog.Fatalf("cloud-controller-manger command failed: %s", err)
	}
}

func cloudInitializer(config *config.CompletedConfig) cloudprovider.Interface {
	cloudConfig := config.ComponentConfig.KubeCloudShared.CloudProvider

	cloud, err := cloudprovider.InitCloudProvider(cloudConfig.Name, cloudConfig.CloudConfigFile)
	if err != nil {
		klog.Fatalf("Cloud provider could not be initialized: %v", err)
	}
	if cloud == nil {
		klog.Fatalf("Cloud provider is nil")
	}

	if !cloud.HasClusterID() {
		if config.ComponentConfig.KubeCloudShared.AllowUntaggedCloud {
			klog.Warning("detected a cluster without a ClusterID.  " +
				"A ClusterID will be required in the future.  " +
				"Please tag your cluster to avoid any future issues")
		} else {
			klog.Fatalf("no ClusterID found.  " +
				"A ClusterID is required for the cloud provider to function properly.  " +
				"This check can be bypassed by setting the allow-untagged-cloud option")
		}
	}
	return cloud
}
