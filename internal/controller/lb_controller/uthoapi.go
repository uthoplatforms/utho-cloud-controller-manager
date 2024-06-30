package lb_controller

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1alpha1 "github.com/uthoplatforms/utho-cloud-controller-manager/api/v1alpha1"
	"github.com/uthoplatforms/utho-go/utho"
)

// Declare global variables for Utho Client and error
var (
	uthoClient *utho.Client
	err        error
)

// AttachLBToCluster attaches the Load Balancer to the Kubernetes cluster using the Utho API
func (r *UthoApplicationReconciler) AttachLBToCluster(ctx context.Context, kubernetesID string, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {
	lbID := app.Status.LoadBalancerID

	if lbID == "" {
		return errors.New("no lb id found in the status field")
	}
	l.Info("Attaching Load Balancer to the Cluster")

	params := utho.CreateKubernetesLoadbalancerParams{
		LoadbalancerId: lbID,
		KubernetesId:   kubernetesID,
	}
	_, err := (*uthoClient).Kubernetes().CreateLoadbalancer(params)
	if err != nil {
		if err.Error() == LBAlreadyAttached {
			l.Info("LB Already attached to cluster")
			return nil
		}
		return errors.Wrap(err, "Error Attaching LB to the Cluster")
	}

	// Update the application status phase to indicate LB attachment is created
	app.Status.Phase = appsv1alpha1.LBAttachmentCreatedPhase
	err = r.Status().Update(ctx, app)
	if err != nil {
		return errors.Wrap(err, "Error Adding LB Attachment Phase to the Status Field")
	}
	return nil
}

// AttachTargetGroupsToCluster attached all target groups to the Kubernetes cluster using the Utho API
func (r *UthoApplicationReconciler) AttachTargetGroupsToCluster(ctx context.Context, kubernetesID string, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {
	if app.Spec.LoadBalancer.Type == "network" {
		return nil
	}
	l.Info("Attaching Target Groups to the Cluster")
	for _, tg := range app.Status.TargetGroupsID {
		if err := r.AttachTargetGroupToCluster(tg, kubernetesID, l); err != nil {
			if err.Error() == TGAlreadyAttached {
				continue
			}
			return errors.Wrapf(err, "Unable to Attach TG: %s to the Cluster", tg)
		}
	}

	// Update the application status phase to indicate target group attachment is created
	app.Status.Phase = appsv1alpha1.TGAttachmentCreatedPhase
	err := r.Status().Update(ctx, app)
	if err != nil {
		return errors.Wrap(err, "Unable to update TG Create Status.")
	}
	return nil
}

// AttachTargetGroupToCluster attaches a singke target group cluster to the Kubernetes cluster using the Utho API
func (r *UthoApplicationReconciler) AttachTargetGroupToCluster(tgID string, kubernetesID string, l *logr.Logger) error {
	params := &utho.CreateKubernetesTargetgroupParams{
		KubernetesId:            kubernetesID,
		KubernetesTargetgroupId: tgID,
	}

	l.Info("Attaching Target Group to Cluster")
	_, err := (*uthoClient).Kubernetes().CreateTargetgroup(*params)
	if err != nil {
		return errors.Wrap(err, "Error Attaching Target Group to Cluster")
	}
	return nil
}

// TGCreationOnwards manages the control flow from creation of target groups onwards and updates the application status
func (r *UthoApplicationReconciler) TGCreationOnwards(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {
	app.Status.Phase = appsv1alpha1.TGPendingPhase
	if err := r.Status().Update(ctx, app); err != nil {
		return err
	}

	l.Info("Creating Target Groups")
	if err := r.CreateTargetGroups(ctx, app, l); err != nil {
		l.Error(err, "Unable to Create Target Groups")
		app.Status.Phase = appsv1alpha1.TGErrorPhase
		if err := r.Status().Update(ctx, app); err != nil {
			return err
		}
		return err
	}

	if err := r.LBAttachmentOnwards(ctx, app, l); err != nil {
		return err
	}
	return nil
}

// LBAttachmentOnwards manages the control flow from Load Balancer attachment onwards and updates the application status
func (r *UthoApplicationReconciler) LBAttachmentOnwards(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {
	app.Status.Phase = appsv1alpha1.LBAttachmentPendingPhase
	if err := r.Status().Update(ctx, app); err != nil {
		return err
	}

	kubernetesID, err := r.GetClusterID(ctx, l)
	if err != nil {
		return errors.Wrap(err, "Unable to Get Cluster ID")
	}

	if err = r.AttachLBToCluster(ctx, kubernetesID, app, l); err != nil {
		l.Error(err, "Unable to Attach LB to Cluster")
		app.Status.Phase = appsv1alpha1.LBAttachmentErrorPhase
		if err := r.Status().Update(ctx, app); err != nil {
			return err
		}
		return errors.Wrap(err, "Unable to Attach LB to Cluster")
	}

	if err = r.TGAttachmentOnwards(ctx, app, l); err != nil {
		return err
	}

	return nil
}

// TGAttachmentOnwards manages the control flow from Target Group attachment onwards and updates the application status
func (r *UthoApplicationReconciler) TGAttachmentOnwards(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {
	app.Status.Phase = appsv1alpha1.TGAttachmentPendingPhase
	if err := r.Status().Update(ctx, app); err != nil {
		return err
	}

	kubernetesID, err := r.GetClusterID(ctx, l)
	if err != nil {
		return errors.Wrap(err, "Unable to Get Cluster ID")
	}

	if err = r.AttachTargetGroupsToCluster(ctx, kubernetesID, app, l); err != nil {
		l.Error(err, "Unable to Attach Target Groups to Cluster")
		app.Status.Phase = appsv1alpha1.TGAttachmentErrorPhase
		if err := r.Status().Update(ctx, app); err != nil {
			return err
		}
		return errors.Wrap(err, "Unable to Attach Target Groups to Cluster")
	}

	if err = r.FrontendCreationOnwards(ctx, app, l); err != nil {
		return err
	}

	return nil
}

// FrontendCreationOnwards manages the control flow from Frontend Creation onwards and updates the application status
func (r *UthoApplicationReconciler) FrontendCreationOnwards(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {
	app.Status.Phase = appsv1alpha1.FrontendPendingPhase
	if err := r.Status().Update(ctx, app); err != nil {
		return err
	}

	if err = r.CreateLBFrontend(ctx, app, l); err != nil {
		l.Error(err, "Unable to Create Frontend")
		app.Status.Phase = appsv1alpha1.FrontendErrorPhase
		if err := r.Status().Update(ctx, app); err != nil {
			return err
		}
		return errors.Wrap(err, "Unable to Create Frontend")
	}
	l.Info("Frontend Created")

	app.Status.Phase = appsv1alpha1.ACLPendingPhase
	if err = r.Status().Update(ctx, app); err != nil {
		return errors.Wrap(err, "Unable to Update ACL Pending Status")
	}

	if err = r.CreateACLRules(ctx, app, l); err != nil {
		return err
	}

	app.Status.Phase = appsv1alpha1.RunningPhase
	if err = r.Status().Update(ctx, app); err != nil {
		return errors.Wrap(err, "Unable to add Running Phase")
	}
	return nil
}
