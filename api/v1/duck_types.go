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

// DuckSpec defines the desired state of Duck.
type DuckSpec struct {
	// Group to read the target Duck.
	Group string `json:"group"`
	// Version to read the target Duck.
	Version string `json:"version"`
	// Kind to read the target Duck.
	Kind string `json:"kind"`
}

// +die

// DuckStatus defines the observed state of Duck.
type DuckStatus struct {
	apis.Status `json:",inline"`
}

// +kubebuilder:object:root=true
// +die:object=true

// Duck is the Schema for the ducks API.
type Duck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DuckSpec   `json:"spec,omitempty"`
	Status DuckStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DuckList contains a list of Duck.
type DuckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Duck `json:"items"`
}
