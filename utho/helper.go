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
)

// GetLabelValue retrieves the value of a specified label from the first node in the cluster
func GetLabelValue(labelKey string, clientset kubernetes.Interface) (string, error) {
	var err error

	if clientset == nil {
		clientset, err = GetKubeClient()
		if err != nil {
			return "", fmt.Errorf("GetLabelValue: error creating Kubernetes client: %v", err)
		}
	}

	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("GetLabelValue: error retrieving nodes: %v", err)
	}

	if len(nodes.Items) == 0 {
		return "", fmt.Errorf("GetLabelValue: no nodes found in the cluster")
	}

	firstNode := nodes.Items[0]

	labels := firstNode.GetLabels()
	if labelValue, exists := labels[labelKey]; exists {
		return labelValue, nil
	}

	return "", fmt.Errorf("GetLabelValue: `%s` label not found on the first node", labelKey)
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

func GetK8sInstance(client utho.Client, clusterId, instanceId string) (*utho.WorkerNode, error) {
	cluster, err := client.Kubernetes().Read(clusterId)
	if err != nil {
		return nil, fmt.Errorf("GetK8sInstance: unable to get kubernetes info: %v", err)
	}

	for _, nodePool := range cluster.Nodepools {
		for _, node := range nodePool.Workers {
			if node.Cloudid == instanceId {
				return &node, nil
			}
		}
	}

	return nil, fmt.Errorf("GetK8sInstance: unable to get cluster node: ClusterId %s, NodeID %s", clusterId, instanceId)
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
