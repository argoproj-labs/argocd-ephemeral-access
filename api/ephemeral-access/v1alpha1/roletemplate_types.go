/*
Copyright 2024.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RoleTemplate is the Schema for the roletemplates API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type RoleTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RoleTemplateSpec   `json:"spec,omitempty"`
	Status RoleTemplateStatus `json:"status,omitempty"`
}

// RoleTemplateList contains a list of RoleTemplate
// +kubebuilder:object:root=true
type RoleTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RoleTemplate `json:"items"`
}

// RoleTemplateSpec defines the desired state of RoleTemplate
type RoleTemplateSpec struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Policies    []string `json:"policies"`
}

// RoleTemplateStatus defines the observed state of RoleTemplate
type RoleTemplateStatus struct {
	Synced   bool   `json:"synced"`
	Message  string `json:"message,omitempty"`
	SyncHash string `json:"syncHash"`
}

func init() {
	SchemeBuilder.Register(&RoleTemplate{}, &RoleTemplateList{})
}
