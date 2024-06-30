package dns_controller

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1alpha1 "github.com/uthoplatforms/utho-cloud-controller-manager/api/v1alpha1"
)

func (r *UthoDNSReconciler) CreateExternalResources(ctx context.Context, dns *appsv1alpha1.UthoDNS, l *logr.Logger) error {
	l.Info("Creating External Resources")

	if err := r.CreateDomain(ctx, dns, l); err != nil {
		if err.Error() == fmt.Sprintf("Domain %s already exits in dns zones ", dns.Spec.Domain) {
			l.Info("Domain Already Exists")
			r.Recorder.Event(dns, "Warning", "Domain Error", "This Domain Already Exists")
			return nil
		}
		if err.Error() == InvalidDomain {
			l.Info("Invalid Domain")
			r.Recorder.Event(dns, "Warning", "Domain Error", InvalidDomain)
			return nil
		}
		dns.Status.Phase = appsv1alpha1.DomainErrorPhase
		if err := r.Status().Update(ctx, dns); err != nil {
			return errors.Wrap(err, "Error Updating DNS Error Status")
		}
		return err
	}

	if err := r.DNSRecordsOnwards(ctx, dns, l); err != nil {
		return err

	}
	return nil
}

func (r *UthoDNSReconciler) DNSRecordsOnwards(ctx context.Context, dns *appsv1alpha1.UthoDNS, l *logr.Logger) error {

	if err := r.CreateDnsRecords(ctx, dns, l); err != nil {
		dns.Status.Phase = appsv1alpha1.DomainRecordsErrorPhase
		if err := r.Status().Update(ctx, dns); err != nil {
			return errors.Wrap(err, "Error Updating DNS Records Error Status")
		}
		return err
	}

	dns.Status.Phase = appsv1alpha1.RunningPhase
	if err := r.Status().Update(ctx, dns); err != nil {
		return errors.Wrap(err, "Error Updating DNS Running Status")
	}
	return nil
}
