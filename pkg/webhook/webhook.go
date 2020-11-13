package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	buildv1 "github.com/openshift/api/build/v1"
	redhatcopv1alpha1 "github.com/redhat-cop/quay-openshift-registry-operator/api/redhatcop/v1alpha1"
	"github.com/redhat-cop/quay-openshift-registry-operator/pkg/constants"
	"github.com/redhat-cop/quay-openshift-registry-operator/pkg/logging"
	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type WebhookServer struct {
	Server       *http.Server
	Deserializer runtime.Decoder
	Client       client.Client
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func (wsvr *WebhookServer) Handle(w http.ResponseWriter, r *http.Request) {

	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		logging.Log.Error(fmt.Errorf("empty body"), "empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		logging.Log.Info("Invalid Content Type", "Content-Type", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *v1beta1.AdmissionResponse
	ar := v1beta1.AdmissionReview{}
	if _, _, err := wsvr.Deserializer.Decode(body, nil, &ar); err != nil {
		logging.Log.Error(err, "Can't decode body")
		admissionResponse = &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {

		// Get QuayIngegration
		quayIntegration, found, err := wsvr.getQuayIntegration(&ar)

		if !found {

			if err != nil {
				admissionResponse = &v1beta1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Message: err.Error(),
					},
				}
			} else {
				admissionResponse = &v1beta1.AdmissionResponse{
					Allowed: true,
				}
			}
		} else {

			admissionResponse = getAdmissionResponseForBuild(&ar, &quayIntegration)

		}
	}

	admissionReview := v1beta1.AdmissionReview{}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		logging.Log.Error(err, "Can't encode response")
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	if _, err := w.Write(resp); err != nil {
		logging.Log.Error(err, "Can't write response")
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}

}

func getAdmissionResponseForBuild(ar *v1beta1.AdmissionReview, quayIntegration *redhatcopv1alpha1.QuayIntegration) *v1beta1.AdmissionResponse {

	var patch []patchOperation

	var build buildv1.Build

	if err := json.Unmarshal(ar.Request.Object.Raw, &build); err != nil {
		logging.Log.Error(err, "Could not unmarshal raw object")
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	quayRegistryHostname, err := quayIntegration.GetRegistryHostname()

	if build.Spec.CommonSpec.Output.To.Kind != "ImageStreamTag" {
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	var imageStreamDestinationNamespace = ar.Request.Namespace

	if build.Spec.CommonSpec.Output.To.Namespace != "" {
		imageStreamDestinationNamespace = build.Spec.CommonSpec.Output.To.Namespace
	}

	// Get ImageStream Name and Tag
	imageStremParts := strings.Split(build.Spec.Output.To.Name, ":")

	dockerImage := fmt.Sprintf("%s/%s/%s:%s", quayRegistryHostname, quayIntegration.GenerateQuayOrganizationNameFromNamespace(imageStreamDestinationNamespace), imageStremParts[0], imageStremParts[1])

	// Update the Kind
	patch = append(patch, patchOperation{
		Op:    "replace",
		Path:  "/spec/output/to/kind",
		Value: "DockerImage",
	})

	// Update the destination
	patch = append(patch, patchOperation{
		Op:    "replace",
		Path:  "/spec/output/to/name",
		Value: dockerImage,
	})

	// Add annotations to Build to for Build Controller to use
	patch = append(patch, patchOperation{
		Op:    "add",
		Path:  "/metadata/annotations/" + escapeJSONPointer(constants.BuildOperatorManagedAnnotation),
		Value: "true",
	})

	patch = append(patch, patchOperation{
		Op:    "add",
		Path:  "/metadata/annotations/" + escapeJSONPointer(constants.BuildDestinationImageStreamAnnotation),
		Value: fmt.Sprintf("%s/%s:%s", imageStreamDestinationNamespace, imageStremParts[0], imageStremParts[1]),
	})

	patchBytes, err := json.Marshal(patch)

	if err != nil {
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	return &v1beta1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *v1beta1.PatchType {
			pt := v1beta1.PatchTypeJSONPatch
			return &pt
		}(),
	}

}

func (wsvr *WebhookServer) getQuayIntegration(ar *v1beta1.AdmissionReview) (redhatcopv1alpha1.QuayIntegration, bool, error) {

	// Find the Current Registered QuayIntegration objects
	quayIntegrations := redhatcopv1alpha1.QuayIntegrationList{}

	err := wsvr.Client.List(context.TODO(), &client.ListOptions{}, &quayIntegrations)

	if err != nil {
		return redhatcopv1alpha1.QuayIntegration{}, false, err
	}

	if len(quayIntegrations.Items) != 1 {
		logging.Log.Info("No QuayIntegrations defined or more than 1 integration present")
		return redhatcopv1alpha1.QuayIntegration{}, false, nil
	}

	quayIntegration := *&quayIntegrations.Items[0]

	// Check is this is a valid namespace (TODO: Use a predicate to filter out?)
	validNamespace := quayIntegration.IsAllowedNamespace(ar.Request.Namespace)

	if !validNamespace {
		return redhatcopv1alpha1.QuayIntegration{}, false, nil
	}

	return quayIntegration, true, nil
}

func escapeJSONPointer(s string) string {
	esc := strings.Replace(s, "~", "~0", -1)
	esc = strings.Replace(esc, "/", "~1", -1)
	return esc
}
