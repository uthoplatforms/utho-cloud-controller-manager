package controller

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1alpha1 "github.com/uthoplatforms/utho-cloud-controller-manager/api/v1alpha1"
	"github.com/uthoplatforms/utho-go/utho"
)

var uthoClient, _ = getAuthenticatedClient()

func getAuthenticatedClient() (*utho.Client, error) {
	apiKey := os.Getenv("API_KEY")
	client, err := utho.NewClient(apiKey)
	if err != nil {
		return nil, err
	}
	return &client, nil
}

const CertifcateIDNotFound string = "Certificate ID Not Found"

// func getLB(id string) (bool, error) {
// 	uthoClient, err := getAuthenticatedClient()
// 	if err != nil {
// 		return false, err
// 	}
// 	lb, err := (*uthoClient).Loadbalancers().Read(id)
// 	if err != nil {
// 		return false, err
// 	}
// 	if lb.ID != id {
// 		return false, nil
// 	}
// 	return true, nil
// }

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

	app.Status.LoadBalancerID = newLB.ID
	app.Status.Phase = appsv1alpha1.LBCreatedPhase

	l.Info("Updating LB Details in the Status")
	if err = r.Status().Update(ctx, app); err != nil {
		return errors.Wrap(err, "Error updating LB status")
	}

	return nil

}

func (r *UthoApplicationReconciler) CreateTargetGroups(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {
	for _, tg := range app.Spec.TargetGroups {
		err := r.CreateTargetGroup(ctx, &tg, app, l)
		if err != nil {
			return err
		}
	}
	app.Status.Phase = appsv1alpha1.TGCreatedPhase
	if err := r.Status().Update(ctx, app); err != nil {
		return err
	}
	return nil
}

func (r *UthoApplicationReconciler) CreateTargetGroup(ctx context.Context, tg *appsv1alpha1.TargetGroup, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {
	l.Info("Creating Target Group")

	tgreq := utho.CreateTargetGroupParams{
		Name:                tg.Name,
		Protocol:            tg.Protocol,
		HealthCheckPath:     tg.HealthCheckPath,
		HealthCheckProtocol: tg.HealthCheckProtocol,
		Port:                fmt.Sprintf("%d", tg.Port),
		HealthCheckTimeout:  fmt.Sprintf("%d", tg.HealthCheckTimeout),
		HealthCheckInterval: fmt.Sprintf("%v", tg.HealthCheckInterval),
		HealthyThreshold:    fmt.Sprintf("%v", tg.HealthyThreshold),
		UnhealthyThreshold:  fmt.Sprintf("%v", tg.UnhealthyThreshold),
	}

	fmt.Printf("%+v", tgreq)

	newTG, err := (*uthoClient).TargetGroup().Create(tgreq)
	if err != nil {
		l.Error(err, "Unable to create TG")
		return err
	}
	l.Info("Adding TG ID to the Status Field")
	app.Status.TargetGroupsID = append(app.Status.TargetGroupsID, fmt.Sprintf("%d", newTG.ID))
	if err = r.Status().Update(ctx, app); err != nil {
		l.Error(err, "Unable to Add TG ID to State")
		return err
	}
	return nil
}

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
		return errors.Wrap(err, "Error Attaching LB to the Cluster")
	}

	app.Status.Phase = appsv1alpha1.LBAttachmentCreatedPhase
	err = r.Status().Update(ctx, app)
	if err != nil {
		return errors.Wrap(err, "Error Adding LB Attachment Phase to the Status Field")
	}
	return nil
}

func (r *UthoApplicationReconciler) AttachTargetGroupsToCluster(ctx context.Context, kubernetesID string, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {

	l.Info("Attaching Target Groups to the Cluster")
	for _, tg := range app.Status.TargetGroupsID {
		if err := r.AttachTargetGroupToCluster(tg, kubernetesID, l); err != nil {
			return errors.Wrapf(err, "Unable to Attach TG: %s to the Cluster", tg)
		}
	}

	app.Status.Phase = appsv1alpha1.TGAttachmentCreatedPhase
	err := r.Status().Update(ctx, app)
	if err != nil {
		return errors.Wrap(err, "Unable to update TG Create Status.")
	}
	return nil
}

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

func (r *UthoApplicationReconciler) CreateLBFrontend(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {

	lbID := app.Status.LoadBalancerID
	if lbID == "" {
		return errors.New("no lb id found in the status field")
	}

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
		if err.Error() == CertifcateIDNotFound {
			l.Info("Certificate ID not found")
		} else {
			return errors.Wrap(err, "Error Getting Certificate ID")
		}
	}

	if certificateID != "" {
		params.CertificateID = certificateID
	}

	fmt.Printf("%+v", params)
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
	return nil
}

// func (r *UthoApplicationReconciler) FetchKubernetesID(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {

// 	ip := os.Getenv("HOST_IP")
// 	uthoClient, err := getAuthenticatedClient()
// 	if err != nil {
// 		return errors.Wrap(err, "Unable to get Utho Client")
// 	}

// 	k8s, err := (*uthoClient).Kubernetes().List()
// 	if err != nil {
// 		return errors.Wrap(err, "Unable to List Kubernetes Clusters")
// 	}

// }

func getCertificateID(certName string, l *logr.Logger) (string, error) {
	l.Info("Getting Certificate ID")

	var certID string
	certs, err := (*uthoClient).Ssl().List()
	if err != nil {
		return "", errors.Wrap(err, "Error Getting Certificate ID")
	}

	for _, cert := range certs {
		if cert.Name == certName {
			certID = cert.ID
		}
	}
	if certID != "" {
		return certID, nil
	}
	return "", errors.New(CertifcateIDNotFound)
}

func (r *UthoApplicationReconciler) DeleteLB(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {
	lbID := app.Status.LoadBalancerID

	if lbID == "" {
		return errors.New("no lb id found in the status field")
	}

	l.Info("Deleting LB")
	_, err := (*uthoClient).Loadbalancers().Delete(lbID)
	if err != nil {
		return errors.Wrap(err, "Error Deleting LB")
	}

	l.Info("Updating Status Field")
	app.Status.Phase = appsv1alpha1.LBDeletedPhase
	if err = r.Status().Update(ctx, app); err != nil {
		return errors.Wrap(err, "Error Updating LB Status.")
	}
	return nil
}

func (r *UthoApplicationReconciler) DeleteTargetGroups(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {

	l.Info("Deleting Target Groups")
	tgs := app.Status.TargetGroupsID

	for i, tg := range tgs {
		if err := DeleteTargetGroup(tg, app.Spec.TargetGroups[i].Name); err != nil {
			return err
		}
	}

	app.Status.Phase = appsv1alpha1.TGDeletedPhase
	if err := r.Status().Update(ctx, app); err != nil {
		return errors.Wrap(err, "Error Updating Target Groups Deletion Status.")
	}
	return nil
}

func DeleteTargetGroup(id, name string) error {
	_, err := (*uthoClient).TargetGroup().Delete(id, name)
	if err != nil {
		return errors.Wrapf(err, "Error Deleting Target Group with ID: %s znd Name: %s", id, name)
	}
	return nil
}
