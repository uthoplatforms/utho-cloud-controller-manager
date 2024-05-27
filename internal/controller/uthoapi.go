package controller

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	appsv1alpha1 "github.com/uthoplatforms/utho-cloud-controller-manager/api/v1alpha1"
	"github.com/uthoplatforms/utho-go/utho"
)

const LBUnavailable string = "Sorry but due to some network resources unvaiable on this location we unable to deploy your cloud, Please come back after sometime"

func getAuthenticatedClient() (*utho.Client, error) {
	apiKey := os.Getenv("API_KEY")
	client, err := utho.NewClient(apiKey)
	if err != nil {
		return nil, err
	}
	return &client, nil
}

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
	uthoClient, err := getAuthenticatedClient()
	if err != nil {
		return err
	}
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
	r.Status().Update(ctx, app)

	return nil

}

func (r *UthoApplicationReconciler) CreateTargetGroups(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {
	uthoClient, err := getAuthenticatedClient()
	if err != nil {
		l.Error(err, "Unable to get Utho Client")
		return err
	}
	for _, tg := range app.Spec.TargetGroups {
		err := r.CreateTargetGroup(ctx, &tg, app, l, uthoClient)
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

func (r *UthoApplicationReconciler) CreateTargetGroup(ctx context.Context, tg *appsv1alpha1.TargetGroup, app *appsv1alpha1.UthoApplication, l *logr.Logger, uthoClient *utho.Client) error {
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
	app.Status.TargetGroupsID = append(app.Status.TargetGroupsID, fmt.Sprintf("%d", newTG.ID))
	if err = r.Status().Update(ctx, app); err != nil {
		l.Error(err, "Unable to Add TG ID to State")
		return err
	}
	return nil
}
