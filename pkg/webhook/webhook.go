package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	buildv1 "github.com/openshift/api/build/v1"
	quayv1 "github.com/quay/quay-bridge-operator/api/v1"
	"github.com/quay/quay-bridge-operator/pkg/constants"
	"github.com/quay/quay-bridge-operator/pkg/logging"
	qotypes "github.com/quay/quay-bridge-operator/pkg/types"
	"github.com/quay/quay-bridge-operator/pkg/utils"
	jsonpatch "gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type QuayIntegrationMutator struct {
	Client  client.Client
	decoder *admission.Decoder
	Log     logr.Logger
}

// +kubebuilder:webhook:path=/admissionwebhook,mutating=true,failurePolicy=fail,verbs=create;update,groups="build.openshift.io",resources=builds,versions=v1,name=quayintegration.quay.redhat.com,sideEffects=None,admissionReviewVersions={v1}

func (q *QuayIntegrationMutator) Handle(ctx context.Context, req admission.Request) admission.Response {

	var admissionResponse *admissionv1.AdmissionResponse
	build := &buildv1.Build{}

	err := q.decoder.Decode(req, build)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Get QuayIntegration
	quayIntegration, found, err := q.getQuayIntegration(ctx, &req)

	if !found {

		if err != nil {
			admissionResponse = &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		} else {
			admissionResponse = &admissionv1.AdmissionResponse{
				Allowed: true,
			}
		}
	} else {
		// Check if builder service account has secret
		hasSecret, serviceAcctErr := q.checkSecretForBuilderServiceAccount(ctx, &req, &quayIntegration)

		if !hasSecret {
			if serviceAcctErr != nil {
				admissionResponse = &admissionv1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Message: serviceAcctErr.Error(),
					},
				}
			} else {
				admissionResponse = &admissionv1.AdmissionResponse{
					Allowed: false,
				}
			}
		} else {
			admissionResponse = getAdmissionResponseForBuild(build, &quayIntegration)
		}

	}

	return admission.Response{AdmissionResponse: *admissionResponse}

}

func (q *QuayIntegrationMutator) checkSecretForBuilderServiceAccount(ctx context.Context, ar *admission.Request, quayIntegration *quayv1.QuayIntegration) (bool, error) {

	builderServiceAccnt := &corev1.ServiceAccount{}

	err := q.Client.Get(ctx, types.NamespacedName{Namespace: ar.Namespace, Name: string(qotypes.BuilderOpenShiftServiceAccount)}, builderServiceAccnt)
	if err != nil {
		logging.Log.Info("Failed to get existing builder service account")
		return false, err
	}

	hasSecret := utils.ObjectReferenceNameExists(builderServiceAccnt.Secrets, utils.GenerateDockerJsonSecretNameForServiceAccount(builderServiceAccnt.ObjectMeta.Name, quayIntegration.Spec.ClusterID))
	if !hasSecret {
		logging.Log.Info("Builder service account does not have required secret")
	}
	return hasSecret, nil
}

func (q *QuayIntegrationMutator) getQuayIntegration(ctx context.Context, ar *admission.Request) (quayv1.QuayIntegration, bool, error) {

	// Find the Current Registered QuayIntegration objects
	quayIntegrations := quayv1.QuayIntegrationList{}

	err := q.Client.List(ctx, &quayIntegrations, &client.ListOptions{})

	if err != nil {
		return quayv1.QuayIntegration{}, false, err
	}

	if len(quayIntegrations.Items) != 1 {
		logging.Log.Info("No QuayIntegrations defined or more than 1 integration present")
		return quayv1.QuayIntegration{}, false, nil
	}

	quayIntegration := *&quayIntegrations.Items[0]

	// Check is this is a valid namespace (TODO: Use a predicate to filter out?)
	validNamespace := quayIntegration.IsAllowedNamespace(ar.Namespace)

	if !validNamespace {
		return quayv1.QuayIntegration{}, false, nil
	}

	return quayIntegration, true, nil
}

func getAdmissionResponseForBuild(build *buildv1.Build, quayIntegration *quayv1.QuayIntegration) *admissionv1.AdmissionResponse {

	var patch []jsonpatch.JsonPatchOperation

	quayRegistryHostname, err := quayIntegration.GetRegistryHostname()

	if (build.Spec.Strategy.DockerStrategy == nil && build.Spec.Strategy.SourceStrategy == nil) || build.Spec.CommonSpec.Output.To.Kind != "ImageStreamTag" {
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	var imageStreamDestinationNamespace = build.Namespace

	if build.Spec.CommonSpec.Output.To.Namespace != "" {
		imageStreamDestinationNamespace = build.Spec.CommonSpec.Output.To.Namespace
	}

	// Get ImageStream Name and Tag
	imageStremParts := strings.Split(build.Spec.Output.To.Name, ":")

	dockerImage := fmt.Sprintf("%s/%s/%s:%s", quayRegistryHostname, quayIntegration.GenerateQuayOrganizationNameFromNamespace(imageStreamDestinationNamespace), imageStremParts[0], imageStremParts[1])

	// Update the Kind
	patch = append(patch, jsonpatch.JsonPatchOperation{
		Operation: "replace",
		Path:      "/spec/output/to/kind",
		Value:     "DockerImage",
	})

	// Update the destination
	patch = append(patch, jsonpatch.JsonPatchOperation{
		Operation: "replace",
		Path:      "/spec/output/to/name",
		Value:     dockerImage,
	})

	// Remove the namespace attribute
	if build.Spec.CommonSpec.Output.To.Namespace != "" {
		patch = append(patch, jsonpatch.JsonPatchOperation{
			Operation: "remove",
			Path:      "/spec/output/to/namespace",
		})
	}

	// Add annotations to Build to for Build Controller to use
	patch = append(patch, jsonpatch.JsonPatchOperation{
		Operation: "add",
		Path:      "/metadata/annotations/" + escapeJSONPointer(constants.BuildOperatorManagedAnnotation),
		Value:     "true",
	})

	patch = append(patch, jsonpatch.JsonPatchOperation{
		Operation: "add",
		Path:      "/metadata/annotations/" + escapeJSONPointer(constants.BuildDestinationImageStreamAnnotation),
		Value:     fmt.Sprintf("%s/%s:%s", imageStreamDestinationNamespace, imageStremParts[0], imageStremParts[1]),
	})

	patchBytes, err := json.Marshal(patch)

	if err != nil {
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}

}

func escapeJSONPointer(s string) string {
	esc := strings.Replace(s, "~", "~0", -1)
	esc = strings.Replace(esc, "/", "~1", -1)
	return esc
}

// InjectDecoder injects the decoder.
func (a *QuayIntegrationMutator) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}
