package controller

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/uthoplatforms/utho-go/utho"
	corev1 "k8s.io/api/core/v1"
)

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

// getClusterID gets the cluster ID from the first node in the cluster
func (r *UthoApplicationReconciler) getClusterID(ctx context.Context, l *logr.Logger) (string, error) {
	nodeList := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodeList); err != nil {
		return "", errors.Wrap(err, "Error Getting Node List")
	}

	l.Info("Fetching Cluster ID Label")
	clusterID := nodeList.Items[0].Labels["cluster_id"]
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
