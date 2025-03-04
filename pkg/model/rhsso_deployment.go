package model

import (
	"fmt"

	"github.com/jaconi-io/keycloak-operator/pkg/apis/keycloak/v1alpha1"
	v13 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getRHSSOEnv(cr *v1alpha1.Keycloak, dbSecret *v1.Secret) []v1.EnvVar {
	var env = []v1.EnvVar{
		// Database settings
		{
			Name:  "DB_SERVICE_PREFIX_MAPPING",
			Value: PostgresqlServiceName + "=DB",
		},
		{
			Name:  "TX_DATABASE_PREFIX_MAPPING",
			Value: PostgresqlServiceName + "=DB",
		},
		{
			Name:  "DB_JNDI",
			Value: "java:jboss/datasources/KeycloakDS",
		},
		{
			Name:  "DB_SCHEMA",
			Value: "public",
		},
		{
			Name: "DB_USERNAME",
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: DatabaseSecretName,
					},
					Key: DatabaseSecretUsernameProperty,
				},
			},
		},
		{
			Name: "DB_PASSWORD",
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: DatabaseSecretName,
					},
					Key: DatabaseSecretPasswordProperty,
				},
			},
		},
		{
			Name:  "DB_DATABASE",
			Value: GetExternalDatabaseName(dbSecret),
		},
		// Discovery settings
		{
			Name:  "JGROUPS_PING_PROTOCOL",
			Value: "dns.DNS_PING",
		},
		{
			Name:  "OPENSHIFT_DNS_PING_SERVICE_NAME",
			Value: KeycloakDiscoveryServiceName + "." + cr.Namespace + ".svc.cluster.local",
		},
		// Cache settings
		{
			Name:  "CACHE_OWNERS_COUNT",
			Value: "2",
		},
		{
			Name:  "CACHE_OWNERS_AUTH_SESSIONS_COUNT",
			Value: "2",
		},
		{
			Name: "SSO_ADMIN_USERNAME",
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "credential-" + cr.Name,
					},
					Key: AdminUsernameProperty,
				},
			},
		},
		{
			Name: "SSO_ADMIN_PASSWORD",
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "credential-" + cr.Name,
					},
					Key: AdminPasswordProperty,
				},
			},
		},
		{
			Name:  "X509_CA_BUNDLE",
			Value: "/var/run/secrets/kubernetes.io/serviceaccount/*.crt",
		},
		{
			Name:  "STATISTICS_ENABLED",
			Value: "TRUE",
		},
	}

	if cr.Spec.ExternalDatabase.Enabled {
		env = append(env, v1.EnvVar{
			Name:  GetServiceEnvVar("SERVICE_HOST"),
			Value: PostgresqlServiceName + "." + cr.Namespace + ".svc.cluster.local",
		})
		env = append(env, v1.EnvVar{
			Name:  GetServiceEnvVar("SERVICE_PORT"),
			Value: fmt.Sprintf("%v", GetExternalDatabasePort(dbSecret)),
		})
	}

	if len(cr.Spec.KeycloakDeploymentSpec.Experimental.Env) > 0 {
		// We override Keycloak pre-defined envs with what user specified. Not the other way around.
		env = MergeEnvs(cr.Spec.KeycloakDeploymentSpec.Experimental.Env, env)
	}

	env = RHSSOSslEnvVariables(dbSecret, env)

	return env
}

func RHSSOSslEnvVariables(dbSecret *v1.Secret, env []v1.EnvVar) []v1.EnvVar {
	if dbSecret != nil {
		sslMode := string(dbSecret.Data[DatabaseSecretSslModeProperty])

		if sslMode != "" {
			// append env variable
			env = append(env,
				v1.EnvVar{
					Name:  RhssoDatabaseXAConnectionParamsProperty + "_sslMode",
					Value: sslMode,
				},
				v1.EnvVar{
					Name:  RhssoDatabaseNONXAConnectionParamsProperty + "_sslmode",
					Value: sslMode,
				},
			)
		}
	}
	return env
}

