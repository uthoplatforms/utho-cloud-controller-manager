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

	l.Info("Updating Frontend")
	frontendID := app.Status.FrontendID
	if frontendID == "" {
		return errors.New(FrontendIDNotFound)
	}

	lbID := app.Status.LoadBalancerID
	if lbID == "" {
		return errors.New(LBIDNotFound)
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
		if err.Error() == CertificateIDNotFound {
			l.Info("Certificate ID not found")
		} else {
			return errors.Wrap(err, "Error Getting Certificate ID")
		}
	}

	if certificateID != "" {
		params.CertificateID = certificateID
	} else {
		params.CertificateID = "0"
	}

	l.Info("Update Frontend Params", "params", params)
	_, err = (*uthoClient).Loadbalancers().UpdateFrontend(*params, lbID, frontendID)
	if err != nil {
		return errors.Wrap(err, "Error Updating Frontend")
	}
	return nil
}

func (r *UthoApplicationReconciler) UpdateTargetGroups(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {
	tgs := app.Spec.TargetGroups

	tgIds := app.Status.TargetGroupsID

	l.Info("Updating Target Groups")
	if len(tgIds) != len(tgs) {
		return errors.New("Target groups Not Matching")
	}

	for i, tg := range tgs {
		//l.Info("Target Groups", "TGID:", tgIds[i], "TG:", tg)
		if err := updateTargetGroup(&tg, tgIds[i], l); err != nil {
			return err
		}
	}
	return nil
}

func updateTargetGroup(tg *appsv1alpha1.TargetGroup, id string, l *logr.Logger) error {

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
	_, err := (*uthoClient).TargetGroup().Update(*params)
	if err != nil {
		return errors.Wrapf(err, "Error Updating Target Group %s\n", id)
	}
	return nil
}

//func (r *UthoApplicationReconciler) UpdateACLRules(app *appsv1alpha1.UthoApplication, l *logr.Logger) error {
//
//	l.Info("Updating ACL Rules")
//
//	acls := app.Spec.LoadBalancer.ACL
//	aclIDs := app.Status.ACLRuleIDs
//
//	frontendID := app.Status.FrontendID
//	lbID := app.Status.LoadBalancerID
//
//	if len(aclIDs) != len(acls) {
//		return errors.New("ACL Rules Not Matching")
//	}
//	for i, aclID := range aclIDs {
//		if err := UpdateACLRule(aclID, frontendID, lbID ,acls[i], l); err != nil {
//
//		}
//	}
//	return nil
//}
//
//func UpdateACLRule(id, frontendID , lbID string, rule appsv1alpha1.ACLRule, l *logr.Logger) error {
//
//	if id == "" {
//		return errors.New(ACLIDNotFound)
//	}
//
//	l.Info("Updating ACL Rule")
//	rule.Value.FrontendID = frontendID
//	byteValue, err := json.Marshal(rule.Value)
//	if err != nil {
//		return errors.Wrap(err, "Error Marshalling ACL Rule")
//	}
//	params := utho.Updat{
//		LoadbalancerId: lbID,
//		Name:           rule.Name,
//		ConditionType:  rule.ConditionType,
//		FrontendID:     frontendID,
//		Value:          string(byteValue),
//	}
//
//	(*uthoClient).Loadbalancers().
//	return nil
//}
