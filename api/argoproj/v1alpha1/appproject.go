package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AppProject provides a logical grouping of applications, providing controls for:
// * who can access these applications (roles, OIDC group claims bindings)
// * and what they can do (RBAC policies)
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=appprojects,shortName=appproj;appprojs
type AppProject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Spec              AppProjectSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
}

// AppProjectSpec is the specification of an AppProject
type AppProjectSpec struct {
	// Roles are user defined RBAC roles associated with this project
	Roles []ProjectRole `json:"roles,omitempty" protobuf:"bytes,1,rep,name=roles"`
}

// ProjectRole represents a role that has access to a project
type ProjectRole struct {
	// Name is a name for this role
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Description is a description of the role
	Description string `json:"description,omitempty" protobuf:"bytes,2,opt,name=description"`
	// Policies Stores a list of casbin formatted strings that define access policies for the role in the project
	Policies []string `json:"policies,omitempty" protobuf:"bytes,3,rep,name=policies"`
	// JWTTokens are a list of generated JWT tokens bound to this role
	JWTTokens []JWTToken `json:"jwtTokens,omitempty" protobuf:"bytes,4,rep,name=jwtTokens"`
	// Groups are a list of OIDC group claims bound to this role
	Groups []string `json:"groups,omitempty" protobuf:"bytes,5,rep,name=groups"`
}

// JWTToken holds the issuedAt and expiresAt values of a token
type JWTToken struct {
	IssuedAt  int64  `json:"iat" protobuf:"int64,1,opt,name=iat"`
	ExpiresAt int64  `json:"exp,omitempty" protobuf:"int64,2,opt,name=exp"`
	ID        string `json:"id,omitempty" protobuf:"bytes,3,opt,name=id"`
}

// AccessRequestList contains a list of AccessRequest
// +kubebuilder:object:root=true
type AppProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppProject `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AppProject{}, &AppProjectList{})
}
