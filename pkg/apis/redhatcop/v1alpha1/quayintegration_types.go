package v1alpha1

import (
	"fmt"
	"strings"

	"github.com/redhat-cop/quay-openshift-registry-operator/pkg/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// QuayIntegrationSpec defines the desired state of QuayIntegration
// +k8s:openapi-gen=true
type QuayIntegrationSpec struct {
	ClusterID             string   `json:"clusterID"`
	CredentialsSecretName string   `json:"credentialsSecretName"`
	OrganizationPrefix    string   `json:"organizationPrefix,omitempty"`
	QuayHostname          string   `json:"quayHostname"`
	BlacklistNamespaces   []string `json:"blacklistNamespaces,omitempty"`
	WhitelistNamespaces   []string `json:"whitelistNamespaces,omitempty"`
}

// QuayIntegrationStatus defines the observed state of QuayIntegration
// +k8s:openapi-gen=true
type QuayIntegrationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QuayIntegration is the Schema for the quayintegrations API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type QuayIntegration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QuayIntegrationSpec   `json:"spec,omitempty"`
	Status QuayIntegrationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QuayIntegrationList contains a list of QuayIntegration
type QuayIntegrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QuayIntegration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QuayIntegration{}, &QuayIntegrationList{})
}

const ()

var (
	defaultBlacklistNamespaces = []string{
		"default",
		"kube-public",
		"kube-service-catalog",
		"kube-system",
		"management-infra",
		"openshift",
		"openshift-ansible-service-broker",
		"openshift-console",
		"openshift-infra",
		"openshift-logging",
		"openshift-monitoring",
		"openshift-node",
		"openshift-sdn",
		"openshift-template-service-broker",
		"openshift-web-console",
	}
)

func (qi *QuayIntegration) GenerateQuayOrganizationNameFromNamespace(namespace string) string {
	return fmt.Sprintf("%s_%s", strings.ToLower(qi.Spec.ClusterID), namespace)
}

func (qi *QuayIntegration) IsAllowedNamespace(namespace string) bool {

	// Add blacklist namespaces
	combinedLists := append(defaultBlacklistNamespaces, qi.Spec.BlacklistNamespaces...)

	// Remove items in whitelist namespaces
	combinedLists = utils.RemoveItemsFromSlice(combinedLists, qi.Spec.WhitelistNamespaces)

	// check to see if value exists
	for _, blacklistNamespace := range combinedLists {
		if namespace == blacklistNamespace {
			return false
		}
	}

	return true
}
