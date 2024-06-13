package controller

import (
	"context"
	"flag"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/uthoplatforms/utho-go/utho"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// GetClientSet returns a Kubernetes clientSet that can be used to interact with the Kubernetes API
func GetClientSet() (*kubernetes.Clientset, error) {
	home := homedir.HomeDir()
	kube := (home + "/.kube/config")

	// Try to build the config from the default kubernetes file
	config, err := clientcmd.BuildConfigFromFlags("", kube)
	if err != nil {
		// If the default config is not available, try to use the in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, errors.Wrap(err, "Unable to get Kubernetes Config")
		}
	}
	flag.Parse()

	// Create the clientSet for the config
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "Error Getting clientset")
	}

	return clientset, nil
}

// containsString checks if a string contains a specific string
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// removeString removes a specific string from a string slice
func removeString(slice []string, s string) []string {
	var result []string
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return result
}

// TrueOrFalse converts a boolean value to string representations
func TrueOrFalse(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// getClusterID gets the cluster ID from the labels of the nodes in the Kubernetes cluster
func getClusterID(ctx context.Context, l *logr.Logger) (string, error) {
	// Get the Kubernetes clientSet
	clientset, err := GetClientSet()
	if err != nil {
		return "", errors.Wrap(err, "Error getting clientset")
	}

	l.Info("Fetching Cluster ID Label")
	// List all the nodes in the cluster
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", errors.Wrap(err, "Error getting Kubernetes nodes")
	}

	// Get the cluster ID from the labels of the first node
	clusterID := nodes.Items[0].Labels["cluster_id"]
	if clusterID == "" {
		return "", errors.Wrap(err, "No Cluster ID found")
	}

	return clusterID, nil
}

// getLB gets the load balancer with the specified ID using the Utho client
func getLB(id string) (*utho.Loadbalancer, error) {
	lb, err := (*uthoClient).Loadbalancers().Read(id)
	if err != nil {
		return nil, err
	}
	return lb, nil
}

// getCertificateID gets the certificate ID for a given certificate name using the Utho client
func getCertificateID(certName string, l *logr.Logger) (string, error) {
	l.Info("Getting Certificate ID")

	if certName == "" {
		return "", errors.New(CertificateIDNotFound)
	}
	var certID string
	// List all certificates using the Utho client
	certs, err := (*uthoClient).Ssl().List()
	if err != nil {
		return "", errors.Wrap(err, "Error Getting Certificate ID")
	}

	// Search for the certificates with the specified name and retrieve its ID
	for _, cert := range certs {
		if cert.Name == certName {
			certID = cert.ID
		}
	}
	if certID != "" {
		return certID, nil
	}
	return "", errors.New(CertificateIDNotFound)
}
