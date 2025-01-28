package model

import (
	grafanav1beta1 "github.com/grafana/grafana-operator/v5/api/v1beta1"
	"github.com/jaconi-io/keycloak-operator/api/v1alpha1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GrafanaDashboard(cr *v1alpha1.Keycloak) *grafanav1beta1.GrafanaDashboard {
	return &grafanav1beta1.GrafanaDashboard{
		ObjectMeta: v12.ObjectMeta{
			Name:      ApplicationName,
			Namespace: cr.Namespace,
			Labels: map[string]string{
				"monitoring-key": MonitoringKey,
			},
		},
		Spec: grafanav1beta1.GrafanaDashboardSpec{
			Json: GrafanaDashboardJSON,
			Plugins: []grafanav1beta1.GrafanaPlugin{
				{
					Name:    "grafana-piechart-panel",
					Version: "1.3.9",
				},
			},
			Datasources: []grafanav1beta1.GrafanaDashboardDatasource{
				{
					InputName:      "DS_PROMETHEUS",
					DatasourceName: "Prometheus",
				},
			},
		},
	}
}

func GrafanaDashboardReconciled(cr *v1alpha1.Keycloak, currentState *grafanav1beta1.GrafanaDashboard) *grafanav1beta1.GrafanaDashboard {
	reconciled := currentState.DeepCopy()
	reconciled.Spec.Json = GrafanaDashboardJSON
	reconciled.Spec.Plugins = []grafanav1beta1.GrafanaPlugin{
		{
			Name:    "grafana-piechart-panel",
			Version: "1.3.9",
		},
	}
	reconciled.Spec.Datasources = []grafanav1beta1.GrafanaDashboardDatasource{
		{
			InputName:      "DS_PROMETHEUS",
			DatasourceName: "Prometheus",
		},
	}
	return reconciled
}

func GrafanaDashboardSelector(cr *v1alpha1.Keycloak) client.ObjectKey {
	return client.ObjectKey{
		Name:      ApplicationName,
		Namespace: cr.Namespace,
	}
}
