package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KeycloakBackupSpec defines the desired state of KeycloakBackup
type KeycloakBackupSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of KeycloakBackup. Edit keycloakbackup_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// KeycloakBackupStatus defines the observed state of KeycloakBackup
type KeycloakBackupStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KeycloakBackup is the Schema for the keycloakbackups API
type KeycloakBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KeycloakBackupSpec   `json:"spec,omitempty"`
	Status KeycloakBackupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KeycloakBackupList contains a list of KeycloakBackup
type KeycloakBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KeycloakBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KeycloakBackup{}, &KeycloakBackupList{})
}
