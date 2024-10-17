package controller

import (
	"context"
	"fmt"
	"time"

	grafanav1alpha1 "github.com/grafana-operator/grafana-operator/v4/api/integreatly/v1alpha1"
	"github.com/keycloak/keycloak-operator/pkg/common"
	"github.com/keycloak/keycloak-operator/pkg/model"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/pkg/errors"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/jaconi-io/keycloak-operator/api/v1alpha1"
	kc "github.com/jaconi-io/keycloak-operator/api/v1alpha1"
	keycloakorgv1alpha1 "github.com/jaconi-io/keycloak-operator/api/v1alpha1"
)

const (
	RequeueDelay      = 30 * time.Second
	RequeueDelayError = 5 * time.Second
	// ControllerName    = "keycloak-controller"
)

// KeycloakReconciler reconciles a Keycloak object
type KeycloakReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=keycloak.org,resources=keycloaks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=keycloak.org,resources=keycloaks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=keycloak.org,resources=keycloaks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Keycloak object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *KeycloakReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	logger.Info("Reconciling Keycloak")

	// Fetch the Keycloak instance
	instance := &keycloakorgv1alpha1.Keycloak{}

	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
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
	err = currentState.Read(ctx, instance, r)
	if err != nil {
		return r.ManageError(ctx, instance, err)
	}

	// Get Action to reconcile current state into desired state
	desiredState := getDesiredState(currentState, instance)

	// Perform migration if needed
	migrator, err := GetMigrator(instance)
	if err != nil {
		return r.ManageError(ctx, instance, err)
	}
	desiredState, err = migrator.Migrate(instance, currentState, desiredState)
	if err != nil {
		return r.ManageError(ctx, instance, err)
	}

	// Run the actions to reach the desired state
	actionRunner := common.NewClusterActionRunner(ctx, r, r.Scheme, instance)
	err = actionRunner.RunAll(desiredState)
	if err != nil {
		return r.ManageError(ctx, instance, err)
	}

	return r.ManageSuccess(ctx, instance, currentState)
}

// SetupWithManager sets up the controller with the Manager.
func (r *KeycloakReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&keycloakorgv1alpha1.Keycloak{}).
		Owns(&corev1.Secret{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&monitoringv1.PrometheusRule{}).
		Owns(&monitoringv1.ServiceMonitor{}).
		Owns(&grafanav1alpha1.GrafanaDashboard{}).
		Owns(&routev1.Route{}).
		Complete(r)
}

func (r *KeycloakReconciler) ManageError(ctx context.Context, instance *v1alpha1.Keycloak, issue error) (reconcile.Result, error) {
	logger := log.FromContext(ctx)

	r.Recorder.Event(instance, "Warning", "ProcessingError", issue.Error())

	instance.Status.Message = issue.Error()
	instance.Status.Ready = false
	instance.Status.Phase = v1alpha1.PhaseFailing

	r.setVersion(instance)

	err := r.Status().Update(ctx, instance)
	if err != nil {
		logger.Error(err, "unable to update status")
	}

	return reconcile.Result{
		RequeueAfter: RequeueDelayError,
		Requeue:      true,
	}, nil
}

