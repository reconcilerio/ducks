/*
Copyright 2025 the original author or authors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reconciler.io/runtime/apis"
)

// +die

// DuckTypeSpec defines the desired state of DuckType.
type DuckTypeSpec struct {
	// Group is the API group of the defined custom resource.
	// Must match the name of the DuckType (in the form `<plural>.<group>`).
	Group string `json:"group"`
	// Plural is the plural name of the resource to serve.
	// Must match the name of the DuckType (in the form `<plural>.<group>`).
	// Must be all lowercase.
	Plural string `json:"plural"`
	// Singular is the singular name of the resource. It must be all lowercase. Defaults to lowercased `kind`.
	// +optional
	Singular string `json:"singular,omitempty"`
	// Kind is the serialized kind of the resource. It is normally CamelCase and singular.
	// Custom resource instances will use this value as the `kind` attribute in API calls.
	Kind string `json:"kind"`
	// ListKind is the serialized kind of the list for this resource. Defaults to "<kind>List".
	// +optional
	ListKind string `json:"listKind,omitempty"`
}

// +die

// DuckTypeStatus defines the observed state of DuckType.
type DuckTypeStatus struct {
	apis.Status `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +die:object=true,apiVersion=duck.reconciler.io/v1,kind=DuckType

// DuckType is the Schema for the ducktypes API.
type DuckType struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DuckTypeSpec   `json:"spec,omitempty"`
	Status DuckTypeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DuckTypeList contains a list of DuckType.
type DuckTypeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DuckType `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DuckType{}, &DuckTypeList{})
}
