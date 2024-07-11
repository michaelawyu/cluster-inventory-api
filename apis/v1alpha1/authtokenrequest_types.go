package v1alpha1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced

// AuthTokenRequest represents a request for access token in a multi-cluster environment.
type AuthTokenRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec AuthTokenRequestSpec `json:"spec"`

	// +optional
	Status AuthTokenRequestStatus `json:"status,omitempty"`
}

// AuthTokenRequestSpec specifies the spec of an AuthTokenRequest object.
//
// For simiplicity reasons, the current design assumes that:
//   - the referenced service account, roles, and cluster roles are guaranteed to be non-existent
//     in the target cluster (that is, for now we disregard the scenario where some service accounts,
//     roles, cluster roles have already existed in the cluster and the application is simply requesting
//     a token to be created or some bindings to be made).
//   - no rotation is necessary.
//
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.roles) || has(self.roles)", message="Roles is required once set"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.clusterRoles) || has(self.clusterRoles)", message="ClusterRoles is required once set"
type AuthTokenRequestSpec struct {
	// TargetClusterProfile is the cluster profile that the access token is requested for.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="TargetClusterProfile is immutable"
	TargetClusterProfile ClusterProfileRef `json:"targetClusterProfile"`

	// ServiceAccountName is the name of the service account that the
	// access token should be associated with.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="ServiceAccountName is immutable"
	// +kubebuilder:validation:MaxLength=63
	ServiceAccountName string `json:"serviceAccountName"`

	// Roles is a list of roles that is associated with the service account.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=20
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Roles is immutable"
	Roles []Role `json:"roles"`

	// ClusterRoleRules is a list of cluster roles that is associated with the service account.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=20
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="ClusterRoles is immutable"
	ClusterRoles []ClusterRole `json:"clusterRoles"`
}

// Role describes a set of permissions that should be set under a specific namespace.
type Role struct {
	// Namespace is the namespace where the set of permissions is applied.
	// The namespace will be created if it does not already exist.
	// +required
	Namespace string `json:"namespace"`

	// Name is the name of the role that should be created.
	// +required
	Name string `json:"name"`

	// Rules is a list of policies for the resources in the specified namespace.
	// +optional
	// +listType=atomic
	Rules []rbacv1.PolicyRule `json:"rules"`
}

// ClusterRole describes a set of permissions that should be set under the cluster scope.
type ClusterRole struct {
	// Name is the name of the cluster role that should be created.
	// +required
	Name string `json:"name"`

	// Rules is a list of policies for the resources in the cluster scope.
	// +optional
	// +listType=atomic
	Rules []rbacv1.PolicyRule `json:"rules"`
}

// ClusterProfileRef points to a specific cluster profile.
// +structType=atomic
type ClusterProfileRef struct {
	// APIGroup is the API group of the referred cluster profile object.
	APIGroup string `json:"apiGroup"`

	// Kind is the kind of the referred cluster profile object.
	Kind string `json:"kind"`

	// Name is the name of the referred cluster profile object.
	Name string `json:"name"`

	// Namespace is the namespace of the referred cluster profile object.
	Namespace string `json:"namespace"`
}

// AuthTokenRequestStatus specifies the status of an AuthTokenRequest object.
type AuthTokenRequestStatus struct {
	// +optional
	TokenResponse ConfigMapRef `json:"tokenResponse"`

	// Conditions is an array of conditions for the token request.
	// +optional
	Conditions []metav1.Condition `json:"conditions"`
}

// ConfigMapRef points to a specific ConfigMap object.
//
// Note that for security reasons, the token response object (i.e., the config map) is
// always kept in the same namespace as the token request object.
// +structType=atomic
type ConfigMapRef struct {
	// APIGroup is the API group of the referred config map object.
	APIGroup string `json:"apiGroup"`

	// Kind is the kind of the referred config map object.
	Kind string `json:"kind"`

	// Name is the name of the referred config map object.
	Name string `json:"name"`
}

//+kubebuilder:object:root=true

// AuthTokenRequestList contains a list of AuthTokenRequests.
type AuthTokenRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AuthTokenRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AuthTokenRequest{}, &AuthTokenRequestList{})
}
