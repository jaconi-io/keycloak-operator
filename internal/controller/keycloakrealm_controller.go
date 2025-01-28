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
	"github.com/jaconi-io/keycloak-operator/internal/controller/keycloakrealm"
	"github.com/pkg/errors"
)

const (
	RealmFinalizer = "realm.cleanup"
)

// KeycloakRealmReconciler reconciles a KeycloakRealm object
type KeycloakRealmReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=keycloak.org,resources=keycloakrealms,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keycloak.org,resources=keycloakrealms/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=keycloak.org,resources=keycloakrealms/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the KeycloakRealm object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *KeycloakRealmReconciler) Reconcile(ctx context.Context, realm *keycloakorgv1alpha1.KeycloakRealm) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling KeycloakRealm")

	if realm.Spec.Unmanaged {
		return reconcile.Result{Requeue: false}, r.manageSuccess(ctx, realm, realm.DeletionTimestamp != nil)
	}

	// If no selector is set we can't figure out which Keycloak instance this realm should
	// be added to. Skip reconcile until a selector has been set.
	if realm.Spec.InstanceSelector == nil {
		logger.Info(fmt.Sprintf("realm %v/%v has no instance selector and will be ignored", realm.Namespace, realm.Name))
		return reconcile.Result{Requeue: false}, nil
	}

	keycloaks, err := common.GetMatchingKeycloaks(ctx, r.Client, realm.Spec.InstanceSelector)
	if err != nil {
		return r.ManageError(ctx, realm, err)
	}

	logger.Info(fmt.Sprintf("found %v matching keycloak(s) for realm %v/%v", len(keycloaks.Items), realm.Namespace, realm.Name))

	// The realm may be applicable to multiple keycloak instances,
	// process all of them
	for _, keycloak := range keycloaks.Items {
		// Get an authenticated keycloak api client for the instance
		keycloakFactory := common.LocalConfigKeycloakFactory{}

		if keycloak.Spec.Unmanaged {
			return r.ManageError(ctx, realm, errors.New("realms cannot be created for unmanaged keycloak instances"))
		}

		authenticated, err := keycloakFactory.AuthenticatedClient(keycloak, false)

		if err != nil {
			return r.ManageError(ctx, realm, err)
		}

		// Compute the current state of the realm
		realmState := common.NewRealmState(ctx, keycloak)

		logger.Info(fmt.Sprintf("read state for keycloak %v/%v, realm %v/%v",
			keycloak.Namespace,
			keycloak.Name,
			realm.Namespace,
			realm.Spec.Realm.Realm))

		err = realmState.Read(realm, authenticated, r.Client)
		if err != nil {
			return r.ManageError(ctx, realm, err)
		}

		// Figure out the actions to keep the realms up to date with
		// the desired state
		reconciler := keycloakrealm.NewKeycloakRealmReconciler(keycloak)
		desiredState := reconciler.Reconcile(realmState, realm)
		actionRunner := common.NewClusterAndKeycloakActionRunner(ctx, r.Client, r.Scheme, realm, authenticated)

		// Run all actions to keep the realms updated
		err = actionRunner.RunAll(desiredState)
		if err != nil {
			return r.ManageError(ctx, realm, err)
		}
	}

	return ctrl.Result{}, r.manageSuccess(ctx, realm, realm.DeletionTimestamp != nil)
}

func (r *KeycloakRealmReconciler) manageSuccess(ctx context.Context, realm *keycloakorgv1alpha1.KeycloakRealm, deleted bool) error {
	logger := log.FromContext(ctx)

	realm.Status.Ready = true
	realm.Status.Message = ""
	realm.Status.Phase = keycloakorgv1alpha1.PhaseReconciling

	err := r.Client.Status().Update(ctx, realm)
	if err != nil {
		logger.Error(err, "unable to update status")
	}

	// Finalizer already set?
	finalizerExists := false
	for _, finalizer := range realm.Finalizers {
		if finalizer == RealmFinalizer {
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
		realm.Finalizers = append(realm.Finalizers, RealmFinalizer)
		logger.Info(fmt.Sprintf("added finalizer to keycloak realm %v/%v",
			realm.Namespace,
			realm.Spec.Realm.Realm))

		return r.Client.Update(ctx, realm)
	}

	// Otherwise remove the finalizer
	newFinalizers := []string{}
	for _, finalizer := range realm.Finalizers {
		if finalizer == RealmFinalizer {
			logger.Info(fmt.Sprintf("removed finalizer from keycloak realm %v/%v",
				realm.Namespace,
				realm.Spec.Realm.Realm))

			continue
		}
		newFinalizers = append(newFinalizers, finalizer)
	}

	realm.Finalizers = newFinalizers
	return r.Client.Update(ctx, realm)
}

func (r *KeycloakRealmReconciler) ManageError(ctx context.Context, realm *keycloakorgv1alpha1.KeycloakRealm, err error) (reconcile.Result, error) {
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
func (r *KeycloakRealmReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&keycloakorgv1alpha1.KeycloakRealm{}).
		Watches(&corev1.Secret{}, handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &keycloakorgv1alpha1.KeycloakRealm{}, handler.OnlyControllerOwner())).
		Named("keycloakrealm").
		Complete(reconcile.AsReconciler(mgr.GetClient(), r))
}
