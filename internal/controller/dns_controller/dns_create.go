package dns_controller

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1alpha1 "github.com/uthoplatforms/utho-cloud-controller-manager/api/v1alpha1"
	"github.com/uthoplatforms/utho-cloud-controller-manager/internal/controller"
	"github.com/uthoplatforms/utho-go/utho"
	"strings"
)

func (r *UthoDNSReconciler) CreateDomain(ctx context.Context, dns *appsv1alpha1.UthoDNS, l *logr.Logger) error {
	l.Info("Creating DNS Domain")

	if !controller.IsValidDomain(dns.Spec.Domain) {
		return errors.New(InvalidDomain)
	}
	dns.Status.Phase = appsv1alpha1.DomainPendingPhase
	if err := r.Status().Update(ctx, dns); err != nil {
		return errors.Wrap(err, "Error Updating DNS Pending Status")

	}
	params := utho.CreateDomainParams{
		Domain: dns.Spec.Domain,
	}
	_, err := (*uthoclient).Domain().CreateDomain(params)
	if err != nil {
		return err
	}

	dns.Status.Phase = appsv1alpha1.DomainCreatedPhase
	if err := r.Status().Update(ctx, dns); err != nil {
		return errors.Wrap(err, "Error Updating DNS Created Status")

	}
	l.Info("Domain Created Successfully")
	return nil
}

func (r *UthoDNSReconciler) CreateDnsRecords(ctx context.Context, dns *appsv1alpha1.UthoDNS, l *logr.Logger) error {
	l.Info("Creating DNS Domain Records")

	dns.Status.Phase = appsv1alpha1.DomainRecordsPendingPhase
	if err := r.Status().Update(ctx, dns); err != nil {
		return errors.Wrap(err, "Error Updating DNS Records Pending Status")
	}
	records := dns.Spec.Records
	for _, record := range records {
		if err := r.CreateDnsRecord(ctx, &record, dns, l); err != nil {
			if err.Error() == InvalidIPAddress {
				r.Recorder.Event(dns, "Warning", "IP Address Error", InvalidIPAddress)
				return nil
			}
			return err
		}
	}
	dns.Status.Phase = appsv1alpha1.DomainRecordsCreatedPhase
	dns.Status.RecordCount = len(records)
	if err := r.Status().Update(ctx, dns); err != nil {
		return errors.Wrap(err, "Error Updating DNS Records Created Status")
	}

	l.Info("DNS Records Created Successfully")
	return nil
}

func (r *UthoDNSReconciler) CreateDnsRecord(ctx context.Context, record *appsv1alpha1.Record, dns *appsv1alpha1.UthoDNS, l *logr.Logger) error {

	l.Info("Creating DNS Record")

	if !controller.IsValidIP(record.Value) {
		return errors.New(InvalidIPAddress)
	}
	params := utho.CreateDnsRecordParams{
		Domain:   dns.Spec.Domain,
		Type:     strings.ToUpper(record.Type),
		Hostname: strings.ToLower(record.Hostname),
		Value:    record.Value,
		TTL:      fmt.Sprintf("%d", record.TTL),
	}
	if record.Type == "SRV" {
		if record.PortType == "" || record.Weight == 0 || record.Port == 0 || record.Priority == 0 {
			return errors.New("Please provide all fields required for SRV record")
		}
	}
	if record.Type == "MX" {
		if record.Priority == 0 {
			return errors.New("Please provide all fields required for MX record")
		}
	}
	params.Porttype = record.PortType
	params.Wight = fmt.Sprintf("%d", record.Weight)
	params.Port = fmt.Sprintf("%d", record.Port)
	params.Priority = fmt.Sprintf("%d", record.Priority)

	res, err := (*uthoclient).Domain().CreateDnsRecord(params)
	if err != nil {
		return errors.Wrap(err, "Unable to Create DNS Record")
	}

	dns.Status.DNSRecordID = append(dns.Status.DNSRecordID, res.ID)
	if err := r.Status().Update(ctx, dns); err != nil {
		return errors.Wrap(err, "Error Updating DNS Record ID")
	}
	return nil
}
