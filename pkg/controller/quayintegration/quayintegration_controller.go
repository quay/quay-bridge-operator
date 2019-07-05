package quayintegration

import (
	"context"

	"github.com/redhat-cop/operator-utils/pkg/util"
	redhatcopv1alpha1 "github.com/redhat-cop/quay-openshift-registry-operator/pkg/apis/redhatcop/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/redhat-cop/quay-openshift-registry-operator/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new QuayIntegration Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {

	reconcilerBase := util.NewReconcilerBase(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), mgr.GetRecorder("quayintegration-controller"))

	return &ReconcileQuayIntegration{reconcilerBase: reconcilerBase}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("quayintegration-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource QuayIntegration
	err = c.Watch(&source.Kind{Type: &redhatcopv1alpha1.QuayIntegration{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileQuayIntegration implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileQuayIntegration{}

// ReconcileQuayIntegration reconciles a QuayIntegration object
type ReconcileQuayIntegration struct {
	reconcilerBase util.ReconcilerBase
}

// Reconcile reads that state of the cluster for a QuayIntegration object and makes changes based on the state read
// and what is in the QuayIntegration.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileQuayIntegration) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	logging.Log.Info("Reconciling QuayIntegration", "Request.Namespace", request.Namespace, "Request.Name", request.Name)

	// Fetch the QuayIntegration instance
	instance := &redhatcopv1alpha1.QuayIntegration{}
	err := r.reconcilerBase.GetClient().Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
