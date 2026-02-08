package utho

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/uthoplatforms/utho-go/utho"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

const (
	nodeIDLabel = "node_id"
)

var _ cloudprovider.InstancesV2 = &instancesv2{}

type instancesv2 struct {
	client utho.Client

	kubeClient kubernetes.Interface
}

func newInstancesV2(client utho.Client) cloudprovider.InstancesV2 {
	return &instancesv2{client, nil}
}

func (i *instancesv2) InstanceExists(ctx context.Context, node *v1.Node) (bool, error) {
	if err := i.GetKubeClient(); err != nil {
		return false, fmt.Errorf("InstanceExists: failed to init kube client: %w", err)
	}

	clusterID, err := GetLabelValue(i.kubeClient, "cluster_id")
	if err != nil {
		return false, fmt.Errorf("InstanceExists: failed to read cluster_id: %w", err)
	}

	k8sNode, err := i.getInstanceById(node, clusterID)
	if err != nil {
		if errors.Is(err, cloudprovider.InstanceNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("InstanceExists: unexpected error: %w", err)
	}

	if k8sNode == nil || k8sNode.ID == "" {
		return false, nil
	}
	return true, nil
}

// InstanceShutdown checks whether the instance is running or powered off.
func (i *instancesv2) InstanceShutdown(ctx context.Context, node *v1.Node) (bool, error) {
	if err := i.GetKubeClient(); err != nil {
		return false, fmt.Errorf("InstanceShutdown: failed to get kubeclient: %w", err)
	}
	// Retrieve the cluster ID
	clusterId, err := GetLabelValue(i.kubeClient, "cluster_id")
	if err != nil {
		return false, fmt.Errorf("InstanceShutdown: failed to get cluster ID: %w", err)
	}

	// Fetch the instance information
	newNode, err := i.getInstanceById(node, clusterId)
	if err != nil {
		klog.Errorf("InstanceShutdown: instance(%s) shutdown check failed: %v", node.Spec.ProviderID, err)
		return false, fmt.Errorf("InstanceShutdown: failed to get instance by ID: %w", err)
	}

	// Check the instance status
	if newNode.Status != "Installed" {
		return true, nil // Instance is not running
	}

	return false, nil // Instance is running
}

// InstanceMetadata returns a struct of type InstanceMetadata containing the node information.
func (i *instancesv2) InstanceMetadata(ctx context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	if err := i.GetKubeClient(); err != nil {
		return nil, fmt.Errorf("InstanceMetadata: failed to get kubeclient: %w", err)
	}
	// Retrieve the cluster ID
	clusterId, err := GetLabelValue(i.kubeClient, "cluster_id")
	if err != nil {
		return nil, fmt.Errorf("InstanceMetadata: failed to get cluster ID: %w", err)
	}

	// Retrieve the data center slug
	slug, err := GetLabelValue(i.kubeClient, "cluster_dcslug")
	if err != nil {
		return nil, fmt.Errorf("InstanceMetadata: failed to get data center slug: %w", err)
	}

	// Fetch the instance information
	k8sNode, err := i.getInstanceById(node, clusterId)
	if err != nil {
		klog.Errorf("InstanceMetadata: instance(%s) metadata retrieval failed: %v", node.Spec.ProviderID, err)
		return nil, fmt.Errorf("InstanceMetadata: failed to get instance by ID: %w", err)
	}

	// Retrieve node instance addresses
	nodeAddress, err := i.nodeInstanceAddresses(k8sNode)
	if err != nil {
		return nil, fmt.Errorf("InstanceMetadata: failed to get node instance addresses: %w", err)
	}

	// Construct the InstanceMetadata struct
	uthoNode := cloudprovider.InstanceMetadata{
		ProviderID:    fmt.Sprintf("utho://%s", k8sNode.ID),
		InstanceType:  k8sNode.Planid,
		Region:        slug,
		NodeAddresses: nodeAddress,
	}

	// Log the returned metadata
	klog.V(5).Infof("InstanceMetadata: returned node metadata: %v", uthoNode)
	return &uthoNode, nil
}

// nodeInstanceAddresses gathers public/private IP addresses and returns a []v1.NodeAddress.
func (i *instancesv2) nodeInstanceAddresses(instance *utho.WorkerNode) ([]v1.NodeAddress, error) {
	var addresses []v1.NodeAddress
	if instance == nil {
		return nil, fmt.Errorf("nodeInstanceAddresses: instance is nil")
	}

	if instance.Ip == "" && instance.PrivateNetwork.Ip == "" {
		return nil, fmt.Errorf("nodeInstanceAddresses: require public or private IP")
	}

	addresses = append(addresses,
		v1.NodeAddress{Type: v1.NodeInternalIP, Address: instance.PrivateNetwork.Ip},
		// v1.NodeAddress{Type: v1.NodeExternalIP, Address: instance.PrivateNetwork.Ip},
	)

	return addresses, nil
}

// getInstanceById attempts to obtain a Utho instance from the Utho API.
func (i *instancesv2) getInstanceById(node *v1.Node, clusterID string) (*utho.WorkerNode, error) {
	id, err := getInstanceIDFromProviderID(node)
	if err != nil {
		return nil, fmt.Errorf("getInstanceById: %w", err)
	}

	newNode, err := GetK8sInstance(i.client, clusterID, id)
	if err != nil {
		if errors.Is(err, cloudprovider.InstanceNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("getInstanceById: %w", err)
	}
	return newNode, nil
}

// getInstanceIDFromProviderID extracts a k8s node ID from the provider ID.
func getInstanceIDFromProviderID(node *v1.Node) (string, error) {
	if node.Spec.ProviderID == "" {
		if nodeID, ok := node.Labels[nodeIDLabel]; ok && nodeID != "" {
			return nodeID, nil
		}
		return "", fmt.Errorf("providerID empty and %s label not set on node %s", nodeIDLabel, node.Name)
	}

	parts := strings.SplitN(node.Spec.ProviderID, "://", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("unexpected providerID format: %s (expected utho://abc123)", node.Spec.ProviderID)
	}
	if parts[0] != ProviderName {
		return "", fmt.Errorf("unexpected provider scheme: %s (expected utho)", parts[0])
	}
	return parts[1], nil
}

func setProviderID(node *v1.Node, c kubernetes.Interface) error {
	id := node.Labels[nodeIDLabel]
	if id == "" {
		return fmt.Errorf("setProviderID: %s label empty on node %s", nodeIDLabel, node.Name)
	}

	patch := []byte(fmt.Sprintf(`{"spec":{"providerID":"utho://%s"}}`, id))
	_, err := c.CoreV1().Nodes().Patch(context.TODO(),
		node.Name,
		types.StrategicMergePatchType,
		patch,
		metav1.PatchOptions{})
	return err
}

// getInstanceByName retrieves a Utho instance for a given NodeName.
// Returns an error if multiple nodes with the same name exist.
func getInstanceByName(client utho.Client, nodeName types.NodeName) (*utho.CloudInstance, error) {
	list, err := client.CloudInstances().List()
	if err != nil {
		return nil, fmt.Errorf("getInstanceByName: failed to list cloud instances: %w", err)
	}

	for _, instance := range list {
		if instance.Hostname == string(nodeName) {
			return &instance, nil
		}
	}

	return nil, fmt.Errorf("getInstanceByName: cloudprovider.InstanceNotFound")
}

func (l *instancesv2) GetKubeClient() error {
	if l.kubeClient != nil {
		return nil
	}

	var (
		kubeConfig *rest.Config
		err        error
		config     string
	)

	// If no kubeconfig was passed in or set then we want to default to an empty string
	// This will have `clientcmd.BuildConfigFromFlags` default to `restclient.InClusterConfig()` which was existing behavior
	if Options.KubeconfigFlag == nil || Options.KubeconfigFlag.Value.String() == "" {
		config = ""
	} else {
		config = Options.KubeconfigFlag.Value.String()
	}

	kubeConfig, err = clientcmd.BuildConfigFromFlags("", config)
	if err != nil {
		return fmt.Errorf("GetKubeClient: error building config: %w", err)
	}

	l.kubeClient, err = kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("GetKubeClient: error creating Kubernetes client: %w", err)
	}

	return nil
}
