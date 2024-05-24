/*
Copyright 2024 Animesh.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"time"

	"github.com/pkg/errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/uthoplatforms/utho-cloud-controller-manager/api/v1alpha1"
)

const (
	finalizerID          = "utho-app-operator"
	errorRequeueDuration = 30 * time.Minute
)

// UthoApplicationReconciler reconciles a UthoApplication object
type UthoApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps.utho.com,resources=uthoapplications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.utho.com,resources=uthoapplications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.utho.com,resources=uthoapplications/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the UthoApplication object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *UthoApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	l.Info("Receieved Reconcile Request", req.Name, req.Namespace)

	app := &appsv1alpha1.UthoApplication{}

	if err := r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, app); err != nil {
		if apierrors.IsNotFound(err) {
			l.Error(err, "Unable to find Utho Application in the Cluster")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{RequeueAfter: errorRequeueDuration}, err
	}
	if app.Status.ObservedGeneration != app.ObjectMeta.Generation {
		app.Status.ObservedGeneration = app.ObjectMeta.Generation
		if err := r.Status().Update(ctx, app); err != nil {
			l.Error(err, "Couldn't Set Observed Generation")
			return ctrl.Result{}, errors.Wrap(err, "Couldn't Set Observed Generation")
		}

		// Is the Object Marked for Deletion
		if !app.ObjectMeta.DeletionTimestamp.IsZero() {
			l.Info("Application Marked for Deletion")
			if containsString(app.ObjectMeta.Finalizers, finalizerID) {
				if err := r.deleteExternalResources(ctx, app, &l); err != nil {
					return ctrl.Result{}, nil
				}
				app.ObjectMeta.Finalizers = removeString(app.ObjectMeta.Finalizers, finalizerID)
				if err := r.Update(ctx, app); err != nil {
					return ctrl.Result{}, errors.Wrap(err, "Could Not Remove Finalizer")
				}
			}
			return ctrl.Result{}, nil
		}

		// Add Finalizer if doesn't exists already
		if !containsString(app.ObjectMeta.Finalizers, finalizerID) {
			app.ObjectMeta.Finalizers = append(app.ObjectMeta.Finalizers, finalizerID)
			if err := r.Update(ctx, app); err != nil {
				return ctrl.Result{}, errors.Wrap(err, "Could Not Add Finalizer")
			}
		}

		if app.Status.Phase == "" {
			app.Status.Phase = appsv1alpha1.LBPendingPhase
			if err := r.Status().Update(ctx, app); err != nil {
				return ctrl.Result{}, errors.Wrap(err, "Unable to Add LB Pending Status.")
			}

			err := r.CreateUthoLoadBalancer(ctx, app, &l)
			if err != nil {
				l.Error(err, "Unable to Create LB")
				app.Status.Phase = appsv1alpha1.LBAttachmentErrorPhase
				if err := r.Status().Update(ctx, app); err != nil {
					return ctrl.Result{}, errors.Wrap(err, "Unale to Update LB Error Status")
				}
				return ctrl.Result{RequeueAfter: errorRequeueDuration}, errors.Wrap(err, "Unable to Create LB")
			}

			return ctrl.Result{}, nil
		} else if app.Status.Phase == appsv1alpha1.RunningPhase {

		}
		return ctrl.Result{}, nil
	} else {
		l.Info("Status Field Updated. No Need to do anything")
		return ctrl.Result{}, nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *UthoApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.UthoApplication{}).
		Complete(r)
}

func (r *UthoApplicationReconciler) createExternalResources(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {
	l.Info("Creating External Resources")
	return nil
}
func (r *UthoApplicationReconciler) deleteExternalResources(ctx context.Context, app *appsv1alpha1.UthoApplication, l *logr.Logger) error {
	l.Info("Deleting External Resources")
	return nil
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	var result []string
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return result
}
