// Package main is the entrypoint into utho-ccm
package main

import (
	goflag "flag"
	"math/rand"
	"time"

	"k8s.io/cloud-provider/names"

	"github.com/spf13/pflag"
	"github.com/utho/utho-cloud-controller-manager/utho"
	"k8s.io/apimachinery/pkg/util/wait"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/app"
	"k8s.io/cloud-provider/app/config"
	"k8s.io/cloud-provider/options"
	"k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	_ "k8s.io/component-base/metrics/prometheus/clientgo" // load all the prometheus client-go plugins
	_ "k8s.io/component-base/metrics/prometheus/version"  // for version metric registration
	"k8s.io/klog/v2"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	ccmOptions, err := options.NewCloudControllerManagerOptions()
	if err != nil {
		klog.Fatalf("unable to initialize command options: %v", err)
	}
	ccmOptions.KubeCloudShared.CloudProvider.Name = utho.ProviderName
	ccmOptions.Authentication.SkipInClusterLookup = true

	controllerAliases := names.CCMControllerAliases()

	command := app.NewCloudControllerManagerCommand(
		ccmOptions,
		cloudInitializer,
		app.DefaultInitFuncConstructors,
		controllerAliases,
		flag.NamedFlagSets{},
		wait.NeverStop)

	utho.Options.KubeconfigFlag = command.Flags().Lookup("kubeconfig")

	pflag.CommandLine.SetNormalizeFunc(flag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	defer logs.FlushLogs()

	// utho.SetupSecretWatcher(context.Background())
	// go utho.SecretWatcher.WatchSecrets()

	if err := command.Execute(); err != nil {
		klog.Fatal(err)
	}
}

func cloudInitializer(c *config.CompletedConfig) cloudprovider.Interface {
	cloudConfig := c.ComponentConfig.KubeCloudShared.CloudProvider
	// initialize cloud provider with the cloud provider name and config file provided
	cloud, err := cloudprovider.InitCloudProvider(cloudConfig.Name, cloudConfig.CloudConfigFile)
	if err != nil {
		klog.Fatalf("Cloud provider could not be initialized: %v", err)
	}
	if cloud == nil {
		klog.Fatalf("Cloud provider is nil")
	}

	return cloud
}
