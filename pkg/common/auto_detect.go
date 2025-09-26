package common

import (
	"time"

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

	pdb := policyv1beta1.SchemeGroupVersion.WithKind(PodDisruptionBudgetKind)

	resources, _ := resourcesExist(b.dc, []schema.GroupVersionKind{
		pdb,
	})

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
