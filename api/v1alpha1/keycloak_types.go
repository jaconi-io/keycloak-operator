package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KeycloakSpec defines the desired state of Keycloak
type KeycloakSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Keycloak. Edit keycloak_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// KeycloakStatus defines the observed state of Keycloak
type KeycloakStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Keycloak is the Schema for the keycloaks API
type Keycloak struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KeycloakSpec   `json:"spec,omitempty"`
	Status KeycloakStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KeycloakList contains a list of Keycloak
type KeycloakList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Keycloak `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Keycloak{}, &KeycloakList{})
}
