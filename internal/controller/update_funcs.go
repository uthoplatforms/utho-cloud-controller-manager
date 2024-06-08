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

func (r *UthoApplicationReconciler) UpdateFrontend(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {

	frontendID := app.Status.FrontendID
	if frontendID == "" {
		return errors.New("no frontend id found in the status field")
	}

	lbID := app.Status.LoadBalancerID
	if lbID == "" {
		return errors.New("no lb id found in the status field")
	}

	frontend := app.Spec.LoadBalancer.Frontend

	params := &utho.UpdateLoadbalancerFrontendParams{
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

	_, err = (*uthoClient).Loadbalancers().UpdateFrontend(*params, lbID, frontendID)
	if err != nil {
		return errors.Wrap(err, "Error Updating Frontend")
	}
	return nil
}
