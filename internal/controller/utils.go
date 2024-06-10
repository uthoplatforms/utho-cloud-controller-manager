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

func GetClientSet() (*kubernetes.Clientset, error) {
	home := homedir.HomeDir()
	kube := (home + "/.kube/config")

	config, err := clientcmd.BuildConfigFromFlags("", kube)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, errors.Wrap(err, "Unable to get Kubernetes Config")
		}
	}
	flag.Parse()

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "Error Getting clientset")
	}

	return clientset, nil
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

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

func TrueOrFalse(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func getClusterID(ctx context.Context, l *logr.Logger) (string, error) {
	clientset, err := GetClientSet()
	if err != nil {
		return "", errors.Wrap(err, "Error getting clientset")
	}

	l.Info("Fetching Cluster ID Label")
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", errors.Wrap(err, "Error getting Kubernetes nodes")
	}
	clusterID := nodes.Items[0].Labels["cluster_id"]
	if clusterID == "" {
		return "", errors.Wrap(err, "No Cluster ID found")
	}

	return clusterID, nil
}

func getLB(id string) (*utho.Loadbalancer, error) {
	lb, err := (*uthoClient).Loadbalancers().Read(id)
	if err != nil {
		return nil, err
	}
	return lb, nil
}

func getCertificateID(certName string, l *logr.Logger) (string, error) {
	l.Info("Getting Certificate ID")

	if certName == "" {
		return "", errors.New(CertificateIDNotFound)
	}
	var certID string
	certs, err := (*uthoClient).Ssl().List()
	if err != nil {
		return "", errors.Wrap(err, "Error Getting Certificate ID")
	}

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
