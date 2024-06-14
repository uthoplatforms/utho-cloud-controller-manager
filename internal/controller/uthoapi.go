package controller

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/util/json"
	"os"
	"strings"

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

// getAuthenticatedClient initialises and returns an authenticated Utho Client
func getAuthenticatedClient() (*utho.Client, error) {
	apiKey := os.Getenv("API_KEY")
	client, err := utho.NewClient(apiKey)
	if err != nil {
		return nil, err
	}
	return &client, nil
}

// CreateUthoLoadBalancer creates a new Load Balancer using the Utho API and updates the status of the application
func (r *UthoApplicationReconciler) CreateUthoLoadBalancer(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {
	lbreq := utho.CreateLoadbalancerParams{
		Dcslug: app.Spec.LoadBalancer.Dcslug,
		Type:   app.Spec.LoadBalancer.Type,
		Name:   app.Spec.LoadBalancer.Name,
	}
	newLB, err := (*uthoClient).Loadbalancers().Create(lbreq)
	if err != nil {
		return err
	}

	// Update the application status with the new Load Balancer ID and phase
	app.Status.LoadBalancerID = newLB.ID
	app.Status.Phase = appsv1alpha1.LBCreatedPhase

	lb, _ := getLB(newLB.ID)
	app.Status.LoadBalancerIP = lb.IP

	fmt.Printf("%+v\n", newLB)
	l.Info("Updating LB Details in the Status")
	if err = r.Status().Update(ctx, app); err != nil {
		return errors.Wrap(err, "Error updating LB status")
	}

	return nil

}

// CreateTargetGroups creates all target groups defined in the application's specifications
func (r *UthoApplicationReconciler) CreateTargetGroups(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {
	for _, tg := range app.Spec.TargetGroups {
		err := r.CreateTargetGroup(ctx, &tg, app, l)
		if err != nil {
			if err.Error() == TGAlreadyExists {
				l.Info(TGAlreadyExists)
				continue
			}
			return errors.Wrap(err, "Unable to Create Target Group")
		}
	}
	app.Status.Phase = appsv1alpha1.TGCreatedPhase
	if err := r.Status().Update(ctx, app); err != nil {
		return err
	}
	return nil
}

// CreateTargetGroup creates a single target group using the Utho API and updates the status of application
func (r *UthoApplicationReconciler) CreateTargetGroup(ctx context.Context, tg *appsv1alpha1.TargetGroup, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {
	l.Info("Creating Target Group")

	tgreq := utho.CreateTargetGroupParams{
		Name:                tg.Name,
		Protocol:            strings.ToUpper(tg.Protocol),
		HealthCheckPath:     tg.HealthCheckPath,
		HealthCheckProtocol: strings.ToUpper(tg.HealthCheckProtocol),
		Port:                fmt.Sprintf("%d", tg.Port),
		HealthCheckTimeout:  fmt.Sprintf("%d", tg.HealthCheckTimeout),
		HealthCheckInterval: fmt.Sprintf("%v", tg.HealthCheckInterval),
		HealthyThreshold:    fmt.Sprintf("%v", tg.HealthyThreshold),
		UnhealthyThreshold:  fmt.Sprintf("%v", tg.UnhealthyThreshold),
	}

	newTG, err := (*uthoClient).TargetGroup().Create(tgreq)
	if err != nil {
		//l.Error(err, "Unable to create TG")
		return err
	}
	// Add the new target group ID to the application's status
	l.Info("Adding TG ID to the Status Field")
	app.Status.TargetGroupsID = append(app.Status.TargetGroupsID, fmt.Sprintf("%d", newTG.ID))
	if err = r.Status().Update(ctx, app); err != nil {
		l.Error(err, "Unable to Add TG ID to State")
		return err
	}
	return nil
}

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

// CreateLBFrontend creates a frontend for the Load Balancer using the Utho API and updates the status of the application
func (r *UthoApplicationReconciler) CreateLBFrontend(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {

	lbID := app.Status.LoadBalancerID
	if lbID == "" {
		return errors.New(LBIDNotFound)
	}

	lb, err := getLB(lbID)
	if err != nil {
		return errors.Wrap(err, "Error Getting LB")
	}

	// Create frontend if none exists
	if len(lb.Frontends) == 0 {
		frontend := app.Spec.LoadBalancer.Frontend

		params := &utho.CreateLoadbalancerFrontendParams{
			LoadbalancerId: lbID,
			Name:           frontend.Name,
			Proto:          strings.ToLower(frontend.Protocol),
			Port:           fmt.Sprintf("%d", frontend.Port),
			Algorithm:      strings.ToLower(frontend.Algorithm),
			Redirecthttps:  TrueOrFalse(frontend.RedirectHttps),
			Cookie:         TrueOrFalse(frontend.Cookie),
		}
		certificateID, err := getCertificateID(frontend.CertificateName, l)
		if err != nil {
			if err.Error() == CertificateIDNotFound {
				l.Info("Certificate ID not found")
			} else {
				return errors.Wrap(err, "Error Getting Certificate ID")
			}
		}

		if certificateID != "" {
			params.CertificateID = certificateID
		}

		l.Info("Creating Frontend for LB")
		res, err := (*uthoClient).Loadbalancers().CreateFrontend(*params)
		if err != nil {
			return errors.Wrap(err, "Error Creating Frontend")
		}

		app.Status.FrontendID = res.ID
		app.Status.Phase = appsv1alpha1.FrontendCreatedPhase

		err = r.Status().Update(ctx, app)
		if err != nil {
			return errors.Wrap(err, "Error Updating Frontend in Status")
		}
	} else {
		// If frontend already exists, update the application status with the existing frontend
		app.Status.FrontendID = lb.Frontends[0].ID
		app.Status.Phase = appsv1alpha1.FrontendCreatedPhase

		err = r.Status().Update(ctx, app)
		if err != nil {
			return errors.Wrap(err, "Error Updating Frontend in Status")
		}
	}
	return nil
}

// CreateACLRules create ACL rules for the Load Balancer using Utho API and updates the status of the application
func (r *UthoApplicationReconciler) CreateACLRules(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {

	l.Info("Creating ACL Rules")
	rules := app.Spec.LoadBalancer.ACL
	for _, rule := range rules {
		if err := r.CreateACLRule(ctx, app, rule, l); err != nil {
			if err.Error() == ACLAlreadyExists {
				l.Info("ACL Rule already exists")
				continue
			}
			return err
		}
	}

	// Update the application status phase to indicate ACL Rules have been created is created
	app.Status.Phase = appsv1alpha1.ACLCreatedPhase
	if err := r.Status().Update(ctx, app); err != nil {
		return errors.Wrap(err, "Error Updating ACL Created Phase")
	}

	l.Info("ACL Rules Created")
	return nil
}

// CreateACLRule creates a single ACL rule for the Load Balancer using Utho API
func (r *UthoApplicationReconciler) CreateACLRule(ctx context.Context, app *appsv1alpha1.UthoApplication, rule appsv1alpha1.ACLRule, l *logr.Logger) error {
	frontendID := app.Status.FrontendID
	if frontendID == "" {
		return errors.New(FrontendIDNotFound)
	}

	lbID := app.Status.LoadBalancerID
	if lbID == "" {
		return errors.New(LBIDNotFound)
	}

	l.Info("Creating ACL Rule")
	rule.Value.FrontendID = frontendID
	byteValue, err := json.Marshal(rule.Value)
	if err != nil {
		return errors.Wrap(err, "Error Marshalling ACL Rule")
	}
	// Creating parameters to create ACL Rule
	params := utho.CreateLoadbalancerACLParams{
		LoadbalancerId: lbID,
		Name:           rule.Name,
		ConditionType:  rule.ConditionType,
		FrontendID:     frontendID,
		Value:          string(byteValue),
	}
	res, err := (*uthoClient).Loadbalancers().CreateACL(params)
	if err != nil {
		return err
	}

	// Updating ACL Rule ID to status of application
	app.Status.ACLRuleIDs = append(app.Status.ACLRuleIDs, res.ID)
	if err = r.Status().Update(ctx, app); err != nil {
		return errors.Wrap(err, "Error Updating ACL Rule ID in Status Field")
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

	kubernetesID, err := r.getClusterID(ctx, l)
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

	kubernetesID, err := r.getClusterID(ctx, l)
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
