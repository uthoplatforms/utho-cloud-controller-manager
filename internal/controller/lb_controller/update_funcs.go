package lb_controller

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

// UpdateFrontend updates the frontend configuration of a load balancer based on the specified UthoApplication
func (r *UthoApplicationReconciler) UpdateFrontend(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {
	// Retrieve the frontend ID from the application's status
	frontendID := app.Status.FrontendID
	if frontendID == "" {
		return errors.New(FrontendIDNotFound)
	}

	currFrontend, err := (*uthoClient).Loadbalancers().ReadFrontend(app.Status.LoadBalancerID, frontendID)
	if err != nil {
		return errors.Wrap(err, "Error fetching frontend")
	}

	if controller.IsFrontendEqual(currFrontend, app.Spec.LoadBalancer.Frontend) {
		l.Info("No Change in Frontend")
		return nil
	}
	l.Info("Updating Frontend")

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
		Redirecthttps:  controller.TrueOrFalse(frontend.RedirectHttps),
		Cookie:         controller.TrueOrFalse(frontend.Cookie),
	}

	// Get the certificate ID if the certificate name is provided
	certificateID, err := GetCertificateID(frontend.CertificateName, l)
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
	if strings.ToLower(app.Spec.LoadBalancer.Type) == "network" || strings.ToLower(app.Spec.LoadBalancer.Type) != "application" {
		return nil
	}
	//tgs := app.Spec.TargetGroups
	//
	//tgIds := app.Status.TargetGroupsID

	l.Info("Updating Target Groups")

	// Fetch the current target groups from the Utho API
	currentTGs, err := (*uthoClient).TargetGroup().List()
	if err != nil {
		return errors.Wrap(err, "Error fetching target groups")
	}

	// Convert the current target groups to a map for easy lookup
	currentTGMap := make(map[string]utho.TargetGroup)
	for _, tg := range currentTGs {
		currentTGMap[tg.Name] = tg
	}

	// Iterate through the target groups in the application spec
	for _, specTG := range app.Spec.TargetGroups {
		// If the target group exists in the Utho API, update it
		if currentTG, ok := currentTGMap[specTG.Name]; ok {
			if !controller.IsTargetGroupEqual(currentTG, specTG) {
				if err := updateTargetGroup(&specTG, currentTG.ID, l); err != nil {
					return err
				}
			}
			// Remove the target group from the map
			delete(currentTGMap, specTG.Name)
		} else {
			// If the target group does not exist in the Utho API, create it
			err := r.CreateTargetGroup(ctx, &specTG, app, l)
			if err != nil {
				return err
			}
		}
	}

	// Any remaining target groups in the map have been removed from the UthoApplication spec, so delete them
	for _, tg := range currentTGMap {
		if err := DeleteTargetGroup(tg.ID, tg.Name); err != nil {
			return err
		}
		// Remove the deleted target group ID from the status
		app.Status.TargetGroupsID = controller.RemoveID(app.Status.TargetGroupsID, tg.ID)
		if err := r.Status().Update(ctx, app); err != nil {
			return errors.Wrap(err, "Error Updating Application Status")
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

func (r *UthoApplicationReconciler) UpdateAClRules(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {
	if strings.ToLower(app.Spec.LoadBalancer.Type) == "network" || strings.ToLower(app.Spec.LoadBalancer.Type) != "application" {
		return nil
	}

	l.Info("Updating ACL Rules")

	// Delete Existing Rules and Create New Rules
	if err := r.DeleteACLRules(ctx, app.Status.LoadBalancerID, app, app.Status.ACLRuleIDs, l); err != nil {
		return err
	}

	if err := r.CreateACLRules(ctx, app, l); err != nil {
		return err
	}

	return nil
}
