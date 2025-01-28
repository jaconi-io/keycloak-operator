/*
Copyright 2025.

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

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	keycloakorgv1alpha1 "github.com/jaconi-io/keycloak-operator/api/v1alpha1"
	"github.com/jaconi-io/keycloak-operator/internal/common"
	"github.com/jaconi-io/keycloak-operator/internal/controller/keycloakclient"
)

const ClientFinalizer = "client.cleanup"

// KeycloakClientReconciler reconciles a KeycloakClient object
type KeycloakClientReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=keycloak.org,resources=keycloakclients,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keycloak.org,resources=keycloakclients/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=keycloak.org,resources=keycloakclients/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the KeycloakClient object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *KeycloakClientReconciler) Reconcile(ctx context.Context, client *keycloakorgv1alpha1.KeycloakClient) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling KeycloakClient")

	r.adjustCrDefaults(client)

	// The client may be applicable to multiple keycloak instances,
	// process all of them
	realms, err := common.GetMatchingRealms(ctx, r.Client, client.Spec.RealmSelector)
	if err != nil {
		return r.ManageError(ctx, client, err)
	}
	logger.Info(fmt.Sprintf("found %v matching realm(s) for client %v/%v", len(realms.Items), client.Namespace, client.Name))
	for _, realm := range realms.Items {
		keycloaks, err := common.GetMatchingKeycloaks(ctx, r.Client, realm.Spec.InstanceSelector)
		if err != nil {
			return r.ManageError(ctx, client, err)
		}
		logger.Info(fmt.Sprintf("found %v matching keycloak(s) for realm %v/%v", len(keycloaks.Items), realm.Namespace, realm.Name))

		for _, keycloak := range keycloaks.Items {
			// Get an authenticated keycloak api client for the instance
			keycloakFactory := common.LocalConfigKeycloakFactory{}
			authenticated, err := keycloakFactory.AuthenticatedClient(keycloak, false)
			if err != nil {
				return r.ManageError(ctx, client, err)
			}

			// Compute the current state of the realm
			logger.Info(fmt.Sprintf("got authenticated client for keycloak at %v", authenticated.Endpoint()))
			clientState := common.NewClientState(ctx, realm.DeepCopy(), keycloak)

			logger.Info(fmt.Sprintf("read client state for keycloak %v/%v, realm %v/%v, client %v/%v",
				keycloak.Namespace,
				keycloak.Name,
				realm.Namespace,
				realm.Name,
				client.Namespace,
				client.Name))

			err = clientState.Read(ctx, client, authenticated, r.Client)
			if err != nil {
				return r.ManageError(ctx, client, err)
			}

			// Figure out the actions to keep the realms up to date with
			// the desired state
			reconciler := keycloakclient.NewKeycloakClientReconciler(keycloak)
			desiredState := reconciler.Reconcile(clientState, client)
			actionRunner := common.NewClusterAndKeycloakActionRunner(ctx, r.Client, r.Scheme, client, authenticated)

			// Run all actions to keep the realms updated
			err = actionRunner.RunAll(desiredState)
			if err != nil {
				return r.ManageError(ctx, client, err)
			}
		}
	}

	return ctrl.Result{}, r.manageSuccess(ctx, client, client.DeletionTimestamp != nil)
}

// Fills the CR with default values. Nils are not acceptable for Kubernetes.
func (r *KeycloakClientReconciler) adjustCrDefaults(cr *keycloakorgv1alpha1.KeycloakClient) {
	if cr.Spec.Client.Attributes == nil {
		cr.Spec.Client.Attributes = make(map[string]string)
	}
	if cr.Spec.Client.Access == nil {
		cr.Spec.Client.Access = make(map[string]bool)
	}
	if cr.Spec.Client.AuthenticationFlowBindingOverrides == nil {
		cr.Spec.Client.AuthenticationFlowBindingOverrides = make(map[string]string)
	}
}

func (r *KeycloakClientReconciler) manageSuccess(ctx context.Context, client *keycloakorgv1alpha1.KeycloakClient, deleted bool) error {
	logger := log.FromContext(ctx)

	client.Status.Ready = true
	client.Status.Message = ""
	client.Status.Phase = keycloakorgv1alpha1.PhaseReconciling

	err := r.Client.Status().Update(ctx, client)
	if err != nil {
		logger.Error(err, "unable to update status")
	}

	// Finalizer already set?
	finalizerExists := false
	for _, finalizer := range client.Finalizers {
		if finalizer == ClientFinalizer {
			finalizerExists = true
			break
		}
	}

	// Resource created and finalizer exists: nothing to do
	if !deleted && finalizerExists {
		return nil
	}

	// Resource created and finalizer does not exist: add finalizer
	if !deleted && !finalizerExists {
		client.Finalizers = append(client.Finalizers, ClientFinalizer)
		logger.Info(fmt.Sprintf("added finalizer to keycloak client %v/%v",
			client.Namespace,
			client.Spec.Client.ClientID))

		return r.Client.Update(ctx, client)
	}

	// Otherwise remove the finalizer
	newFinalizers := []string{}
	for _, finalizer := range client.Finalizers {
		if finalizer == ClientFinalizer {
			logger.Info(fmt.Sprintf("removed finalizer from keycloak client %v/%v",
				client.Namespace,
				client.Spec.Client.ClientID))

			continue
		}
		newFinalizers = append(newFinalizers, finalizer)
	}

	client.Finalizers = newFinalizers
	return r.Client.Update(ctx, client)
}

func (r *KeycloakClientReconciler) ManageError(ctx context.Context, realm *keycloakorgv1alpha1.KeycloakClient, err error) (reconcile.Result, error) {
	logger := log.FromContext(ctx)
	r.Recorder.Event(realm, "Warning", "ProcessingError", err.Error())

	realm.Status.Message = err.Error()
	realm.Status.Ready = false
	realm.Status.Phase = keycloakorgv1alpha1.PhaseFailing

	if err := r.Client.Status().Update(ctx, realm); err != nil {
		logger.Error(err, "unable to update status")
	}

	return reconcile.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *KeycloakClientReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&keycloakorgv1alpha1.KeycloakClient{}).
		Watches(&corev1.Secret{}, handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &keycloakorgv1alpha1.KeycloakClient{}, handler.OnlyControllerOwner())).
		Named("keycloakclient").
		Complete(reconcile.AsReconciler(mgr.GetClient(), r))
}
