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

package v1

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// QuayIntegrationSpec defines the desired state of QuayIntegration
type QuayIntegrationSpec struct {

	// ClusterID refers to the ID associated with this cluster.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Cluster ID",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	// +kubebuilder:validation:Required
	ClusterID string `json:"clusterID"`

	// CredentialsSecret refers to the Secret containing credentials to communicate with the Quay registry.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Credentials secret",xDescriptors={"urn:alm:descriptor:io.kubernetes:Secret"}
	// +kubebuilder:validation:Required
	CredentialsSecret *SecretRef `json:"credentialsSecret"`

	// OrganizationPrefix is the prefix assigned to organizations.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Organization Prefix",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	// +kubebuilder:validation:Optional
	OrganizationPrefix string `json:"organizationPrefix,omitempty"`

	// QuayHostname is the hostname of the Quay registry.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Quay hostname",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	// +kubebuilder:validation:Required
	QuayHostname string `json:"quayHostname"`

	// InsecureRegistry refers to whether to skip TLS verification to the Quay registry.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Insecure Registry",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// +kubebuilder:validation:Optional
	InsecureRegistry bool `json:"insecureRegistry,omitempty"`

	// ScheduledImageStreamImport determines whether to enable import scheduling on all managed ImageStreams.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Schedule ImageStream Imports",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// +kubebuilder:validation:Optional
	ScheduledImageStreamImport bool `json:"scheduledImageStreamImport,omitempty"`

	// DenylistNamespaces is a list of namespaces to exclude.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="List of namespaces to exclude"
	// +kubebuilder:validation:Optional
	DenylistNamespaces []string `json:"denylistNamespaces,omitempty"`

	// AllowlistNamespaces is a list of namespaces to include
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="List of namespaces to include"
	// +kubebuilder:validation:Optional
	AllowlistNamespaces []string `json:"allowlistNamespaces,omitempty"`
}

// QuayIntegrationStatus defines the observed state of QuayIntegration
type QuayIntegrationStatus struct {

	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Conditions",xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Last Updated Time",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	LastUpdate string `json:"lastUpdate,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// QuayIntegration is the Schema for the quayintegrations API
// +kubebuilder:resource:path=quayintegrations,scope=Cluster
type QuayIntegration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QuayIntegrationSpec   `json:"spec,omitempty"`
	Status QuayIntegrationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// QuayIntegrationList contains a list of QuayIntegration
type QuayIntegrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QuayIntegration `json:"items"`
}

// SecretRef represents a reference to an item within a Secret
type SecretRef struct {

	// Name represents the name of the secret
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Name of the secret",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace represents the namespace containing the secret
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Namespace containing the secret",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`

	// Key represents the specific key to reference from the secret
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Key within the secret",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	// +kubebuilder:validation:Optional
	Key string `json:"key,omitempty"`
}

func (q *QuayIntegration) GetConditions() []metav1.Condition {
	return q.Status.Conditions
}

func (q *QuayIntegration) SetConditions(conditions []metav1.Condition) {
	q.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&QuayIntegration{}, &QuayIntegrationList{})
}

var (
	defaultDenylistNamespaces = map[string]string{
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
	for _, denylistNamespace := range qi.Spec.DenylistNamespaces {
		if namespace == denylistNamespace {
			return false
		}
	}

	for _, allowlistNamespace := range qi.Spec.AllowlistNamespaces {
		if namespace == allowlistNamespace {
			return true
		}
	}

	if _, ok := defaultDenylistNamespaces[namespace]; ok || strings.HasPrefix(namespace, "openshift-") || strings.HasPrefix(namespace, "kube-") {
		return false
	}

	return len(qi.Spec.AllowlistNamespaces) == 0
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
