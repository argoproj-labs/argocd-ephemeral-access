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

	"github.com/expr-lang/expr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// AccessBinding is the Schema for the accessbindings API
// +kubebuilder:object:root=true
type AccessBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AccessBindingSpec `json:"spec,omitempty"`
}

// AccessBindingList contains a list of AccessBinding
// +kubebuilder:object:root=true
type AccessBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []AccessBinding `json:"items"`
}

// AccessBindingSpec defines the desired state of AccessBinding
type AccessBindingSpec struct {
	// RoleTemplateRef is the reference to the RoleTemplate this bindings grants
	// access to
	// +kubebuilder:validation:Required
	RoleTemplateRef RoleTemplateReference `json:"roleTemplateRef"`
	// Subjects is list of strings, supporting go template, that a user's group
	// claims must match at least one of to be allowed
	Subjects []string `json:"subjects"`
	// If is a condition that must be true to evaluate the subjects
	If *string `json:"if,omitempty"`
	// Ordinal defines an ordering number of this role compared to others.
	// AccessBindings associated with roles with higher privilege should
	// be set with lower ordinal value than AccessBindings associated with
	// roles with lesser privilege.
	Ordinal int `json:"ordinal,omitempty"`
	// FriendlyName defines a name for this role
	// +kubebuilder:validation:MaxLength=512
	FriendlyName *string `json:"friendlyName,omitempty"`
}

// RoleTemplateReference is a reference to a RoleTemplate
type RoleTemplateReference struct {
	// Name of the role template object
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// RenderSubjects renders the access bindings subjects when the If condition is evaluated to true
func (ab *AccessBinding) RenderSubjects(app, project *unstructured.Unstructured) ([]string, error) {
	if len(ab.Spec.Subjects) == 0 {
		return nil, nil
	}

	values := map[string]interface{}{
		"app":         app.Object,
		"application": app.Object,
		"project":     project.Object,
	}

	if ab.Spec.If != nil {
		out, err := expr.Eval(*ab.Spec.If, values)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate binding condition '%s': %w", *ab.Spec.If, err)
		}
		switch condResult := out.(type) {
		case bool:
			if !condResult {
				// No need to render template, condition is false
				return nil, nil
			}
		default:
			return nil, fmt.Errorf("binding condition '%s' evaluated to non-boolean value", *ab.Spec.If)
		}
	}

	subStr := strings.Join(ab.Spec.Subjects, "\n")
	subTmpl, err := template.New("subjects").Parse(subStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing AccessBinding subjects: %w", err)
	}
	p, err := ab.execTemplate(subTmpl, values)
	if err != nil {
		return nil, fmt.Errorf("error rendering AccessBinding subjects: %w", err)
	}
	subjects := strings.Split(p, "\n")

	return subjects, nil
}

func (ab *AccessBinding) execTemplate(
	tmpl *template.Template,
	values any,
) (string, error) {
	var s strings.Builder
	err := tmpl.Execute(&s, values)
	if err != nil {
		return "", err
	}
	return s.String(), nil
}

func init() {
	SchemeBuilder.Register(&AccessBinding{}, &AccessBindingList{})
}
