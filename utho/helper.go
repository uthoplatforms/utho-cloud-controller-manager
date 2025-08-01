package utho

import (
	"context"
	"fmt"
	"time"

	"github.com/uthoplatforms/utho-go/utho"
	"golang.org/x/exp/rand"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	cloudprovider "k8s.io/cloud-provider"
)

// GetLabelValue fetches `labelKey` from the first node that carries it.
func GetLabelValue(clientset kubernetes.Interface, labelKey string) (string, error) {
	if clientset == nil {
		var err error
		clientset, err = GetKubeClient()
		if err != nil {
			return "", fmt.Errorf("GetLabelValue: %w", err)
		}
	}

	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("GetLabelValue: %w", err)
	}
	if len(nodes.Items) == 0 {
		return "", fmt.Errorf("GetLabelValue: cluster has no nodes")
	}

	for _, n := range nodes.Items {
		if v := n.Labels[labelKey]; v != "" {
			return v, nil
		}
	}

	return "", fmt.Errorf("GetLabelValue: label %q not found on any of %d nodes",
		labelKey, len(nodes.Items))
}

// GetNodePoolsID retrieves all unique node pool IDs from the nodes in the cluster
func GetNodePoolsID() ([]string, error) {
	clientset, err := GetKubeClient()
	if err != nil {
		return nil, fmt.Errorf("GetNodePoolsID: error creating Kubernetes client: %v", err)
	}

	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("GetNodePoolsID: error retrieving nodes: %v", err)
	}

	if len(nodes.Items) == 0 {
		return nil, fmt.Errorf("GetNodePoolsID: no nodes found in the cluster")
	}

	nodePoolIDs := make(map[string]struct{})
	for _, node := range nodes.Items {
		labels := node.GetLabels()
		if nodePoolId, exists := labels["nodepool_id"]; exists {
			nodePoolIDs[nodePoolId] = struct{}{}
		}
	}

	// Convert map keys to a slice
	uniqueNodePoolIDs := make([]string, 0, len(nodePoolIDs))
	for id := range nodePoolIDs {
		uniqueNodePoolIDs = append(uniqueNodePoolIDs, id)
	}

	return uniqueNodePoolIDs, nil
}

func GetDcslug(client utho.Client, clusterId string) (string, error) {
	cluster, err := client.Kubernetes().Read(clusterId)
	if err != nil {
		return "", fmt.Errorf("GetDcslug: unable to get kubernetes info: %v", err)
	}

	slug := cluster.Info.Cluster.Dcslug

	return slug, nil
}

func GenerateRandomString(length int) string {
	rand.Seed(uint64(time.Now().UnixNano()))

	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)

	for i := 0; i < length; i++ {
		result[i] = chars[rand.Intn(len(chars))]
	}

	return string(result)
}

func GetK8sInstance(client utho.Client, clusterID, instanceID string) (*utho.WorkerNode, error) {
	cluster, err := client.Kubernetes().Read(clusterID)
	if err != nil {
		return nil, fmt.Errorf("GetK8sInstance: %w", err)
	}
	for _, pool := range cluster.Nodepools {
		for idx := range pool.Workers {
			if pool.Workers[idx].ID == instanceID {
				return &pool.Workers[idx], nil
			}
		}
	}
	return nil, cloudprovider.InstanceNotFound
}

func GetKubeClient() (*kubernetes.Clientset, error) {
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
		return nil, fmt.Errorf("GetKubeClient: error building config: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("GetKubeClient: error creating Kubernetes client: %v", err)
	}

	return kubeClient, nil
}
