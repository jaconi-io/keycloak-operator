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

	batchv1 "k8s.io/api/batch/v1"
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
	"github.com/jaconi-io/keycloak-operator/internal/controller/keycloakbackup"
	"github.com/pkg/errors"
)

// KeycloakBackupReconciler reconciles a KeycloakBackup object
type KeycloakBackupReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=keycloak.org,resources=keycloakbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keycloak.org,resources=keycloakbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=keycloak.org,resources=keycloakbackups/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the KeycloakBackup object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *KeycloakBackupReconciler) Reconcile(ctx context.Context, backup *keycloakorgv1alpha1.KeycloakBackup) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling KeycloakBackup")

	// If no selector is set we can't figure out which Keycloak instance this backup should
	// be added for. Skip reconcile until a selector has been set.
	if backup.Spec.InstanceSelector == nil {
		logger.Info(fmt.Sprintf("backup %v/%v has no instance selector and will be ignored", backup.Namespace, backup.Name))
		return reconcile.Result{Requeue: false}, nil
	}

	keycloaks, err := common.GetMatchingKeycloaks(ctx, r.Client, backup.Spec.InstanceSelector)
	if err != nil {
		return r.ManageError(ctx, backup, err)
	}

	// backups without instances to backup are treated as errors
	if len(keycloaks.Items) == 0 {
		return r.ManageError(ctx, backup, errors.Errorf("no instance to backup for %v/%v", backup.Namespace, backup.Name))
	}

	logger.Info(fmt.Sprintf("found %v matching keycloak(s) for backup %v/%v", len(keycloaks.Items), backup.Namespace, backup.Name))

	var currentState *common.BackupState
	for _, keycloak := range keycloaks.Items {
		if keycloak.Spec.Unmanaged {
			return r.ManageError(ctx, backup, errors.Errorf("backups cannot be created for unmanaged keycloak instances"))
		}

		currentState = common.NewBackupState(keycloak)
		err = currentState.Read(ctx, backup, r.Client)
		if err != nil {
			return r.ManageError(ctx, backup, err)
		}
		reconciler := keycloakbackup.NewKeycloakBackupReconciler(keycloak)
		desiredState := reconciler.Reconcile(currentState, backup)
		actionRunner := common.NewClusterActionRunner(ctx, r.Client, r.Scheme, backup)
		err = actionRunner.RunAll(desiredState)
		if err != nil {
			return r.ManageError(ctx, backup, err)
		}
	}

	return r.ManageSuccess(ctx, backup, currentState)
}

func (r *KeycloakBackupReconciler) ManageError(ctx context.Context, backup *keycloakorgv1alpha1.KeycloakBackup, err error) (reconcile.Result, error) {
	logger := log.FromContext(ctx)
	r.Recorder.Event(backup, "Warning", "ProcessingError", err.Error())

	backup.Status.Message = err.Error()
	backup.Status.Ready = false
	backup.Status.Phase = keycloakorgv1alpha1.BackupPhaseFailing

	if err := r.Client.Status().Update(ctx, backup); err != nil {
		logger.Error(err, "unable to update status")
	}

	return reconcile.Result{}, err
}

func (r *KeycloakBackupReconciler) ManageSuccess(ctx context.Context, backup *keycloakorgv1alpha1.KeycloakBackup, currentState *common.BackupState) (reconcile.Result, error) {
	logger := log.FromContext(ctx)

	resourcesReady, err := currentState.IsResourcesReady()
	if err != nil {
		return r.ManageError(ctx, backup, err)
	}
	backup.Status.Ready = resourcesReady
	backup.Status.Message = ""

	if resourcesReady {
		backup.Status.Phase = keycloakorgv1alpha1.BackupPhaseCreated
	} else {
		backup.Status.Phase = keycloakorgv1alpha1.BackupPhaseReconciling
	}

	err = r.Client.Status().Update(ctx, backup)
	if err != nil {
		logger.Error(err, "unable to update status")
		return reconcile.Result{}, err
	}

	logger.Info("desired cluster state met")
	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KeycloakBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&keycloakorgv1alpha1.KeycloakBackup{}).
		Watches(&batchv1.Job{}, handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &keycloakorgv1alpha1.KeycloakBackup{}, handler.OnlyControllerOwner())).
		Watches(&batchv1.CronJob{}, handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &keycloakorgv1alpha1.KeycloakBackup{}, handler.OnlyControllerOwner())).
		Watches(&corev1.PersistentVolumeClaim{}, handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &keycloakorgv1alpha1.KeycloakBackup{}, handler.OnlyControllerOwner())).
		Named("keycloakbackup").
		Complete(reconcile.AsReconciler(mgr.GetClient(), r))
}
