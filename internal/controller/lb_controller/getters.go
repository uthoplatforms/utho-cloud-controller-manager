package lb_controller

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/uthoplatforms/utho-go/utho"
	corev1 "k8s.io/api/core/v1"
)

// GetClusterID gets the cluster ID from the first node in the cluster
func (r *UthoApplicationReconciler) GetClusterID(ctx context.Context, l *logr.Logger) (string, error) {
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

// GetLB gets the load balancer with the specified ID using the Utho client
func GetLB(id string) (*utho.Loadbalancer, error) {
	lb, err := (*uthoClient).Loadbalancers().Read(id)
	if err != nil {
		return nil, err
	}
	return lb, nil
}

// GetCertificateID gets the certificate ID for a given certificate name using the Utho client
func GetCertificateID(certName string, l *logr.Logger) (string, error) {
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

// GetLB gets the load balancer with the specified ID using the Utho client
func GetVpcId(id string) (string, error) {
	k8s, err := (*uthoClient).Kubernetes().Read(id)
	if err != nil {
		return "", errors.Wrap(err, "Unable to get VPC Id")
	}

	return k8s.Info.Cluster.Vpc, nil
}
