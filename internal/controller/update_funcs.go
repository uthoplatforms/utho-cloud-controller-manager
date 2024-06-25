package controller

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1alpha1 "github.com/uthoplatforms/utho-cloud-controller-manager/api/v1alpha1"
	"github.com/uthoplatforms/utho-go/utho"
	"strings"
)

// UpdateFrontend updates the frontend configuration of a load balancer based on the specified UthoApplication
func (r *UthoApplicationReconciler) UpdateFrontend(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {

	l.Info("Updating Frontend")
	// Retrieve the frontend ID from the application's status
	frontendID := app.Status.FrontendID
	if frontendID == "" {
		return errors.New(FrontendIDNotFound)
	}

	// Retrieve the load balancer ID from the application's status
	lbID := app.Status.LoadBalancerID
	if lbID == "" {
		return errors.New(LBIDNotFound)
	}

	// Retrieve the frontend specifications from the application's spec
	frontend := app.Spec.LoadBalancer.Frontend

	// Create the parameters for updating the frontend
	params := &utho.UpdateLoadbalancerFrontendParams{
		LoadbalancerId: lbID,
		Name:           frontend.Name,
		Proto:          strings.ToLower(frontend.Protocol),
		Port:           fmt.Sprintf("%d", frontend.Port),
		Algorithm:      strings.ToLower(frontend.Algorithm),
		Redirecthttps:  TrueOrFalse(frontend.RedirectHttps),
		Cookie:         TrueOrFalse(frontend.Cookie),
	}

	// Get the certificate ID if the certificate name is provided
	certificateID, err := getCertificateID(frontend.CertificateName, l)
	if err != nil {
		if err.Error() == CertificateIDNotFound {
			l.Info("Certificate ID not found")
		} else {
			return errors.Wrap(err, "Error Getting Certificate ID")
		}
	}

	// Set the certificate ID in parameters
	if certificateID != "" {
		params.CertificateID = certificateID
	} else {
		params.CertificateID = "0"
	}

	// Update the parameters using Utho client
	_, err = (*uthoClient).Loadbalancers().UpdateFrontend(*params, lbID, frontendID)
	if err != nil {
		return errors.Wrap(err, "Error Updating Frontend")
	}
	return nil
}

// UpdateTargetGroups updates the target groups associated with an UthoApplication
func (r *UthoApplicationReconciler) UpdateTargetGroups(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {
	if app.Spec.LoadBalancer.Type == "network" {
		return nil
	}
	tgs := app.Spec.TargetGroups

	tgIds := app.Status.TargetGroupsID

	l.Info("Updating Target Groups")
	// Ensure the number of target groups matches the number of target group IDs
	if len(tgIds) != len(tgs) {
		return errors.New("Target groups Not Matching")
	}

	// Update each target group
	for i, tg := range tgs {
		if err := updateTargetGroup(&tg, tgIds[i], l); err != nil {
			return err
		}
	}
	return nil
}

// UpdateTargetGroup updates a single target group based on the provided parameters
func updateTargetGroup(tg *appsv1alpha1.TargetGroup, id string, l *logr.Logger) error {

	// Create the parameters for updating the target group
	params := &utho.UpdateTargetGroupParams{
		Name:                tg.Name,
		TargetGroupId:       id,
		Protocol:            strings.ToUpper(tg.Protocol),
		HealthCheckPath:     tg.HealthCheckPath,
		HealthCheckProtocol: strings.ToUpper(tg.HealthCheckProtocol),
		Port:                fmt.Sprintf("%d", tg.Port),
		HealthCheckTimeout:  fmt.Sprintf("%d", tg.HealthCheckTimeout),
		HealthCheckInterval: fmt.Sprintf("%v", tg.HealthCheckInterval),
		HealthyThreshold:    fmt.Sprintf("%v", tg.HealthyThreshold),
		UnhealthyThreshold:  fmt.Sprintf("%v", tg.UnhealthyThreshold),
	}

	l.Info("Updating Target Group")
	// Update the target group using the Utho client
	_, err := (*uthoClient).TargetGroup().Update(*params)
	if err != nil {
		return errors.Wrapf(err, "Error Updating Target Group %s\n", id)
	}
	return nil
}
