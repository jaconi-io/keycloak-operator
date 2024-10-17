package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	keycloakorgv1alpha1 "github.com/jaconi-io/keycloak-operator/api/v1alpha1"
)

// KeycloakClientReconciler reconciles a KeycloakClient object
type KeycloakClientReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=keycloak.org.jaconi.io,resources=keycloakclients,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=keycloak.org.jaconi.io,resources=keycloakclients/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=keycloak.org.jaconi.io,resources=keycloakclients/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the KeycloakClient object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *KeycloakClientReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KeycloakClientReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&keycloakorgv1alpha1.KeycloakClient{}).
		Complete(r)
}