func (r *KeycloakReconciler) ManageSuccess(ctx context.Context, instance *v1alpha1.Keycloak, currentState *common.ClusterState) (reconcile.Result, error) {
	var logger = log.FromContext(ctx)

	// Check if the resources are ready
	resourcesReady, err := currentState.IsResourcesReady(instance)
	if err != nil {
		return r.ManageError(ctx, instance, err)
	}

	instance.Status.Ready = resourcesReady
	instance.Status.Message = ""

	// If resources are ready and we have not errored before now, we are in a reconciling phase
	if resourcesReady {
		instance.Status.Phase = v1alpha1.PhaseReconciling
	} else {
		instance.Status.Phase = v1alpha1.PhaseInitialising
	}

	if currentState.KeycloakService != nil && currentState.KeycloakService.Spec.ClusterIP != "" {
		instance.Status.InternalURL = fmt.Sprintf("https://%v.%v.svc:%v",
			currentState.KeycloakService.Name,
			currentState.KeycloakService.Namespace,
			model.KeycloakServicePort)
	}

	if instance.Spec.External.URL != "" { //nolint
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

	r.setVersion(instance)

	err = r.Status().Update(ctx, instance)
	if err != nil {
		logger.Error(err, "unable to update status")
		return reconcile.Result{
			RequeueAfter: RequeueDelayError,
			Requeue:      true,
		}, nil
	}

	logger.Info("desired cluster state met")
	return reconcile.Result{RequeueAfter: RequeueDelay}, nil
}

func getDesiredState(clusterState *common.ClusterState, cr *kc.Keycloak) common.DesiredClusterState {
	desired := common.DesiredClusterState{}

	desired = desired.AddAction(getKeycloakAdminSecretDesiredState(clusterState, cr))

	if !cr.Spec.DisableMonitoringServices {
		desired = desired.AddAction(getKeycloakPrometheusRuleDesiredState(clusterState, cr))
		desired = desired.AddAction(getKeycloakServiceMonitorDesiredState(clusterState, cr))
		desired = desired.AddAction(getKeycloakGrafanaDashboardDesiredState(clusterState, cr))
	}

	if !cr.Spec.ExternalDatabase.Enabled {
		desired = desired.AddAction(getDatabaseSecretDesiredState(clusterState, cr))
		desired = desired.AddAction(getPostgresqlPersistentVolumeClaimDesiredState(clusterState, cr))
		desired = desired.AddAction(getPostgresqlDeploymentDesiredState(clusterState, cr))
		desired = desired.AddAction(getPostgresqlServiceDesiredState(clusterState, cr, false))
	} else {
		reconcileExternalDatabase(&desired, clusterState, cr)
	}

	desired = desired.AddAction(getKeycloakServiceDesiredState(clusterState, cr))
	desired = desired.AddAction(getKeycloakDiscoveryServiceDesiredState(clusterState, cr))
	desired = desired.AddAction(getKeycloakMonitoringServiceDesiredState(clusterState, cr))
	desired = desired.AddAction(getKeycloakProbesDesiredState(clusterState, cr))
	desired = desired.AddAction(getKeycloakDeploymentOrRHSSODesiredState(clusterState, cr))
	reconcileExternalAccess(&desired, clusterState, cr)
	desired = desired.AddAction(getPodDisruptionBudgetDesiredState(clusterState, cr))

	if cr.Spec.Migration.Backups.Enabled {
		desired = desired.AddAction(getKeycloakBackupDesiredState(clusterState, cr))
	}
	return desired
}

func reconcileExternalDatabase(desired *common.DesiredClusterState, clusterState *common.ClusterState, cr *kc.Keycloak) {
	// If the database secret does not exist we can't continue
	if clusterState.DatabaseSecret == nil {
		return
	}
	if model.IsIP(clusterState.DatabaseSecret.Data[model.DatabaseSecretExternalAddressProperty]) {
		// If the address of the external database is an IP address then we have to
		// set up an endpoints object for the service to send traffic. An externalName
		// type service won't work in this case. For more details, see https://cloud.google.com/blog/products/gcp/kubernetes-best-practices-mapping-external-services
		desired.AddAction(getPostgresqlServiceEndpointsDesiredState(clusterState, cr))
	}
	desired.AddAction(getPostgresqlServiceDesiredState(clusterState, cr, true))
}

func reconcileExternalAccess(desired *common.DesiredClusterState, clusterState *common.ClusterState, cr *kc.Keycloak) {
	if !cr.Spec.ExternalAccess.Enabled {
		return
	}

	// Find out if we're on OpenShift or Kubernetes and create either a Route or
	// an Ingress
	stateManager := common.GetStateManager()
	openshift, keyExists := stateManager.GetState(common.OpenShiftAPIServerKind).(bool)

	if keyExists && openshift {
		desired.AddAction(getKeycloakRouteDesiredState(clusterState, cr))
		desired.AddAction(getKeycloakMetricsRouteDesiredState(clusterState, cr))
	} else {
		desired.AddAction(getKeycloakIngressDesiredState(clusterState, cr))
	}
}

func getKeycloakAdminSecretDesiredState(clusterState *common.ClusterState, cr *kc.Keycloak) common.ClusterAction {
	keycloakAdminSecret := model.KeycloakAdminSecret(cr)

	if clusterState.KeycloakAdminSecret == nil {
		return common.GenericCreateAction{
			Ref: keycloakAdminSecret,
			Msg: "Create Keycloak admin secret",
		}
	}
	return common.GenericUpdateAction{
		Ref: model.KeycloakAdminSecretReconciled(cr, clusterState.KeycloakAdminSecret),
		Msg: "Update Keycloak admin secret",
	}
}

func getKeycloakProbesDesiredState(clusterState *common.ClusterState, cr *kc.Keycloak) common.ClusterAction {
	keycloakProbesConfigMap := model.KeycloakProbes(cr)

	if clusterState.KeycloakProbes == nil {
		return common.GenericCreateAction{
			Ref: keycloakProbesConfigMap,
			Msg: "Create Keycloak probes configmap",
		}
	}
	return nil
}

func getPostgresqlPersistentVolumeClaimDesiredState(clusterState *common.ClusterState, cr *kc.Keycloak) common.ClusterAction {
	postgresqlPersistentVolume := model.PostgresqlPersistentVolumeClaim(cr)
	if clusterState.PostgresqlPersistentVolumeClaim == nil {
		return common.GenericCreateAction{
			Ref: postgresqlPersistentVolume,
			Msg: "Create Postgresql PersistentVolumeClaim",
		}
	}
	return common.GenericUpdateAction{
		Ref: model.PostgresqlPersistentVolumeClaimReconciled(cr, clusterState.PostgresqlPersistentVolumeClaim),
		Msg: "Update Postgresql PersistentVolumeClaim",
	}
}

func getPostgresqlServiceDesiredState(clusterState *common.ClusterState, cr *kc.Keycloak, isExternal bool) common.ClusterAction {
	postgresqlService := model.PostgresqlService(cr, clusterState.DatabaseSecret, isExternal)
	if clusterState.PostgresqlService == nil {
		return common.GenericCreateAction{
			Ref: postgresqlService,
			Msg: "Create Postgresql KeycloakService",
		}
	}
	return common.GenericUpdateAction{
		Ref: model.PostgresqlServiceReconciled(clusterState.PostgresqlService, clusterState.DatabaseSecret, isExternal),
		Msg: "Update Postgresql KeycloakService",
	}
}

func getPostgresqlDeploymentDesiredState(clusterState *common.ClusterState, cr *kc.Keycloak) common.ClusterAction {
	// Find out if we're on OpenShift or Kubernetes
	stateManager := common.GetStateManager()
	isOpenshift, _ := stateManager.GetState(common.OpenShiftAPIServerKind).(bool)

	postgresqlDeployment := model.PostgresqlDeployment(cr, isOpenshift)

	if clusterState.PostgresqlDeployment == nil {
		return common.GenericCreateAction{
			Ref: postgresqlDeployment,
			Msg: "Create Postgresql Deployment",
		}
	}
	return common.GenericUpdateAction{
		Ref: model.PostgresqlDeploymentReconciled(cr, clusterState.PostgresqlDeployment),
		Msg: "Update Postgresql Deployment",
	}
}

func getKeycloakServiceDesiredState(clusterState *common.ClusterState, cr *kc.Keycloak) common.ClusterAction {
	keycloakService := model.KeycloakService(cr)

	if clusterState.KeycloakService == nil {
		return common.GenericCreateAction{
			Ref: keycloakService,
			Msg: "Create Keycloak Service",
		}
	}
	return common.GenericUpdateAction{
		Ref: model.KeycloakServiceReconciled(cr, clusterState.KeycloakService),
		Msg: "Update keycloak Service",
	}
}

func getKeycloakDiscoveryServiceDesiredState(clusterState *common.ClusterState, cr *kc.Keycloak) common.ClusterAction {
	keycloakDiscoveryService := model.KeycloakDiscoveryService(cr)

	if clusterState.KeycloakDiscoveryService == nil {
		return common.GenericCreateAction{
			Ref: keycloakDiscoveryService,
			Msg: "Create Keycloak Discovery Service",
		}
	}
	return common.GenericUpdateAction{
		Ref: model.KeycloakDiscoveryServiceReconciled(cr, clusterState.KeycloakDiscoveryService),
		Msg: "Update keycloak Discovery Service",
	}
}

func getKeycloakMonitoringServiceDesiredState(clusterState *common.ClusterState, cr *kc.Keycloak) common.ClusterAction {
	stateManager := common.GetStateManager()
	resourceWatchExists, keyExists := stateManager.GetState(common.GetStateFieldName(ControllerName, monitoringv1.ServiceMonitorsKind)).(bool)
	// Only add or update the monitoring resources if the resource type exists on the cluster. These booleans are set in the common/autodetect logic
	if !keyExists || !resourceWatchExists {
		return nil
	}

	keycloakMonitoringService := model.KeycloakMonitoringService(cr)

	if clusterState.KeycloakMonitoringService == nil {
		return common.GenericCreateAction{
			Ref: keycloakMonitoringService,
			Msg: "Create Keycloak Monitoring Service",
		}
	}
	return common.GenericUpdateAction{
		Ref: model.KeycloakMonitoringServiceReconciled(cr, clusterState.KeycloakMonitoringService),
		Msg: "Update keycloak Monitoring Service",
	}
}

func getKeycloakPrometheusRuleDesiredState(clusterState *common.ClusterState, cr *kc.Keycloak) common.ClusterAction {
	stateManager := common.GetStateManager()
	resourceWatchExists, keyExists := stateManager.GetState(common.GetStateFieldName(ControllerName, monitoringv1.PrometheusRuleKind)).(bool)
	// Only add or update the monitoring resources if the resource type exists on the cluster. These booleans are set in the common/autodetect logic
	if !keyExists || !resourceWatchExists {
		return nil
	}

	prometheusrule := model.PrometheusRule(cr)

	if clusterState.KeycloakPrometheusRule == nil {
		return common.GenericCreateAction{
			Ref: prometheusrule,
			Msg: "create keycloak prometheus rule",
		}
	}

	prometheusrule.ResourceVersion = clusterState.KeycloakPrometheusRule.ResourceVersion
	return common.GenericUpdateAction{
		Ref: prometheusrule,
		Msg: "update keycloak prometheus rule",
	}
}

func getKeycloakServiceMonitorDesiredState(clusterState *common.ClusterState, cr *kc.Keycloak) common.ClusterAction {
	stateManager := common.GetStateManager()
	resourceWatchExists, keyExists := stateManager.GetState(common.GetStateFieldName(ControllerName, monitoringv1.ServiceMonitorsKind)).(bool)
	// Only add or update the monitoring resources if the resource type exists on the cluster. These booleans are set in the common/autodetect logic
	if !keyExists || !resourceWatchExists {
		return nil
	}

	servicemonitor := model.ServiceMonitor(cr)

	if clusterState.KeycloakServiceMonitor == nil {
		return common.GenericCreateAction{
			Ref: servicemonitor,
			Msg: "create keycloak service monitor",
		}
	}

	servicemonitor.ResourceVersion = clusterState.KeycloakServiceMonitor.ResourceVersion
	return common.GenericUpdateAction{
		Ref: servicemonitor,
		Msg: "update keycloak service monitor",
	}
}

func getKeycloakGrafanaDashboardDesiredState(clusterState *common.ClusterState, cr *kc.Keycloak) common.ClusterAction {
	stateManager := common.GetStateManager()
	resourceWatchExists, keyExists := stateManager.GetState(common.GetStateFieldName(ControllerName, grafanav1alpha1.GrafanaDashboardKind)).(bool)
	// Only add or update the monitoring resources if the resource type exists on the cluster. These booleans are set in the common/autodetect logic
	if !keyExists || !resourceWatchExists {
		return nil
	}

	grafanadashboard := model.GrafanaDashboard(cr)

	if clusterState.KeycloakGrafanaDashboard == nil {
		return common.GenericCreateAction{
			Ref: grafanadashboard,
			Msg: "create keycloak grafana dashboard",
		}
	}

	return common.GenericUpdateAction{
		Ref: model.GrafanaDashboardReconciled(cr, clusterState.KeycloakGrafanaDashboard),
		Msg: "update keycloak grafana dashboard",
	}
}

func getDatabaseSecretDesiredState(clusterState *common.ClusterState, cr *kc.Keycloak) common.ClusterAction {
	databaseSecret := model.DatabaseSecret(cr)
	if clusterState.DatabaseSecret == nil {
		return common.GenericCreateAction{
			Ref: databaseSecret,
			Msg: "Create Database Secret",
		}
	}
	return common.GenericUpdateAction{
		Ref: model.DatabaseSecretReconciled(cr, clusterState.DatabaseSecret),
		Msg: "Update Database Secret",
	}
}

func getKeycloakDeploymentOrRHSSODesiredState(clusterState *common.ClusterState, cr *kc.Keycloak) common.ClusterAction {
	isRHSSO := model.Profiles.IsRHSSO(cr)

	deployment := model.KeycloakDeployment(cr, clusterState.DatabaseSecret, clusterState.DatabaseSSLCert)
	deploymentName := "Keycloak"

	if isRHSSO {
		deployment = model.RHSSODeployment(cr, clusterState.DatabaseSecret, clusterState.DatabaseSSLCert)
		deploymentName = model.RHSSOProfile
	}

	if clusterState.KeycloakDeployment == nil {
		return common.GenericCreateAction{
			Ref: deployment,
			Msg: "Create " + deploymentName + " Deployment (StatefulSet)",
		}
	}

	deploymentReconciled := model.KeycloakDeploymentReconciled(cr, clusterState.KeycloakDeployment, clusterState.DatabaseSecret, clusterState.DatabaseSSLCert)
	if isRHSSO {
		deploymentReconciled = model.RHSSODeploymentReconciled(cr, clusterState.KeycloakDeployment, clusterState.DatabaseSecret, clusterState.DatabaseSSLCert)
	}

	return common.GenericUpdateAction{
		Ref: deploymentReconciled,
		Msg: "Update " + deploymentName + " Deployment (StatefulSet)",
	}
}

func getKeycloakRouteDesiredState(clusterState *common.ClusterState, cr *kc.Keycloak) common.ClusterAction {
	if clusterState.KeycloakRoute == nil {
		return common.GenericCreateAction{
			Ref: model.KeycloakRoute(cr),
			Msg: "Create Keycloak Route",
		}
	}

	return common.GenericUpdateAction{
		Ref: model.KeycloakRouteReconciled(cr, clusterState.KeycloakRoute),
		Msg: "Update Keycloak Route",
	}
}

func getKeycloakMetricsRouteDesiredState(clusterState *common.ClusterState, cr *kc.Keycloak) common.ClusterAction {
	if clusterState.KeycloakRoute == nil {
		return nil
	}

	if clusterState.KeycloakMetricsRoute == nil {
		return common.GenericCreateAction{
			Ref: model.KeycloakMetricsRoute(cr, clusterState.KeycloakRoute),
			Msg: "Create Keycloak Metrics Route",
		}
	}

	return common.GenericUpdateAction{
		Ref: model.KeycloakMetricsRouteReconciled(cr, clusterState.KeycloakMetricsRoute, clusterState.KeycloakRoute),
		Msg: "Update Keycloak Metrics Route",
	}
}

func getKeycloakIngressDesiredState(clusterState *common.ClusterState, cr *kc.Keycloak) common.ClusterAction {
	if clusterState.KeycloakIngress == nil {
		return common.GenericCreateAction{
			Ref: model.KeycloakIngress(cr),
			Msg: "Create Keycloak Ingress",
		}
	}

	return common.GenericUpdateAction{
		Ref: model.KeycloakIngressReconciled(cr, clusterState.KeycloakIngress),
		Msg: "Update Keycloak Ingress",
	}
}

func getPostgresqlServiceEndpointsDesiredState(clusterState *common.ClusterState, cr *kc.Keycloak) common.ClusterAction {
	if clusterState.PostgresqlServiceEndpoints == nil {
		// This happens only during initial run
		return nil
	}
	return common.GenericUpdateAction{
		Ref: model.PostgresqlServiceEndpointsReconciled(cr, clusterState.PostgresqlServiceEndpoints, clusterState.DatabaseSecret),
		Msg: "Update External Database Service Endpoints",
	}
}

func getPodDisruptionBudgetDesiredState(clusterState *common.ClusterState, cr *kc.Keycloak) common.ClusterAction {
	if cr.Spec.PodDisruptionBudget.Enabled {
		stateManager := common.GetStateManager()
		podDisruptionBudgetKind, keyExists := stateManager.GetState(common.PodDisruptionBudgetKind).(bool)
		if !keyExists || !podDisruptionBudgetKind {
			log.Info("podDisruptionBudget is enabled in the CR but policy/v1beta1 PodDisruptionBudget API was not found; please create podDisruptionBudget manually")
			return nil
		}

		log.Info("using deprecated podDisruptionBudget field")

		if clusterState.PodDisruptionBudget == nil {
			return common.GenericCreateAction{
				Ref: model.PodDisruptionBudget(cr),
				Msg: "Create PodDisruptionBudget",
			}
		}
		return common.GenericUpdateAction{
			Ref: model.PodDisruptionBudgetReconciled(cr, clusterState.PodDisruptionBudget),
			Msg: "Update PodDisruptionBudget",
		}
	}
	return nil
}

func getKeycloakBackupDesiredState(clusterState *common.ClusterState, cr *kc.Keycloak) common.ClusterAction {
	backupCr := &v1alpha1.KeycloakBackup{}
	backupCr.Namespace = cr.Namespace
	backupCr.Name = model.MigrateBackupName + "-" + common.BackupTime
	labelSelect := metav1.LabelSelector{
		MatchLabels: cr.Labels,
	}
	backupCr.Spec.InstanceSelector = &labelSelect
	backupCr.Spec.StorageClassName = cr.Spec.StorageClassName

	if clusterState.KeycloakBackup == nil {
		// This happens before migration
		return nil
	}

	keycloakbackup := model.KeycloakMigrationOneTimeBackup(backupCr)
	keycloakbackup.ResourceVersion = clusterState.KeycloakBackup.ResourceVersion
	return common.GenericUpdateAction{
		Ref: keycloakbackup,
		Msg: "Update Postgresql Backup for Keycloak Migration",
	}
}
