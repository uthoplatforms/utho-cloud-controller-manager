package dns_controller

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1alpha1 "github.com/uthoplatforms/utho-cloud-controller-manager/api/v1alpha1"
)

func (r *UthoDNSReconciler) deleteExternalResources(ctx context.Context, dns *appsv1alpha1.UthoDNS, l *logr.Logger) error {
	l.Info("Deleting External Resources")

	_, err := (*uthoclient).Domain().DeleteDomain(dns.Spec.Domain)
	if err != nil {
		if err.Error() == DomainDeletionNotFound {
			l.Info("Domain Not Found")
			return nil
		}
		return errors.Wrap(err, "Unable to Delete Domain")
	}

	l.Info("Domain Deleted")
	return nil
}

func (r *UthoDNSReconciler) DeleteDNSRecord(ctx context.Context, domain, id string, l *logr.Logger) error {
	l.Info("Deleting DNS Record")

	_, err := (*uthoclient).Domain().DeleteDnsRecord(domain, id)
	if err != nil {
		return errors.Wrap(err, "Unable to Delete DNS Record")
	}

	l.Info("DNS Record Deleted")
	return nil
}
