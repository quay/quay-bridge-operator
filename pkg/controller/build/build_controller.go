package build

import (
	"context"

	"strings"
	"time"

	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/redhat-cop/operator-utils/pkg/util"
	redhatcopv1alpha1 "github.com/redhat-cop/quay-openshift-registry-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/quay-openshift-registry-operator/pkg/constants"
	"github.com/redhat-cop/quay-openshift-registry-operator/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Build Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {

	reconcilerBase := util.NewReconcilerBase(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), mgr.GetRecorder("build-controller"))

	return &ReconcileBuildIntegration{reconcilerBase: reconcilerBase}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("build-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	buildPredicate := predicate.Funcs{

		UpdateFunc: func(e event.UpdateEvent) bool {

			newBuild, ok := e.ObjectNew.(*buildv1.Build)

			// Check to see if it has the Managed Annotations
			_, managedAnnotationFound := e.MetaNew.GetAnnotations()[constants.BuildOperatorManagedAnnotation]
			_, imageStreamImportedAnnotationFound := e.MetaNew.GetAnnotations()[constants.BuildDestinationImageStreamTagImportedAnnotation]

			if !ok || !managedAnnotationFound || imageStreamImportedAnnotationFound || newBuild.Status.Phase != buildv1.BuildPhaseComplete {
				return false
			}

			return true
		},

		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}

	// Watch for changes to primary resource Build
	err = c.Watch(&source.Kind{Type: &buildv1.Build{}}, &handler.EnqueueRequestForObject{}, buildPredicate)
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileBuildIntegration implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileBuildIntegration{}

// ReconcileBuildIntegration reconciles a Build object
type ReconcileBuildIntegration struct {
	reconcilerBase util.ReconcilerBase
}

// Reconcile reads that state of the cluster for a Build object and makes changes based on the state read
// and what is in the Build.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileBuildIntegration) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := logging.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	logging.Log.Info("Reconciling Build")

	// Fetch the Build instance
	instance := &buildv1.Build{}
	err := r.reconcilerBase.GetClient().Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {

			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Get ImageStream Tag
	buildImageStreamTagAnnotation, found := instance.GetAnnotations()[constants.BuildDestinationImageStreamAnnotation]

	if !found {
		// If annotation not found, ImageStreamTag import has been completed
		return reconcile.Result{}, nil
	}

	buildImageStreamComponents := strings.Split(buildImageStreamTagAnnotation, "/")

	// Validate Annotation
	if len(buildImageStreamComponents) != 2 {
		reqLogger.Info("Unexpected number of ImageStream Annotation Components", "Component Size", len(buildImageStreamComponents))
		return reconcile.Result{}, err
	}

	buildImageStreamNamespace := buildImageStreamComponents[0]

	imageNameTagComponents := strings.Split(buildImageStreamComponents[1], ":")

	if len(imageNameTagComponents) != 2 {
		reqLogger.Info("Unexpected number of ImageStream Name Components", "Component Size", len(imageNameTagComponents))
		return reconcile.Result{}, err
	}

	buildImageName := imageNameTagComponents[0]
	buildImageTag := imageNameTagComponents[1]

	logging.Log.Info("Importing ImageStream after Build", "ImageStream Namespace", buildImageStreamNamespace, "ImageStream Name", buildImageName, "ImageStream Tag", buildImageTag)

	quayIntegration, found, err := r.getQuayIntegration()

	if !found {
		logging.Log.Info("No QuayIntegration Resource Found")
		return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 10}, err
	}

	if err != nil {
		logging.Log.Error(err, "Error attempting to locate QuayIntegration")
		return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 10}, err
	}

	// First, Get the ImageStream
	existingImageStream := &imagev1.ImageStream{}
	err = r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Namespace: buildImageStreamNamespace, Name: buildImageName}, existingImageStream)

	if err != nil {
		logging.Log.Error(err, "Unable to locate ImageStream")
		return reconcile.Result{}, err
	}

	isi := &imagev1.ImageStreamImport{
		ObjectMeta: metav1.ObjectMeta{
			Name:            buildImageName,
			Namespace:       buildImageStreamNamespace,
			ResourceVersion: existingImageStream.GetResourceVersion(),
		},
		Spec: imagev1.ImageStreamImportSpec{
			Import: true,
			Images: []imagev1.ImageImportSpec{
				{
					From: corev1.ObjectReference{
						Kind: "DockerImage",
						Name: instance.Spec.Output.To.Name,
					},
					To: &corev1.LocalObjectReference{Name: buildImageTag},
					ImportPolicy: imagev1.TagImportPolicy{
						Insecure:  quayIntegration.Spec.InsecureRegistry,
						Scheduled: quayIntegration.Spec.ScheduledImageStreamImport,
					},
					ReferencePolicy: imagev1.TagReferencePolicy{
						Type: imagev1.SourceTagReferencePolicy,
					},
				},
			},
		},
	}

	err = r.reconcilerBase.GetClient().Create(context.TODO(), isi)

	if err != nil {
		logging.Log.Error(err, "Error occurred creating ImageStream")
	}

	// Update the Build
	instance.GetAnnotations()[constants.BuildDestinationImageStreamTagImportedAnnotation] = "true"

	err = r.reconcilerBase.GetClient().Update(context.TODO(), instance)
	if err != nil {
		logging.Log.Error(err, "Unable to update build", "build", instance.Name)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileBuildIntegration) getQuayIntegration() (redhatcopv1alpha1.QuayIntegration, bool, error) {

	// Find the Current Registered QuayIntegration objects
	quayIntegrations := redhatcopv1alpha1.QuayIntegrationList{}

	err := r.reconcilerBase.GetClient().List(context.TODO(), &client.ListOptions{}, &quayIntegrations)

	if err != nil {
		return redhatcopv1alpha1.QuayIntegration{}, false, err
	}

	if len(quayIntegrations.Items) != 1 {
		logging.Log.Info("No QuayIntegrations defined or more than 1 integration present")
		return redhatcopv1alpha1.QuayIntegration{}, false, nil
	}

	return *&quayIntegrations.Items[0], true, nil
}
