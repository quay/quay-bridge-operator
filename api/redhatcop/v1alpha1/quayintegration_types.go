package v1alpha1

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// QuayIntegrationSpec defines the desired state of QuayIntegration
// +k8s:openapi-gen=true
type QuayIntegrationSpec struct {
	ClusterID                  string   `json:"clusterID"`
	CredentialsSecretName      string   `json:"credentialsSecretName"`
	OrganizationPrefix         string   `json:"organizationPrefix,omitempty"`
	QuayHostname               string   `json:"quayHostname"`
	InsecureRegistry           bool     `json:"insecureRegistry,omitempty"`
	ScheduledImageStreamImport bool     `json:"scheduledImageStreamImport,omitempty"`
	BlacklistNamespaces        []string `json:"blacklistNamespaces,omitempty"`
	WhitelistNamespaces        []string `json:"whitelistNamespaces,omitempty"`
}

// QuayIntegrationStatus defines the observed state of QuayIntegration
// +k8s:openapi-gen=true
type QuayIntegrationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
	LastUpdate string `json:"lastUpdate,omitempty"`
}

// +kubebuilder:object:root=true

// QuayIntegration is the Schema for the quayintegrations API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type QuayIntegration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QuayIntegrationSpec   `json:"spec,omitempty"`
	Status QuayIntegrationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// QuayIntegrationList contains a list of QuayIntegration
type QuayIntegrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QuayIntegration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QuayIntegration{}, &QuayIntegrationList{})
}

var (
	defaultBlacklistNamespaces = map[string]string{
		"default":          "default",
		"openshift":        "openshift",
		"management-infra": "management-infra",
	}
)

func (qi *QuayIntegration) GenerateQuayOrganizationNameFromNamespace(namespace string) string {
	return fmt.Sprintf("%s_%s", strings.ToLower(qi.Spec.ClusterID), namespace)
}

// IsAllowedNamespace returns whether a namespace is allowed to be managed.
func (qi *QuayIntegration) IsAllowedNamespace(namespace string) bool {
	for _, blacklistNamespace := range qi.Spec.BlacklistNamespaces {
		if namespace == blacklistNamespace {
			return false
		}
	}

	for _, whitelistNamespace := range qi.Spec.WhitelistNamespaces {
		if namespace == whitelistNamespace {
			return true
		}
	}

	if _, ok := defaultBlacklistNamespaces[namespace]; ok || strings.HasPrefix(namespace, "openshift-") || strings.HasPrefix(namespace, "kube-") {
		return false
	}

	return len(qi.Spec.WhitelistNamespaces) == 0
}

func (qi *QuayIntegration) GetRegistryHostname() (string, error) {
	quayURL, err := url.Parse(qi.Spec.QuayHostname)

	if err != nil {
		return "", err
	}

	return quayURL.Host, nil
}

func (qi *QuayIntegration) SetStatus(status *QuayIntegrationStatus) (*QuayIntegration, error) {
	qi.Status.LastUpdate = time.Now().UTC().String()

	return qi, nil
}
