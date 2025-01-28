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

	keycloakorgv1alpha1 "github.com/jaconi-io/keycloak-operator/api/v1alpha1"
	"github.com/jaconi-io/keycloak-operator/internal/common"
	"github.com/jaconi-io/keycloak-operator/internal/controller/keycloak"
	"github.com/jaconi-io/keycloak-operator/internal/model"
	"github.com/jaconi-io/keycloak-operator/version"
	"github.com/pkg/errors"

	grafanav1beta1 "github.com/grafana/grafana-operator/v5/api/v1beta1"
	routev1 "github.com/openshift/api/route/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// KeycloakReconciler reconciles a Keycloak object
type KeycloakReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=keycloak.org,resources=keycloaks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keycloak.org,resources=keycloaks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=keycloak.org,resources=keycloaks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Keycloak object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *KeycloakReconciler) Reconcile(ctx context.Context, instance *keycloakorgv1alpha1.Keycloak) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Keycloak")

	currentState := common.NewClusterState()

	if instance.Spec.Unmanaged {
		return r.ManageSuccess(ctx, instance, currentState)
	}

	if instance.Spec.External.Enabled {
		return r.ManageError(ctx, instance, errors.Errorf("if external.enabled is true, unmanaged also needs to be true"))
	}

	if instance.Spec.ExternalAccess.Host != "" {
		isOpenshift, _ := common.GetStateManager().GetState(common.OpenShiftAPIServerKind).(bool)
		if isOpenshift {
			return r.ManageError(ctx, instance, errors.Errorf("Setting Host in External Access on OpenShift is prohibited"))
		}
	}

	// Read current state
	err := currentState.Read(ctx, instance, r.Client)
	if err != nil {
		return r.ManageError(ctx, instance, err)
	}

	// Get Action to reconcile current state into desired state
	reconciler := keycloak.NewKeycloakReconciler()
	desiredState := reconciler.Reconcile(currentState, instance)

	// Perform migration if needed
	migrator, err := keycloak.GetMigrator(instance)
	if err != nil {
		return r.ManageError(ctx, instance, err)
	}
	desiredState, err = migrator.Migrate(instance, currentState, desiredState)
	if err != nil {
		return r.ManageError(ctx, instance, err)
	}

	// Run the actions to reach the desired state
	actionRunner := common.NewClusterActionRunner(ctx, r.Client, r.Scheme, instance)
	err = actionRunner.RunAll(desiredState)
	if err != nil {
		return r.ManageError(ctx, instance, err)
	}

	return r.ManageSuccess(ctx, instance, currentState)
}

func (r *KeycloakReconciler) ManageError(ctx context.Context, instance *keycloakorgv1alpha1.Keycloak, err error) (reconcile.Result, error) {
	logger := log.FromContext(ctx)
	r.Recorder.Event(instance, "Warning", "ProcessingError", err.Error())

	instance.Status.Message = err.Error()
	instance.Status.Ready = false
	instance.Status.Phase = keycloakorgv1alpha1.PhaseFailing

	setVersion(instance)

	if err := r.Client.Status().Update(ctx, instance); err != nil {
		logger.Error(err, "unable to update status")
	}

	return reconcile.Result{}, err
}

func (r *KeycloakReconciler) ManageSuccess(ctx context.Context, instance *keycloakorgv1alpha1.Keycloak, currentState *common.ClusterState) (reconcile.Result, error) {
	logger := log.FromContext(ctx)

	// Check if the resources are ready
	resourcesReady, err := currentState.IsResourcesReady(instance)
	if err != nil {
		return r.ManageError(ctx, instance, err)
	}

	instance.Status.Ready = resourcesReady
	instance.Status.Message = ""

	// If resources are ready and we have not errored before now, we are in a reconciling phase
	if resourcesReady {
		instance.Status.Phase = keycloakorgv1alpha1.PhaseReconciling
	} else {
		instance.Status.Phase = keycloakorgv1alpha1.PhaseInitialising
	}

	if currentState.KeycloakService != nil && currentState.KeycloakService.Spec.ClusterIP != "" {
		instance.Status.InternalURL = fmt.Sprintf("https://%v.%v.svc:%v",
			currentState.KeycloakService.Name,
			currentState.KeycloakService.Namespace,
			model.KeycloakServicePort)
	}

	if instance.Spec.External.URL != "" {
		instance.Status.ExternalURL = instance.Spec.External.URL
	} else if currentState.KeycloakRoute != nil && currentState.KeycloakRoute.Spec.Host != "" {
		instance.Status.ExternalURL = fmt.Sprintf("https://%v", currentState.KeycloakRoute.Spec.Host)
	} else if currentState.KeycloakIngress != nil && currentState.KeycloakIngress.Spec.Rules[0].Host != "" {
		instance.Status.ExternalURL = fmt.Sprintf("https://%v", currentState.KeycloakIngress.Spec.Rules[0].Host)
	}

	// Let the clients know where the admin credentials are stored
	if currentState.KeycloakAdminSecret != nil {
		instance.Status.CredentialSecret = currentState.KeycloakAdminSecret.Name
	}

	setVersion(instance)

	err = r.Client.Status().Update(ctx, instance)
	if err != nil {
		logger.Error(err, "unable to update status")
		return reconcile.Result{}, err
	}

	logger.Info("desired cluster state met")
	return ctrl.Result{}, nil
}

func setVersion(instance *keycloakorgv1alpha1.Keycloak) {
	instance.Status.Version = version.Version
}

// SetupWithManager sets up the controller with the Manager.
func (r *KeycloakReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&keycloakorgv1alpha1.Keycloak{}).
		Watches(&corev1.Secret{}, handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &keycloakorgv1alpha1.Keycloak{}, handler.OnlyControllerOwner())).
		Watches(&appsv1.StatefulSet{}, handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &keycloakorgv1alpha1.Keycloak{}, handler.OnlyControllerOwner())).
		Watches(&corev1.Service{}, handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &keycloakorgv1alpha1.Keycloak{}, handler.OnlyControllerOwner())).
		Watches(&networkingv1.Ingress{}, handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &keycloakorgv1alpha1.Keycloak{}, handler.OnlyControllerOwner())).
		Watches(&appsv1.Deployment{}, handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &keycloakorgv1alpha1.Keycloak{}, handler.OnlyControllerOwner())).
		Watches(&corev1.PersistentVolumeClaim{}, handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &keycloakorgv1alpha1.Keycloak{}, handler.OnlyControllerOwner())).
		Watches(&policyv1.PodDisruptionBudget{}, handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &keycloakorgv1alpha1.Keycloak{}, handler.OnlyControllerOwner())).
		Watches(&monitoringv1.PrometheusRule{}, handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &keycloakorgv1alpha1.Keycloak{}, handler.OnlyControllerOwner())).
		Watches(&monitoringv1.ServiceMonitor{}, handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &keycloakorgv1alpha1.Keycloak{}, handler.OnlyControllerOwner())).
		Watches(&grafanav1beta1.GrafanaDashboard{}, handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &keycloakorgv1alpha1.Keycloak{}, handler.OnlyControllerOwner())).
		Watches(&routev1.Route{}, handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &keycloakorgv1alpha1.Keycloak{}, handler.OnlyControllerOwner())).
		Named("keycloak").
		Complete(reconcile.AsReconciler(mgr.GetClient(), r))
}
