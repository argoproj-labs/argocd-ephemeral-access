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
	"fmt"
	"strings"
	"text/template"

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

// Render will return a new RoleTemplate instance with the templates replaced by
// the given projName, appName and appNs. The RoleTemplate fields that accept
// templated values are 'rt.Spec.Description' and 'rt.Spec.Policies'.
func (rt *RoleTemplate) Render(projName, appName, appNs string) (*RoleTemplate, error) {
	rendered := rt.DeepCopy()
	descTmpl, err := template.New("description").Parse(rt.Spec.Description)
	if err != nil {
		return nil, fmt.Errorf("error parsing RoleTemplate description: %w", err)
	}
	desc, err := rt.execTemplate(descTmpl, projName, appName, appNs)
	if err != nil {
		return nil, fmt.Errorf("error rendering RoleTemplate description: %w", err)
	}
	rendered.Spec.Description = desc

	policiesStr := strings.Join(rt.Spec.Policies, "\n")
	policiesTmpl, err := template.New("policies").Parse(policiesStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing RoleTemplate policies: %w", err)
	}
	p, err := rt.execTemplate(policiesTmpl, projName, appName, appNs)
	if err != nil {
		return nil, fmt.Errorf("error rendering RoleTemplate policies: %w", err)
	}
	rendered.Spec.Policies = strings.Split(p, "\n")

	return rendered, nil
}

func (rt *RoleTemplate) execTemplate(tmpl *template.Template, projName, appName, appNs string) (string, error) {
	roleName := rt.AppProjectRoleName(appName, appNs)
	vars := map[string]string{
		"role":        fmt.Sprintf("proj:%s:%s", projName, roleName),
		"project":     projName,
		"application": appName,
		"namespace":   appNs,
	}
	var s strings.Builder
	err := tmpl.Execute(&s, vars)
	if err != nil {
		return "", err
	}
	return s.String(), nil
}

// roleName will return the role name to be used in the AppProject
func (rt *RoleTemplate) AppProjectRoleName(appName, namespace string) string {
	roleName := rt.Spec.Name
	return fmt.Sprintf("ephemeral-%s-%s-%s", roleName, namespace, appName)
}

func init() {
	SchemeBuilder.Register(&RoleTemplate{}, &RoleTemplateList{})
}
