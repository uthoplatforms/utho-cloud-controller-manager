package utho

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/pflag"
	"github.com/uthoplatforms/utho-go/utho"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

const (
	// ProviderName defines the cloud provider
	ProviderName   = "utho"
	accessTokenEnv = "UTHO_API_KEY"
	userAgent      = "CCM_USER_AGENT"
)

// Options currently stores the Kubeconfig that was passed in.
// We can use this to extend any other flags that may have been passed in that we require
var Options struct {
	KubeconfigFlag *pflag.Flag
}

type cloud struct {
	client        utho.Client
	instances     cloudprovider.InstancesV2
	loadbalancers cloudprovider.LoadBalancer
}

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(_ io.Reader) (i cloudprovider.Interface, err error) {
		return newCloud()
	})
}

func newCloud() (cloudprovider.Interface, error) {
	apiToken := os.Getenv(accessTokenEnv)
	if apiToken == "" {
		return nil, fmt.Errorf("newCloud: %s must be set in the environment (use a k8s secret)", accessTokenEnv)
	}
	debug := os.Getenv("debug")

	utho, err := utho.NewClient(apiToken)
	if err != nil {
		return nil, fmt.Errorf("newCloud: failed to create utho client: %v", err)
	}

	var dcslug string
	if debug != "" {
		dcslug = "inmumbaizone2"
	} else {
		clusterId, err := GetLabelValue(nil, "cluster_id")
		if err != nil {
			return nil, fmt.Errorf("newCloud: failed to get cluster ID: %w", err)
		}

		dcslug, err = GetDcslug(utho, clusterId)
		if err != nil {
			return nil, fmt.Errorf("newCloud: failed to get data center slug: %w", err)
		}
	}

	return &cloud{
		client:        utho,
		instances:     newInstancesV2(utho),
		loadbalancers: newLoadbalancers(utho, dcslug),
	}, nil
}

func (c *cloud) Initialize(_ cloudprovider.ControllerClientBuilder, _ <-chan struct{}) {
}

func (c *cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	klog.V(5).Info("called LoadBalancer")
	return c.loadbalancers, true
}

func (c *cloud) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

func (c *cloud) InstancesV2() (cloudprovider.InstancesV2, bool) {
	klog.V(5).Info("called InstancesV2")
	return c.instances, true
}

func (c *cloud) Zones() (cloudprovider.Zones, bool) {
	klog.V(5).Info("called Zones")
	return nil, false
}

func (c *cloud) Clusters() (cloudprovider.Clusters, bool) {
	klog.V(5).Info("called Clusters")
	return nil, false
}

func (c *cloud) Routes() (cloudprovider.Routes, bool) {
	klog.V(5).Info("called Routes")
	return nil, false
}

func (c *cloud) ProviderName() string {
	klog.V(5).Info("called ProviderName")
	return ProviderName
}

func (c *cloud) HasClusterID() bool {
	klog.V(5).Info("called HasClusterID")
	return false
}
