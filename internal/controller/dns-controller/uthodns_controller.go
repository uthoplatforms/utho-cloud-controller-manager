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

package dns_controller

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/uthoplatforms/utho-cloud-controller-manager/internal/controller"
	"github.com/uthoplatforms/utho-go/utho"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerOptions "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1alpha1 "github.com/uthoplatforms/utho-cloud-controller-manager/api/v1alpha1"
)

// UthoDNSReconciler reconciles a UthoDNS object
type UthoDNSReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

const (
	finalizerID = "utho-dns-operator"
)

var (
	uthoclient *utho.Client
	err        error
)

func init() {
	uthoclient, err = controller.GetAuthenticatedClient()
	if err != nil {
		panic(fmt.Errorf("No API Key Present to get authenticated client: %v", err))
	}
}

//+kubebuilder:rbac:groups=apps.utho.com,resources=uthodns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.utho.com,resources=uthodns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.utho.com,resources=uthodns/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the UthoDNS object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *UthoDNSReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	l.Info("Receieved Reconcile Request", req.Name, req.Namespace)

	//Fetch the UthoDNS instance
	dns := &appsv1alpha1.UthoDNS{}

	if err := r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, dns); err != nil {
		if apierrors.IsNotFound(err) {
			l.Error(err, "Unable to find Utho DNS in the Cluster")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !dns.ObjectMeta.DeletionTimestamp.IsZero() {
		l.Info("Application Marked for Deletion")
		if controller.ContainsString(dns.ObjectMeta.Finalizers, finalizerID) {
			if err := r.deleteExternalResources(ctx, dns, &l); err != nil {
				return ctrl.Result{}, err
			}
			dns.ObjectMeta.Finalizers = controller.RemoveString(dns.ObjectMeta.Finalizers, finalizerID)
			if err := r.Update(ctx, dns); err != nil {
				return ctrl.Result{}, errors.Wrap(err, "Could Not Remove Finalizer")
			}
		}

		return ctrl.Result{}, nil
	}

	// Add Finalizer if it doesn't exists already
	if !controller.ContainsString(dns.ObjectMeta.Finalizers, finalizerID) {
		dns.ObjectMeta.Finalizers = append(dns.ObjectMeta.Finalizers, finalizerID)
		if err := r.Update(ctx, dns); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "Could Not Add Finalizer")
		}
	}

	phase := dns.Status.Phase
	if phase == "" || phase == appsv1alpha1.DomainPendingPhase || phase == appsv1alpha1.DomainErrorPhase {
		// Domain Creation Onwards
		if err := r.CreateExternalResources(ctx, dns, &l); err != nil {
			return ctrl.Result{}, err
		}
	} else if phase == appsv1alpha1.DomainCreatedPhase || phase == appsv1alpha1.DomainRecordsPendingPhase || phase == appsv1alpha1.DomainRecordsErrorPhase {
		// Domain Records Creation Onwards
		l.Info("DNS Records Creation Phase")
		if err := r.DNSRecordsOnwards(ctx, dns, &l); err != nil {
			return ctrl.Result{}, err
		}
	} else if phase == appsv1alpha1.RunningPhase || phase == appsv1alpha1.DomainRecordsCreatedPhase {
		// Running Phase
		// Updation
		l.Info("Running Phase")
		dns.Status.Phase = appsv1alpha1.RunningPhase
		if err := r.Status().Update(ctx, dns); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "Unable to add Running Phase")
		}

		if err := r.UpdateDNSRecords(ctx, dns, &l); err != nil {
			return ctrl.Result{}, err
		}

		dns.Status.Phase = appsv1alpha1.RunningPhase
		if err := r.Status().Update(ctx, dns); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "Unable to add Running Phase")
		}
	}

	l.Info("Reconcile Completed")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UthoDNSReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.GenerationChangedPredicate{}
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.UthoDNS{}).
		WithEventFilter(pred).
		WithOptions(controllerOptions.Options{MaxConcurrentReconciles: 2}).
		Complete(r)
}
