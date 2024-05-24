package controller

import (
	"context"
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
func getLB(id string) (bool, error) {
	uthoClient, err := getAuthenticatedClient()
	if err != nil {
		return false, err
	}
	lb, err := (*uthoClient).Loadbalancers().Read(id)
	if err != nil {
		return false, err
	}
	if lb.ID != id {
		return false, nil
	}
	return true, nil
}

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
	l.Info("Creating Load Balancer")
	newLB, err := (*uthoClient).Loadbalancers().Create(lbreq)
	if err != nil {
		return err
	}
	app.Status.LoadBalancerID = newLB.Loadbalancerid
	app.Status.Phase = appsv1alpha1.LBCreatedPhase
	l.Info("Updating LB Details in the Status")
	r.Status().Update(ctx, app)

	return nil

}
