package common

import (
	"time"

	grafanav1beta1 "github.com/grafana/grafana-operator/v5/api/v1beta1"
	routev1 "github.com/openshift/api/route/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Background represents a procedure that runs in the background, periodically auto-detecting features
type Background struct {
	dc     discovery.DiscoveryInterface
	ticker *time.Ticker
}

// New creates a new auto-detect runner
func NewAutoDetect(mgr manager.Manager) (*Background, error) {
	dc, err := discovery.NewDiscoveryClientForConfig(mgr.GetConfig())
	if err != nil {
		return nil, err
	}

	return &Background{dc: dc}, nil
}

// Start initializes the auto-detection process that runs in the background
func (b *Background) Start() {
	b.autoDetectCapabilities()
	// periodically attempts to auto detect all the capabilities for this operator
	b.ticker = time.NewTicker(10 * time.Hour)

	go func() {
		for range b.ticker.C {
			b.autoDetectCapabilities()
		}
	}()
}

// Stop causes the background process to stop auto detecting capabilities
func (b *Background) Stop() {
	b.ticker.Stop()
}

func (b *Background) autoDetectCapabilities() {
	stateManager := GetStateManager()

	openshift := schema.FromAPIVersionAndKind("operator.openshift.io/v1", OpenShiftAPIServerKind)
	prometheusRule := monitoringv1.SchemeGroupVersion.WithKind(monitoringv1.PrometheusRuleKind)
	serviceMonitor := monitoringv1.SchemeGroupVersion.WithKind(monitoringv1.ServiceMonitorsKind)
	grafanaDashboard := grafanav1beta1.SchemeGroupVersion.WithKind(grafanav1beta1.GrafanaDashboardKind)
	route := routev1.SchemeGroupVersion.WithKind(RouteKind)
	pdb := policyv1beta1.SchemeGroupVersion.WithKind(PodDisruptionBudgetKind)

	resources, _ := resourcesExist(b.dc, []schema.GroupVersionKind{
		openshift, prometheusRule, serviceMonitor, grafanaDashboard, route, pdb,
	})

	// Set state that its Openshift (helps to differentiate between openshift and kubernetes)
	stateManager.SetState(OpenShiftAPIServerKind, resources[openshift])

	stateManager.SetState(monitoringv1.PrometheusRuleKind, resources[prometheusRule])
	stateManager.SetState(monitoringv1.ServiceMonitorsKind, resources[serviceMonitor])
	stateManager.SetState(grafanav1beta1.GrafanaDashboardKind, resources[grafanaDashboard])

	// Set state that the Route kind exists. Used to determine when a route or an Ingress should be created
	stateManager.SetState(RouteKind, resources[route])

	stateManager.SetState(PodDisruptionBudgetKind, resources[pdb])
}

// resourcesExist is a multi-resource version of k8sutil.ResourceExists, to reduce strain on the Kubernetes API when
// checking multiple resources.
func resourcesExist(dc discovery.DiscoveryInterface, resources []schema.GroupVersionKind) (map[schema.GroupVersionKind]bool, error) {
	_, apiLists, err := dc.ServerGroupsAndResources()
	if err != nil {
		return nil, err
	}

	// Populate result map.
	res := map[schema.GroupVersionKind]bool{}
	for _, resource := range resources {
		res[resource] = false
	}

	for _, apiList := range apiLists {
		for _, r := range apiList.APIResources {
			gvk := schema.GroupVersionKind{
				Group:   r.Group,
				Version: r.Version,
				Kind:    r.Kind,
			}

			if _, ok := res[gvk]; ok {
				res[gvk] = true
			}
		}
	}

	return res, nil
}
