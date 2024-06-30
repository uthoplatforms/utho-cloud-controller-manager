package dns_controller

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1alpha1 "github.com/uthoplatforms/utho-cloud-controller-manager/api/v1alpha1"
)

func (r *UthoDNSReconciler) UpdateDNSRecords(ctx context.Context, dns *appsv1alpha1.UthoDNS, l *logr.Logger) error {
	l.Info("Updating DNS Domain Records")

	recs, err := (*uthoclient).Domain().ListDnsRecords(dns.Spec.Domain)
	if err != nil {
		return errors.Wrap(err, "Unable to List DNS Records")
	}

	for _, record := range recs {
		if err := r.DeleteDNSRecord(ctx, dns.Spec.Domain, record.ID, l); err != nil {
			return err
		}
	}
	dns.Status.DNSRecordID = []string{}
	if err := r.Status().Update(ctx, dns); err != nil {
		return errors.Wrap(err, "Error Updating DNS Records Created Status")
	}

	if err := r.CreateDnsRecords(ctx, dns, l); err != nil {
		return err
	}
	return nil
}
