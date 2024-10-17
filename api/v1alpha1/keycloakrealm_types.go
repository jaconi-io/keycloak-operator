package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KeycloakRealmSpec defines the desired state of KeycloakRealm
type KeycloakRealmSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of KeycloakRealm. Edit keycloakrealm_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// KeycloakRealmStatus defines the observed state of KeycloakRealm
type KeycloakRealmStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KeycloakRealm is the Schema for the keycloakrealms API
type KeycloakRealm struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KeycloakRealmSpec   `json:"spec,omitempty"`
	Status KeycloakRealmStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KeycloakRealmList contains a list of KeycloakRealm
type KeycloakRealmList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KeycloakRealm `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KeycloakRealm{}, &KeycloakRealmList{})
}
