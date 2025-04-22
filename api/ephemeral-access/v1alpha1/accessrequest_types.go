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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Status defines the different stages a given access request can be
// at a given time.
// +kubebuilder:validation:Enum=initiated;requested;granted;expired;denied;invalid
type Status string

const (
	// InitiatedStatus is the stage that defines the access request as initiated
	InitiatedStatus Status = "initiated"

	// RequestedStatus is the stage that defines the access request as pending
	RequestedStatus Status = "requested"

	// GrantedStatus is the stage that defines the access request as granted
	GrantedStatus Status = "granted"

	// ExpiredStatus is the stage that defines the access request as expired
	ExpiredStatus Status = "expired"

	// DeniedStatus is the stage that defines the access request as refused
	DeniedStatus Status = "denied"

	// InvalidStatus is the used to identify invalid access requests
	InvalidStatus Status = "invalid"
)

// AccessRequestSpec defines the desired state of AccessRequest
type AccessRequestSpec struct {
	// Duration defines the ammount of time that the elevated access
	// will be granted once approved
	Duration metav1.Duration `json:"duration"`
	// TargetRoleName defines the role name the user will be assigned
	// to once the access is approved
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Role TargetRole `json:"role"`
	// Application defines the Argo CD Application to assign the elevated
	// permission
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Application TargetApplication `json:"application"`
	// Subject defines the subject for this access request
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Subject Subject `json:"subject"`
}

// TargetApplication defines the Argo CD AppProject to assign the elevated permission
type TargetApplication struct {
	// Name refers to the Argo CD Application name
	Name string `json:"name"`
	// Namespace refers to the namespace where the Argo CD Application lives
	Namespace string `json:"namespace"`
}

// TargetRole defines the role that is requested
type TargetRole struct {
	// TemplateName defines the role template the user will be assigned
	// +kubebuilder:validation:Required
	TemplateRef TargetRoleTemplate `json:"templateRef"`
	// Ordinal defines an ordering number of this role compared to others
	Ordinal int `json:"ordinal,omitempty"`
	// FriendlyName defines a name for this role
	// +kubebuilder:validation:MaxLength=512
	FriendlyName *string `json:"friendlyName,omitempty"`
}

// TargetRoleTemplate defines the reference to the RoleTemplate to be associated
// with the AccessRequest
type TargetRoleTemplate struct {
	// Name refers to the RoleTemplate name
	// +kubebuilder:validation:MaxLength=512
	Name string `json:"name"`
	// Namespace refers to the namespace where the RoleTemplate lives
	Namespace string `json:"namespace"`
}

// Subject defines the user details to get elevated permissions assigned
type Subject struct {
	// Username refers to the entity requesting the elevated permission
	Username string `json:"username"`
}

// AccessRequestStatus defines the observed state of AccessRequest
type AccessRequestStatus struct {
	RequestState     Status                 `json:"requestState,omitempty"`
	TargetProject    string                 `json:"targetProject,omitempty"`
	ExpiresAt        *metav1.Time           `json:"expiresAt,omitempty"`
	RoleTemplateHash string                 `json:"roleTemplateHash,omitempty"`
	RoleName         string                 `json:"roleName,omitempty"`
	History          []AccessRequestHistory `json:"history,omitempty"`
}

// AccessRequestHistory contain the history of all status transitions associated
// with this access request
type AccessRequestHistory struct {
	// TransitionTime is the time the transition is observed
	TransitionTime metav1.Time `json:"transitionTime"`
	// RequestState is the new status assigned to this access request
	RequestState Status `json:"status"`
	// Details may contain detailed information about the transition
	Details *string `json:"details,omitempty"`
}

func (h AccessRequestHistory) String() string {
	details := ""
	if h.Details != nil {
		details = *h.Details
	}
	return fmt.Sprintf("{TransitionTime: %s, RequestState: %s, Details: %s }", h.TransitionTime.String(), h.RequestState, details)
}

// AccessRequest is the Schema for the accessrequests API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=areq;areqs
type AccessRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AccessRequestSpec   `json:"spec,omitempty"`
	Status AccessRequestStatus `json:"status,omitempty"`
}

// UpdateStatusHistory will update this AccessRequest status and history fields
// based on the given status and details. This function should only depend on the
// objects provided by this package. If any additional dependency is needed
// than this function should be moved to another package.
func (ar *AccessRequest) UpdateStatusHistory(newStatus Status, details string) {
	status := ar.Status.DeepCopy()
	status.RequestState = newStatus

	// set the expiresAt only when transitioning to GrantedStatus
	if newStatus == GrantedStatus && status.ExpiresAt == nil {
		expiresAt := metav1.NewTime(time.Now().Add(ar.Spec.Duration.Duration))
		status.ExpiresAt = &expiresAt
	}

	var detailsPtr *string
	if details != "" {
		detailsPtr = &details
	}
	history := AccessRequestHistory{
		TransitionTime: metav1.Now(),
		RequestState:   newStatus,
		Details:        detailsPtr,
	}
	status.History = append(status.History, history)
	ar.Status = *status
}

// GetLastStatusDetails will return the last recorded details message in the
// history associated with the given status.
func (ar *AccessRequest) GetLastStatusDetails(status Status) string {
	for i := len(ar.Status.History) - 1; i >= 0; i-- {
		if ar.Status.History[i].RequestState == status {
			msg := ""
			if ar.Status.History[i].Details != nil {
				msg = *ar.Status.History[i].Details
			}
			return msg
		}
	}
	return ""
}

// IsExpiring will return true if this AccessRequest is expired by
// verifying the .status.ExpiresAt field. Otherwise it returns false.
func (ar *AccessRequest) IsExpiring() bool {
	if ar.Status.ExpiresAt != nil &&
		ar.Status.ExpiresAt.Time.Before(time.Now()) {
		return true
	}
	return false
}

// AccessRequestList contains a list of AccessRequest
// +kubebuilder:object:root=true
type AccessRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AccessRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AccessRequest{}, &AccessRequestList{})
}
