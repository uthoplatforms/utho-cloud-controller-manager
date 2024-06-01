package controller

import (
	"flag"
	"path/filepath"

	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func GetClientSet() (*kubernetes.Clientset, error) {
	home := homedir.HomeDir()
	kubeconfig := flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "Location to the Kubeconfig file")

	/* Check whether code is running internally or externally and authenticate accordingly
	if kubeconfig exists {
		build config from kubeconfig
	} else {
		Build Config from SA credentials
	}
	*/
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
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