func RHSSODeployment(cr *v1alpha1.Keycloak, dbSecret *v1.Secret, dbSSLSecret *v1.Secret) *v13.StatefulSet {
	podLabels := AddPodLabels(cr, GetLabelsSelector())
	podAnnotations := cr.Spec.KeycloakDeploymentSpec.PodAnnotations
	rhssoStatefulSet := &v13.StatefulSet{
		ObjectMeta: v12.ObjectMeta{
			Name:        KeycloakDeploymentName,
			Namespace:   cr.Namespace,
			Labels:      podLabels,
			Annotations: podAnnotations,
		},
		Spec: v13.StatefulSetSpec{
			Replicas: SanitizeNumberOfReplicas(cr.Spec.Instances, true),
			Selector: &v12.LabelSelector{
				MatchLabels: GetLabelsSelector(),
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: v12.ObjectMeta{
					Name:        KeycloakDeploymentName,
					Namespace:   cr.Namespace,
					Labels:      podLabels,
					Annotations: podAnnotations,
				},
				Spec: v1.PodSpec{
					Volumes:        KeycloakVolumes(cr, dbSSLSecret),
					InitContainers: KeycloakExtensionsInitContainers(cr),
					Affinity:       KeycloakPodAffinity(cr),
					Containers: []v1.Container{
						{
							Name:  KeycloakDeploymentName,
							Image: Images.Images[RHSSOImage],
							Ports: []v1.ContainerPort{
								{
									ContainerPort: KeycloakServicePort,
									Protocol:      "TCP",
								},
								{
									ContainerPort: 8080,
									Protocol:      "TCP",
								},
								{
									ContainerPort: 9990,
									Protocol:      "TCP",
								},
								{
									ContainerPort: 8778,
									Protocol:      "TCP",
								},
							},
							LivenessProbe:   livenessProbe(),
							ReadinessProbe:  readinessProbe(),
							Env:             getRHSSOEnv(cr, dbSecret),
							Args:            cr.Spec.KeycloakDeploymentSpec.Experimental.Args,
							Command:         cr.Spec.KeycloakDeploymentSpec.Experimental.Command,
							VolumeMounts:    KeycloakVolumeMounts(cr, RhssoExtensionPath, dbSSLSecret, RhssoCertificatePath),
							Resources:       getResources(cr),
							ImagePullPolicy: cr.Spec.KeycloakDeploymentSpec.ImagePullPolicy,
						},
					},
					ServiceAccountName: cr.Spec.KeycloakDeploymentSpec.Experimental.ServiceAccountName,
				},
			},
		},
	}

	if cr.Spec.KeycloakDeploymentSpec.Experimental.Affinity != nil {
		rhssoStatefulSet.Spec.Template.Spec.Affinity = cr.Spec.KeycloakDeploymentSpec.Experimental.Affinity
	} else if cr.Spec.MultiAvailablityZones.Enabled {
		rhssoStatefulSet.Spec.Template.Spec.Affinity = KeycloakPodAffinity(cr)
	}

	return rhssoStatefulSet
}

func RHSSODeploymentSelector(cr *v1alpha1.Keycloak) client.ObjectKey {
	return client.ObjectKey{
		Name:      KeycloakDeploymentName,
		Namespace: cr.Namespace,
	}
}

func RHSSODeploymentReconciled(cr *v1alpha1.Keycloak, currentState *v13.StatefulSet, dbSecret *v1.Secret, dbSSLSecret *v1.Secret) *v13.StatefulSet {
	reconciled := currentState.DeepCopy()

	reconciled.ObjectMeta.Labels = AddPodLabels(cr, reconciled.ObjectMeta.Labels)
	reconciled.ObjectMeta.Annotations = AddPodAnnotations(cr, reconciled.ObjectMeta.Annotations)
	reconciled.Spec.Template.ObjectMeta.Labels = AddPodLabels(cr, reconciled.Spec.Template.ObjectMeta.Labels)
	reconciled.Spec.Template.ObjectMeta.Annotations = AddPodAnnotations(cr, reconciled.Spec.Template.ObjectMeta.Annotations)
	reconciled.Spec.Selector.MatchLabels = GetLabelsSelector()
	reconciled.Spec.Template.Spec.ServiceAccountName = cr.Spec.KeycloakDeploymentSpec.Experimental.ServiceAccountName

	reconciled.ResourceVersion = currentState.ResourceVersion
	if !cr.Spec.DisableReplicasSyncing {
		reconciled.Spec.Replicas = SanitizeNumberOfReplicas(cr.Spec.Instances, false)
	}
	reconciled.Spec.Template.Spec.Volumes = KeycloakVolumes(cr, dbSSLSecret)
	reconciled.Spec.Template.Spec.Containers = []v1.Container{
		{
			Name:    KeycloakDeploymentName,
			Image:   Images.Images[RHSSOImage],
			Args:    cr.Spec.KeycloakDeploymentSpec.Experimental.Args,
			Command: cr.Spec.KeycloakDeploymentSpec.Experimental.Command,
			Ports: []v1.ContainerPort{
				{
					ContainerPort: KeycloakServicePort,
					Protocol:      "TCP",
				},
				{
					ContainerPort: 8080,
					Protocol:      "TCP",
				},
				{
					ContainerPort: 9990,
					Protocol:      "TCP",
				},
				{
					ContainerPort: 8778,
					Protocol:      "TCP",
				},
			},
			VolumeMounts:    KeycloakVolumeMounts(cr, RhssoExtensionPath, dbSSLSecret, RhssoCertificatePath),
			LivenessProbe:   livenessProbe(),
			ReadinessProbe:  readinessProbe(),
			Env:             getRHSSOEnv(cr, dbSecret),
			Resources:       getResources(cr),
			ImagePullPolicy: cr.Spec.KeycloakDeploymentSpec.ImagePullPolicy,
		},
	}
	reconciled.Spec.Template.Spec.InitContainers = KeycloakExtensionsInitContainers(cr)
	if cr.Spec.KeycloakDeploymentSpec.Experimental.Affinity != nil {
		reconciled.Spec.Template.Spec.Affinity = cr.Spec.KeycloakDeploymentSpec.Experimental.Affinity
	}

	return reconciled
}
