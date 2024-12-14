package utho

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/uthoplatforms/utho-go/utho"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	cloudprovider "k8s.io/cloud-provider"
)

const (
	PENDING  = "pending"
	ACTIVE   = "active"
	RESIZING = "resizing"
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
		return false, fmt.Errorf("failed to get kubeclient to update service: %s", err)
	}
	// Retrieve the cluster ID
	clusterId, err := GetLabelValue("cluster_id")
	if err != nil {
		return false, fmt.Errorf("InstanceExists: failed to get cluster ID: %w", err)
	}

	// Fetch utho Kubernetes node instance
	k8sNode, err := i.getInstanceById(node, clusterId)
	if err != nil {
		log.Printf("InstanceExists: instance(%s) exists check failed: %v", node.Spec.ProviderID, err) //

		if strings.Contains(err.Error(), "InstanceExists: invalid instance ID") ||
			strings.Contains(err.Error(), "InstanceExists: instance not found") {
			return false, nil
		}

		return false, fmt.Errorf("InstanceExists: unexpected error during instance existence check: %w", err)
	}

	// Check if instance exist
	if k8sNode.Cloudid == "" {
		log.Printf("InstanceExists: instance(%s) doesn't exist", node.Spec.ProviderID)
		return false, nil
	}

	return true, nil
}

// InstanceShutdown checks whether the instance is running or powered off.
func (i *instancesv2) InstanceShutdown(ctx context.Context, node *v1.Node) (bool, error) {
	// Retrieve the cluster ID
	clusterId, err := GetLabelValue("cluster_id")
	if err != nil {
		return false, fmt.Errorf("InstanceShutdown: failed to get cluster ID: %w", err)
	}

	// Fetch the instance information
	newNode, err := i.getInstanceById(node, clusterId)
	if err != nil {
		log.Printf("InstanceShutdown: instance(%s) shutdown check failed: %v", node.Spec.ProviderID, err) //
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
	// Retrieve the cluster ID
	clusterId, err := GetLabelValue("cluster_id")
	if err != nil {
		return nil, fmt.Errorf("InstanceMetadata: failed to get cluster ID: %w", err)
	}

	// Retrieve the data center slug
	slug, err := GetDcslug(i.client, clusterId)
	if err != nil {
		return nil, fmt.Errorf("InstanceMetadata: failed to get data center slug: %w", err)
	}
	x := node.Spec.ProviderID
	z := node.ObjectMeta.Name
	fmt.Println(x)
	fmt.Println(z)
	// Fetch the instance information
	k8sNode, err := i.getInstanceById(node, clusterId)
	if err != nil {
		log.Printf("InstanceMetadata: instance(%s) metadata retrieval failed: %v", node.Spec.ProviderID, err) //
		return nil, fmt.Errorf("InstanceMetadata: failed to get instance by ID: %w", err)
	}

	// Retrieve node instance addresses
	nodeAddress, err := i.nodeInstanceAddresses(k8sNode)
	if err != nil {
		return nil, fmt.Errorf("InstanceMetadata: failed to get node instance addresses: %w", err)
	}

	// Construct the InstanceMetadata struct
	uthoNode := cloudprovider.InstanceMetadata{
		ProviderID:    node.Spec.ProviderID,
		InstanceType:  k8sNode.Planid,
		Region:        slug,
		NodeAddresses: nodeAddress,
	}

	// Log the returned metadata
	log.Printf("InstanceMetadata: returned node metadata: %v", uthoNode) //
	return &uthoNode, nil
}

// nodeInstanceAddresses gathers public/private IP addresses and returns a []v1.NodeAddress.
func (i *instancesv2) nodeInstanceAddresses(instance *utho.WorkerNode) ([]v1.NodeAddress, error) {
	var addresses []v1.NodeAddress
	if instance == nil {
		return nil, fmt.Errorf("instance is nil, uninitialized: %v", instance)
	}

	if instance.Ip == "" && instance.PrivateNetwork.Ip == "" {
		return nil, fmt.Errorf("require public or private IP")
	}

	addresses = append(addresses,
		v1.NodeAddress{Type: v1.NodeInternalIP, Address: instance.Ip},
		v1.NodeAddress{Type: v1.NodeExternalIP, Address: instance.PrivateNetwork.Ip},
	)

	return addresses, nil
}

// getInstanceById attempts to obtain a Utho instance from the Utho API.
func (i *instancesv2) getInstanceById(node *v1.Node, clusterId string) (*utho.WorkerNode, error) {
	id, err := getInstanceIDFromProviderID(node)
	if err != nil {
		log.Printf("failed to parse provider ID (%s): %v", node.Spec.ProviderID, err)
		return nil, fmt.Errorf("failed to parse provider ID: %w", err)
	}

	newNode, err := GetK8sInstance(i.client, clusterId, id)
	if err != nil {
		log.Printf("failed to get instance by ID (%s): %v", id, err)
		return nil, fmt.Errorf("failed to fetch instance from Utho API: %w", err)
	}

	return newNode, nil
}

// getInstanceIDFromProviderID extracts a k8s node ID from the provider ID.
func getInstanceIDFromProviderID(node *v1.Node) (string, error) {
	if node.Spec.ProviderID == "" {
		nodeID, exists := node.Labels["node_id"]
		if !exists {
			return "", fmt.Errorf("setProviderID: label %s not found on node %s", "node_id", node.Name)
		}

		if nodeID == "" {
			return "", fmt.Errorf("setProviderID: label %s is empty on node %s", "node_id", node.Name)
		}

		node.Spec.ProviderID = "utho://" + nodeID
	}

	split := strings.Split(node.Spec.ProviderID, "://")
	if len(split) != 2 {
		return "", fmt.Errorf("getInstanceIDFromProviderID: unexpected providerID format: %s (expected format: utho://abc123)", node.Spec.ProviderID)
	}

	if split[0] != ProviderName {
		return "", fmt.Errorf("getInstanceIDFromProviderID: unexpected provider scheme: %s (expected: utho)", split[0])
	}

	return split[1], nil
}

func setProviderID(node *v1.Node, kubeClient kubernetes.Interface) error {
	if node == nil {
		return fmt.Errorf("setProviderID: node is nil")
	}

	node_id, exists := node.Labels["node_id"]
	if !exists {
		return fmt.Errorf("setProviderID: label %s not found on node %s", "node_id", node.Name)
	}

	if node_id == "" {
		return fmt.Errorf("setProviderID: label %s is empty on node %s", "node_id", node.Name)
	}

	node.Spec.ProviderID = "utho//" + node_id

	// Update the node object
	_, err := kubeClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("setProviderID: failed to update node: %v", err)
	}

	log.Printf("setProviderID: Successfully set providerID for node %s to utho//%s", node.Name, node_id)
	return nil
}

// getInstanceByName retrieves a Utho instance for a given NodeName.
// Returns an error if multiple nodes with the same name exist.
func getInstanceByName(client utho.Client, nodeName types.NodeName) (*utho.CloudInstance, error) {
	list, err := client.CloudInstances().List()
	if err != nil {
		return nil, fmt.Errorf("failed to list cloud instances: %w", err)
	}

	for _, instance := range list {
		if instance.Hostname == string(nodeName) {
			return &instance, nil
		}
	}

	return nil, cloudprovider.InstanceNotFound
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
		return err
	}

	l.kubeClient, err = kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}

	return nil
}
