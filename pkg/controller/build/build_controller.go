package build

import (
	"context"

	"strings"

	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/redhat-cop/operator-utils/pkg/util"
	"github.com/redhat-cop/quay-openshift-registry-operator/pkg/constants"
	"github.com/redhat-cop/quay-openshift-registry-operator/pkg/core"
	"github.com/redhat-cop/quay-openshift-registry-operator/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

	coreComponents := core.NewCoreComponents(reconcilerBase)

	return &ReconcileBuildIntegration{coreComponents: coreComponents}
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
	coreComponents core.CoreComponents
}

// Reconcile reads that state of the cluster for a Build object and makes changes based on the state read
// and what is in the Build.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileBuildIntegration) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	logging.Log.Info("Reconciling Build", "Request.Namespace", request.Namespace, "Request.Name", request.Name)

	// Fetch the Build instance
	instance := &buildv1.Build{}
	err := r.coreComponents.ReconcilerBase.GetClient().Get(context.TODO(), request.NamespacedName, instance)
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
		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       instance,
			Message:      "Unexpected number of ImageStream Annotation Components",
			KeyAndValues: []interface{}{"Namespace", instance.Namespace, "Build", instance.Name, "Annotation", buildImageStreamTagAnnotation, "Expected Size", "2", "Actual Size", len(buildImageStreamComponents)},
			Reason:       "ProcessingError",
		})
	}

	buildImageStreamNamespace := buildImageStreamComponents[0]

	imageNameTagComponents := strings.Split(buildImageStreamComponents[1], ":")

	if len(imageNameTagComponents) != 2 {
		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       instance,
			Message:      "Unexpected number of ImageStream Name Components",
			KeyAndValues: []interface{}{"Namespace", instance.Namespace, "Build", instance.Name, "Annotation", buildImageStreamTagAnnotation, "Actual Size", len(imageNameTagComponents)},
			Reason:       "ProcessingError",
		})

	}

	buildImageName := imageNameTagComponents[0]
	buildImageTag := imageNameTagComponents[1]

	logging.Log.Info("Importing ImageStream after Build", "ImageStream Namespace", buildImageStreamNamespace, "ImageStream Name", buildImageName, "ImageStream Tag", buildImageTag)

	quayIntegration, result, err := r.coreComponents.GetQuayIntegration(instance)

	if err != nil {
		return result, err
	}

	// First, Get the ImageStream
	existingImageStream := &imagev1.ImageStream{}
	err = r.coreComponents.ReconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Namespace: buildImageStreamNamespace, Name: buildImageName}, existingImageStream)

	if err != nil {
		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       instance,
			Message:      "Unable to locate ImageStream",
			KeyAndValues: []interface{}{"Namespace", buildImageStreamNamespace, "Build", buildImageName},
			Reason:       "ProcessingError",
		})

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

	err = r.coreComponents.ReconcilerBase.GetClient().Create(context.TODO(), isi)

	if err != nil {
		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       instance,
			Message:      "Error occurred creating ImageStreamImport",
			KeyAndValues: []interface{}{"Namespace", buildImageStreamNamespace, "ImageStream", buildImageName},
			Reason:       "ProcessingError",
			Error:        err,
		})
	}

	// Update the Build
	instance.GetAnnotations()[constants.BuildDestinationImageStreamTagImportedAnnotation] = "true"

	err = r.coreComponents.ReconcilerBase.GetClient().Update(context.TODO(), instance)
	if err != nil {
		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       instance,
			Message:      "Error occurred updating Build",
			KeyAndValues: []interface{}{"Namespace", instance.Namespace, "Build", instance.Name},
			Reason:       "ProcessingError",
			Error:        err,
		})
	}

	return reconcile.Result{}, nil
}
