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
	"errors"
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
	"github.com/jaconi-io/keycloak-operator/internal/controller/keycloakuser"
)

// KeycloakUserReconciler reconciles a KeycloakUser object
type KeycloakUserReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=keycloak.org,resources=keycloakusers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keycloak.org,resources=keycloakusers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=keycloak.org,resources=keycloakusers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the KeycloakUser object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *KeycloakUserReconciler) Reconcile(ctx context.Context, user *keycloakorgv1alpha1.KeycloakUser) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling KeycloakUser")

	// If no selector is set we can't figure out which realm instance this user should be added to. Skip reconcile until
	// a selector has been set.
	if user.Spec.RealmSelector == nil {
		logger.Info(fmt.Sprintf("user %v/%v has no realm selector and will be ignored", user.Namespace, user.Name))
		return reconcile.Result{}, nil
	}

	// Find the realms that this user should be added to based on the label selector.
	realms, err := common.GetMatchingRealms(ctx, r.Client, user.Spec.RealmSelector)
	if err != nil {
		return reconcile.Result{}, err
	}

	logger.Info(fmt.Sprintf("found %v matching realm(s) for user %v/%v", len(realms.Items), user.Namespace, user.Name))

	for _, realm := range realms.Items {
		if realm.Spec.Unmanaged {
			return r.ManageError(ctx, user, errors.New("users cannot be created for unmanaged keycloak realms"))
		}

		keycloaks, err := common.GetMatchingKeycloaks(ctx, r.Client, realm.Spec.InstanceSelector)
		if err != nil {
			return r.ManageError(ctx, user, err)
		}

		for _, keycloak := range keycloaks.Items {
			if keycloak.Spec.Unmanaged {
				return r.ManageError(ctx, user, errors.New("users cannot be created for unmanaged keycloak instances"))
			}

			// Get an authenticated keycloak api client for the instance
			keycloakFactory := common.LocalConfigKeycloakFactory{}
			authenticated, err := keycloakFactory.AuthenticatedClient(keycloak, false)
			if err != nil {
				return r.ManageError(ctx, user, err)
			}

			// Compute the current state of the realm
			userState := common.NewUserState(keycloak)

			logger.Info(fmt.Sprintf("read state for keycloak %v/%v, realm %v/%v",
				keycloak.Namespace,
				keycloak.Name,
				user.Namespace,
				realm.Spec.Realm.Realm))

			err = userState.Read(authenticated, r.Client, user, realm)
			if err != nil {
				return r.ManageError(ctx, user, err)
			}
			reconciler := keycloakuser.NewKeycloakuserReconciler(keycloak, realm)
			desiredState := reconciler.Reconcile(userState, user)

			actionRunner := common.NewClusterAndKeycloakActionRunner(ctx, r.Client, r.Scheme, user, authenticated)
			err = actionRunner.RunAll(desiredState)
			if err != nil {
				return r.ManageError(ctx, user, err)
			}
		}
	}

	return ctrl.Result{}, r.manageSuccess(ctx, user, user.DeletionTimestamp != nil)
}

func (r *KeycloakUserReconciler) manageSuccess(ctx context.Context, user *keycloakorgv1alpha1.KeycloakUser, deleted bool) error {
	logger := log.FromContext(ctx)

	user.Status.Phase = keycloakorgv1alpha1.UserPhaseReconciled
	user.Status.Message = ""

	err := r.Client.Status().Update(ctx, user)
	if err != nil {
		logger.Error(err, "unable to update status")
	}

	// Finalizer already set?
	finalizerExists := false
	for _, finalizer := range user.Finalizers {
		if finalizer == keycloakorgv1alpha1.UserFinalizer {
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
		user.Finalizers = append(user.Finalizers, keycloakorgv1alpha1.UserFinalizer)
		logger.Info(fmt.Sprintf("added finalizer to keycloak user %v/%v", user.Namespace, user.Name))
		return r.Client.Update(ctx, user)
	}

	// Otherwise remove the finalizer
	newFinalizers := []string{}
	for _, finalizer := range user.Finalizers {
		if finalizer == keycloakorgv1alpha1.UserFinalizer {
			logger.Info(fmt.Sprintf("removed finalizer from keycloak user %v/%v", user.Namespace, user.Name))
			continue
		}
		newFinalizers = append(newFinalizers, finalizer)
	}

	user.Finalizers = newFinalizers
	return r.Client.Update(ctx, user)
}

func (r *KeycloakUserReconciler) ManageError(ctx context.Context, user *keycloakorgv1alpha1.KeycloakUser, err error) (reconcile.Result, error) {
	logger := log.FromContext(ctx)

	r.Recorder.Event(user, "Warning", "ProcessingError", err.Error())

	user.Status.Phase = keycloakorgv1alpha1.UserPhaseFailing
	user.Status.Message = err.Error()

	if err := r.Client.Status().Update(ctx, user); err != nil {
		logger.Error(err, "unable to update status")
	}

	return reconcile.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *KeycloakUserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&keycloakorgv1alpha1.KeycloakUser{}).
		Watches(&corev1.Pod{}, handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &keycloakorgv1alpha1.KeycloakUser{}, handler.OnlyControllerOwner())).
		Watches(&corev1.Secret{}, handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &keycloakorgv1alpha1.KeycloakUser{}, handler.OnlyControllerOwner())).
		Named("keycloakuser").
		Complete(reconcile.AsReconciler(mgr.GetClient(), r))
}
