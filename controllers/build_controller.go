/*
Copyright 2021.

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

package controllers

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	buildv1 "github.com/openshift/api/build/v1"

	imagev1 "github.com/openshift/api/image/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/quay/quay-bridge-operator/pkg/constants"
	"github.com/quay/quay-bridge-operator/pkg/core"
	"github.com/quay/quay-bridge-operator/pkg/logging"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// BuildIntegrationReconciler reconciles a QuayIntegration object
type BuildIntegrationReconciler struct {
	CoreComponents core.CoreComponents
	Log            logr.Logger
}

//+kubebuilder:rbac:groups=build.openshift.io,resources=builds,verbs=get;list;watch;create;update;patch

func (r *BuildIntegrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	logging.Log.Info("Reconciling Build", "Request.Namespace", req.Namespace, "Request.Name", req.Name)

	instance := &buildv1.Build{}
	err := r.CoreComponents.ReconcilerBase.GetClient().Get(ctx, req.NamespacedName, instance)

	if err != nil {
		if apierrors.IsNotFound(err) {
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
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       instance,
			Message:      "Unexpected number of ImageStream Annotation Components",
			KeyAndValues: []interface{}{"Namespace", instance.Namespace, "Build", instance.Name, "Annotation", buildImageStreamTagAnnotation, "Expected Size", "2", "Actual Size", len(buildImageStreamComponents)},
			Reason:       "ProcessingError",
		})
	}

	buildImageStreamNamespace := buildImageStreamComponents[0]

	imageNameTagComponents := strings.Split(buildImageStreamComponents[1], ":")

	if len(imageNameTagComponents) != 2 {
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       instance,
			Message:      "Unexpected number of ImageStream Name Components",
			KeyAndValues: []interface{}{"Namespace", instance.Namespace, "Build", instance.Name, "Annotation", buildImageStreamTagAnnotation, "Actual Size", len(imageNameTagComponents)},
			Reason:       "ProcessingError",
		})

	}

	buildImageName := imageNameTagComponents[0]
	buildImageTag := imageNameTagComponents[1]

	logging.Log.Info("Importing ImageStream after Build", "ImageStream Namespace", buildImageStreamNamespace, "ImageStream Name", buildImageName, "ImageStream Tag", buildImageTag)

	quayIntegration, result, err := r.CoreComponents.GetQuayIntegration(instance)

	if err != nil {
		return result, err
	}

	// First, Get the ImageStream
	existingImageStream := &imagev1.ImageStream{}
	err = r.CoreComponents.ReconcilerBase.GetClient().Get(ctx, types.NamespacedName{Namespace: buildImageStreamNamespace, Name: buildImageName}, existingImageStream)

	if err != nil {
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
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

	err = r.CoreComponents.ReconcilerBase.GetClient().Create(ctx, isi)

	if err != nil {
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       instance,
			Message:      "Error occurred creating ImageStreamImport",
			KeyAndValues: []interface{}{"Namespace", buildImageStreamNamespace, "ImageStream", buildImageName},
			Reason:       "ProcessingError",
			Error:        err,
		})
	}

	// Update the Build
	instance.GetAnnotations()[constants.BuildDestinationImageStreamTagImportedAnnotation] = "true"

	err = r.CoreComponents.ReconcilerBase.GetClient().Update(ctx, instance)
	if err != nil {
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       instance,
			Message:      "Error occurred updating Build",
			KeyAndValues: []interface{}{"Namespace", instance.Namespace, "Build", instance.Name},
			Reason:       "ProcessingError",
			Error:        err,
		})
	}

	return reconcile.Result{}, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *BuildIntegrationReconciler) SetupWithManager(mgr ctrl.Manager) error {

	buildPredicates := []predicate.Predicate{
		predicate.Funcs{

			UpdateFunc: func(e event.UpdateEvent) bool {

				newBuild, ok := e.ObjectNew.(*buildv1.Build)

				// Check to see if it has the Managed Annotations
				_, managedAnnotationFound := e.ObjectNew.GetAnnotations()[constants.BuildOperatorManagedAnnotation]
				_, imageStreamImportedAnnotationFound := e.ObjectNew.GetAnnotations()[constants.BuildDestinationImageStreamTagImportedAnnotation]

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
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&buildv1.Build{}).
		Watches(&source.Kind{Type: &buildv1.Build{}}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(buildPredicates...)).
		Complete(r)
}
