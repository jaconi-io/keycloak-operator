package common

import (
	"context"

	kc "github.com/jaconi-io/keycloak-operator/api/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// These kinds are not provided by the openshift api
const (
	RouteKind                 = "Route"
	JobKind                   = "Job"
	CronJobKind               = "CronJob"
	SecretKind                = "Secret"
	StatefulSetKind           = "StatefulSet"
	ServiceKind               = "Service"
	IngressKind               = "Ingress"
	DeploymentKind            = "Deployment"
	PersistentVolumeClaimKind = "PersistentVolumeClaim"
	PodDisruptionBudgetKind   = "PodDisruptionBudget"
	OpenShiftAPIServerKind    = "OpenShiftAPIServer"
)

func GetStateFieldName(controllerName string, kind string) string {
	return controllerName + "-watch-" + kind
}

// Try to get a list of keycloak instances that match the selector specified on the realm
func GetMatchingKeycloaks(ctx context.Context, c client.Client, labelSelector *v1.LabelSelector) (kc.KeycloakList, error) {
	var list kc.KeycloakList
	opts := []client.ListOption{
		client.MatchingLabels(labelSelector.MatchLabels),
	}

	err := c.List(ctx, &list, opts...)
	return list, err
}

// Try to get a list of keycloak instances that match the selector specified on the realm
func GetMatchingRealms(ctx context.Context, c client.Client, labelSelector *v1.LabelSelector) (kc.KeycloakRealmList, error) {
	var list kc.KeycloakRealmList
	opts := []client.ListOption{
		client.MatchingLabels(labelSelector.MatchLabels),
	}

	err := c.List(ctx, &list, opts...)
	return list, err
}
