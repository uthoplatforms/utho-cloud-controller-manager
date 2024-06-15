package controller

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1alpha1 "github.com/uthoplatforms/utho-cloud-controller-manager/api/v1alpha1"
)

// DeleteLB deletes the Load Balancer associated with the UthoApplication
// and updates the application's status to reflect the deletion.

func (r *UthoApplicationReconciler) DeleteLB(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {
	lbID := app.Status.LoadBalancerID

	// Check if the Load Balancer ID is present in the application's status
	if lbID == "" {
		return errors.New(LBIDNotFound)
	}

	// Delete the Load Balancer using uthoClient
	l.Info("Deleting LB")
	_, err := (*uthoClient).Loadbalancers().Delete(lbID)
	if err != nil {
		return errors.Wrap(err, "Error Deleting LB")
	}

	// Update the application's status to indicate that the Load Balancer has been deleted
	l.Info("Updating Status Field")
	app.Status.Phase = appsv1alpha1.LBDeletedPhase
	if err = r.Status().Update(ctx, app); err != nil {
		return errors.Wrap(err, "Error Updating LB Status.")
	}
	return nil
}

// DeleteTargetGroups deletes all Target Groups associated with the UthoApplication
// and updates the application's status to reflect the deletion.

func (r *UthoApplicationReconciler) DeleteTargetGroups(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {

	if app.Spec.LoadBalancer.Type == "network" {
		return nil
	}
	l.Info("Deleting Target Groups")
	tgs := app.Status.TargetGroupsID

	// Iterate through all Target Groups and delete each one
	for i, tg := range tgs {
		if err := DeleteTargetGroup(tg, app.Spec.TargetGroups[i].Name); err != nil {
			return err
		}
	}

	// Update the application's status to indicate that the Target Groups have been deleted
	app.Status.Phase = appsv1alpha1.TGDeletedPhase
	if err := r.Status().Update(ctx, app); err != nil {
		return errors.Wrap(err, "Error Updating Target Groups Deletion Status.")
	}
	return nil
}

// DeleteTargetGroup deletes a Target Group given its ID and name.
// This is a helper function used by DeleteTargetGroups.
func DeleteTargetGroup(id, name string) error {
	_, err := (*uthoClient).TargetGroup().Delete(id, name)
	if err != nil {
		return errors.Wrapf(err, "Error Deleting Target Group with ID: %s znd Name: %s", id, name)
	}
	return nil
}
