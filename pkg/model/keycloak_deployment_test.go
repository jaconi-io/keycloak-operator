package model

import (
	"fmt"
	"testing"

	"github.com/jaconi-io/keycloak-operator/pkg/apis/keycloak/v1alpha1"
	"github.com/stretchr/testify/assert"
	v13 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type createDeploymentStatefulSet func(*v1alpha1.Keycloak, *v1.Secret, *v1.Secret) *v13.StatefulSet
type reconciledDeployment func(*v1alpha1.Keycloak, *v13.StatefulSet, *v1.Secret, *v1.Secret) *v13.StatefulSet

func TestKeycloakDeployment_testExperimentalEnvs(t *testing.T) {
	testExperimentalEnvs(t, KeycloakDeployment)
}

func TestKeycloakDeployment_testExperimentalArgs(t *testing.T) {
	testExperimentalArgs(t, KeycloakDeployment)
}

func TestKeycloakDeployment_testExperimentalCommand(t *testing.T) {
	testExperimentalCommand(t, KeycloakDeployment)
}

func TestKeycloakDeployment_testExperimentalVolumesWithConfigMaps(t *testing.T) {
	testExperimentalVolumesWithConfigMaps(t, KeycloakDeployment)
}

func TestKeycloakDeployment_testExperimentalVolumesWithSecrets(t *testing.T) {
	testExperimentalVolumesWithSecrets(t, KeycloakDeployment)
}

func TestKeycloakDeployment_testExperimentalVolumesWithConfigMapsAndSecrets(t *testing.T) {
	testExperimentalVolumesWithConfigMapsAndSecrets(t, KeycloakDeployment)
}

func TestKeycloakDeployment_testPostgresEnvs(t *testing.T) {
	testPostgresEnvs(t, KeycloakDeployment)
}

func TestKeycloakDeployment_testAffinityDefaultMultiAZ(t *testing.T) {
	testAffinityDefaultMultiAZ(t, KeycloakDeployment)
}

func TestKeycloakDeployment_testAffinityExperimental(t *testing.T) {
	testAffinityExperimentalAffinitySet(t, KeycloakDeployment)
}

func TestKeycloakDeployment_testServiceAccountSetExperimental(t *testing.T) {
	testServiceAccountSet(t, KeycloakDeployment)
}

func TestKeycloakDeployment_testServiceAccountReconciledSetExperimental(t *testing.T) {
	testServiceAccountReconciledSet(t, KeycloakDeployment, KeycloakDeploymentReconciled)
}

func TestKeycloakDeployment_testDeploymentSpecImagePolicy(t *testing.T) {
	testDeploymentSpecImagePolicy(t, KeycloakDeployment)
}

func TestKeycloakDeploymentReconciled_testDisableReplicasSyncingFalse(t *testing.T) {
	testDisableDeploymentReplicasSyncingFalse(t, KeycloakDeployment, KeycloakDeploymentReconciled)
}

func TestKeycloakDeploymentReconciled_testDisableReplicasSyncingTrue(t *testing.T) {
	testDisableDeploymentReplicasSyncingTrue(t, KeycloakDeployment, KeycloakDeploymentReconciled)
}

func testExperimentalEnvs(t *testing.T, deploymentFunction createDeploymentStatefulSet) {
	//given
	dbSecret := &v1.Secret{}
	cr := &v1alpha1.Keycloak{
		Spec: v1alpha1.KeycloakSpec{
			KeycloakDeploymentSpec: v1alpha1.KeycloakDeploymentSpec{
				Experimental: v1alpha1.ExperimentalSpec{
					Env: []v1.EnvVar{
						{
							// New value
							Name:  "testName",
							Value: "testValue",
						},
						{
							// An overridden value
							Name:  "DB_SCHEMA",
							Value: "test",
						},
					},
				},
			},
		},
	}

	//when
	envs := deploymentFunction(cr, dbSecret, nil).Spec.Template.Spec.Containers[0].Env

	//then
	hasTestNameKey := false
	testNameValue := ""
	dbSchemaValue := ""
	for _, v := range envs {
		if v.Name == "testName" {
			hasTestNameKey = true
			testNameValue = v.Value
		}
		if v.Name == "DB_SCHEMA" {
			dbSchemaValue = v.Value
		}
	}
	assert.True(t, hasTestNameKey)
	assert.Equal(t, "testValue", testNameValue)
	assert.Equal(t, "test", dbSchemaValue)
	assert.True(t, len(envs) > 1)
}

func testExperimentalArgs(t *testing.T, deploymentFunction createDeploymentStatefulSet) {
	//given
	dbSecret := &v1.Secret{}
	cr := &v1alpha1.Keycloak{
		Spec: v1alpha1.KeycloakSpec{
			KeycloakDeploymentSpec: v1alpha1.KeycloakDeploymentSpec{
				Experimental: v1alpha1.ExperimentalSpec{
					Args: []string{"test"},
				},
			},
		},
	}

	//when
	args := deploymentFunction(cr, dbSecret, nil).Spec.Template.Spec.Containers[0].Args

	//then
	assert.Equal(t, []string{"test"}, args)
}

func testDeploymentSpecImagePolicy(t *testing.T, deploymentFunction createDeploymentStatefulSet) {
	//given
	dbSecret := &v1.Secret{}
	cr := &v1alpha1.Keycloak{}
	cr.Spec.KeycloakDeploymentSpec = v1alpha1.KeycloakDeploymentSpec{
		DeploymentSpec: v1alpha1.DeploymentSpec{
			ImagePullPolicy: v1.PullNever,
		},
	}

	//when
	imagePullPolicyContainer := deploymentFunction(cr, dbSecret, nil).Spec.Template.Spec.Containers[0].ImagePullPolicy

	//then
	assert.Equal(t, v1.PullNever, imagePullPolicyContainer)
}

func testExperimentalCommand(t *testing.T, deploymentFunction createDeploymentStatefulSet) {
	//given
	dbSecret := &v1.Secret{}
	cr := &v1alpha1.Keycloak{
		Spec: v1alpha1.KeycloakSpec{
			KeycloakDeploymentSpec: v1alpha1.KeycloakDeploymentSpec{
				Experimental: v1alpha1.ExperimentalSpec{
					Command: []string{"test"},
				},
			},
		},
	}

	//when
	command := deploymentFunction(cr, dbSecret, nil).Spec.Template.Spec.Containers[0].Command

	//then
	assert.Equal(t, []string{"test"}, command)
}

func testExperimentalVolumesWithConfigMaps(t *testing.T, deploymentFunction createDeploymentStatefulSet) {
	//given
	dbSecret := &v1.Secret{}
	cr := &v1alpha1.Keycloak{
		Spec: v1alpha1.KeycloakSpec{
			KeycloakDeploymentSpec: v1alpha1.KeycloakDeploymentSpec{
				Experimental: v1alpha1.ExperimentalSpec{
					Volumes: v1alpha1.VolumesSpec{
						Items: []v1alpha1.VolumeSpec{{
							Name:       "testName",
							MountPath:  "testMountPath",
							ConfigMaps: []string{"ConfigMap1", "ConfigMap2"},
							Items: []v1.KeyToPath{
								{
									Key:  "testKey",
									Path: "testPath",
								},
							},
						}},
						DefaultMode: &[]int32{1}[0],
					},
				},
			},
		},
	}

	//when
	template := deploymentFunction(cr, dbSecret, nil).Spec.Template.Spec
	volumeMount := template.Containers[0].VolumeMounts[3]
	volume := template.Volumes[3]

	//then
	assert.Equal(t, "testName", volumeMount.Name)
	assert.Equal(t, "testMountPath", volumeMount.MountPath)
	assert.Equal(t, "testName", volume.Name)
	assert.Equal(t, "ConfigMap1", volume.Projected.Sources[0].ConfigMap.Name)
	assert.Equal(t, "testKey", volume.Projected.Sources[0].ConfigMap.Items[0].Key)
	assert.Equal(t, "testPath", volume.Projected.Sources[0].ConfigMap.Items[0].Path)
	assert.Equal(t, "ConfigMap2", volume.Projected.Sources[1].ConfigMap.Name)
	assert.Equal(t, "testKey", volume.Projected.Sources[1].ConfigMap.Items[0].Key)
	assert.Equal(t, "testPath", volume.Projected.Sources[1].ConfigMap.Items[0].Path)
}

func testExperimentalVolumesWithSecrets(t *testing.T, deploymentFunction createDeploymentStatefulSet) {
	//given
	dbSecret := &v1.Secret{}
	cr := &v1alpha1.Keycloak{
		Spec: v1alpha1.KeycloakSpec{
			KeycloakDeploymentSpec: v1alpha1.KeycloakDeploymentSpec{
				Experimental: v1alpha1.ExperimentalSpec{
					Volumes: v1alpha1.VolumesSpec{
						Items: []v1alpha1.VolumeSpec{{
							Name:      "testName",
							MountPath: "testMountPath",
							Secrets:   []string{"Secret1", "Secret2"},
							Items: []v1.KeyToPath{
								{
									Key:  "testKey",
									Path: "testPath",
								},
							},
						}},
						DefaultMode: &[]int32{1}[0],
					},
				},
			},
		},
	}

	//when
	template := deploymentFunction(cr, dbSecret, nil).Spec.Template.Spec
	volumeMount := template.Containers[0].VolumeMounts[3]
	volume := template.Volumes[3]

	//then
	assert.Equal(t, "testName", volumeMount.Name)
	assert.Equal(t, "testMountPath", volumeMount.MountPath)
	assert.Equal(t, "testName", volume.Name)
	assert.Equal(t, "Secret1", volume.Projected.Sources[0].Secret.Name)
	assert.Equal(t, "testKey", volume.Projected.Sources[0].Secret.Items[0].Key)
	assert.Equal(t, "testPath", volume.Projected.Sources[0].Secret.Items[0].Path)
	assert.Equal(t, "Secret2", volume.Projected.Sources[1].Secret.Name)
	assert.Equal(t, "testKey", volume.Projected.Sources[1].Secret.Items[0].Key)
	assert.Equal(t, "testPath", volume.Projected.Sources[1].Secret.Items[0].Path)
}
func testExperimentalVolumesWithConfigMapsAndSecrets(t *testing.T, deploymentFunction createDeploymentStatefulSet) {
	//given
	dbSecret := &v1.Secret{}
	cr := &v1alpha1.Keycloak{
		Spec: v1alpha1.KeycloakSpec{
			KeycloakDeploymentSpec: v1alpha1.KeycloakDeploymentSpec{
				Experimental: v1alpha1.ExperimentalSpec{
					Volumes: v1alpha1.VolumesSpec{
						Items: []v1alpha1.VolumeSpec{{
							Name:       "testName",
							MountPath:  "testMountPath",
							Secrets:    []string{"Secret1", "Secret2"},
							ConfigMaps: []string{"ConfigMap1", "ConfigMap2"},
							Items: []v1.KeyToPath{
								{
									Key:  "testKey",
									Path: "testPath",
								},
							},
						}},
						DefaultMode: &[]int32{1}[0],
					},
				},
			},
		},
	}

	//when
	template := deploymentFunction(cr, dbSecret, nil).Spec.Template.Spec
	volumeMount := template.Containers[0].VolumeMounts[3]
	volume := template.Volumes[3]

	//then
	assert.Equal(t, "testName", volumeMount.Name)
	assert.Equal(t, "testMountPath", volumeMount.MountPath)
	assert.Equal(t, "testName", volume.Name)
	assert.Equal(t, "ConfigMap1", volume.Projected.Sources[0].ConfigMap.Name)
	assert.Equal(t, "testKey", volume.Projected.Sources[0].ConfigMap.Items[0].Key)
	assert.Equal(t, "testPath", volume.Projected.Sources[0].ConfigMap.Items[0].Path)
	assert.Equal(t, "ConfigMap2", volume.Projected.Sources[1].ConfigMap.Name)
	assert.Equal(t, "testKey", volume.Projected.Sources[1].ConfigMap.Items[0].Key)
	assert.Equal(t, "testPath", volume.Projected.Sources[1].ConfigMap.Items[0].Path)
	assert.Equal(t, "Secret1", volume.Projected.Sources[2].Secret.Name)
	assert.Equal(t, "testKey", volume.Projected.Sources[2].Secret.Items[0].Key)
	assert.Equal(t, "testPath", volume.Projected.Sources[2].Secret.Items[0].Path)
	assert.Equal(t, "Secret2", volume.Projected.Sources[3].Secret.Name)
	assert.Equal(t, "testKey", volume.Projected.Sources[3].Secret.Items[0].Key)
	assert.Equal(t, "testPath", volume.Projected.Sources[3].Secret.Items[0].Path)
}

func testPostgresEnvs(t *testing.T, deploymentFunction createDeploymentStatefulSet) {
	//given
	cr := &v1alpha1.Keycloak{}

	//when
	envs := deploymentFunction(cr, nil, nil).Spec.Template.Spec.Containers[0].Env

	//then
	assert.Equal(t, getEnvValueByName(envs, "DB_VENDOR"), "POSTGRES")
	assert.Equal(t, getEnvValueByName(envs, "DB_SCHEMA"), "public")
	assert.Equal(t, getEnvValueByName(envs, "DB_ADDR"), PostgresqlServiceName+"."+cr.Namespace)
	assert.True(t, getEnvValueByName(envs, "DB_PORT") != "")
	assert.Equal(t, getEnvValueByName(envs, "DB_PORT"), fmt.Sprintf("%v", PostgresDefaultPort))
	assert.Equal(t, getEnvValueByName(envs, "DB_DATABASE"), PostgresqlDatabase)

	//given
	cr = &v1alpha1.Keycloak{
		Spec: v1alpha1.KeycloakSpec{
			ExternalDatabase: v1alpha1.KeycloakExternalDatabase{
				Enabled: true,
			},
		},
	}

	//when
	dbSecret := &v1.Secret{
		Data: map[string][]byte{
			DatabaseSecretDatabaseProperty:        []byte("test"),
			DatabaseSecretExternalAddressProperty: []byte("postgres.example.com"),
			DatabaseSecretExternalPortProperty:    []byte("12345"),
		},
	}
	envs = deploymentFunction(cr, dbSecret, nil).Spec.Template.Spec.Containers[0].Env

	//then
	assert.Equal(t, "POSTGRES", getEnvValueByName(envs, "DB_VENDOR"))
	assert.Equal(t, "public", getEnvValueByName(envs, "DB_SCHEMA"))
	assert.Equal(t, PostgresqlServiceName+"."+cr.Namespace, getEnvValueByName(envs, "DB_ADDR"))
	assert.True(t, getEnvValueByName(envs, "DB_PORT") != "")
	assert.Equal(t, "12345", getEnvValueByName(envs, "DB_PORT"))
	assert.Equal(t, "test", getEnvValueByName(envs, "DB_DATABASE"))
}

func getEnvValueByName(envs []v1.EnvVar, name string) string {
	for _, v := range envs {
		if v.Name == name {
			return v.Value
		}
	}
	return ""
}

func testAffinityDefaultMultiAZ(t *testing.T, deploymentFunction createDeploymentStatefulSet) {
	//given
	dbSecret := &v1.Secret{}
	cr := &v1alpha1.Keycloak{}

	cr.Spec.MultiAvailablityZones.Enabled = true

	//when
	affinity := deploymentFunction(cr, dbSecret, nil).Spec.Template.Spec.Affinity

	weight0 := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight
	matchExprKey0 := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.LabelSelector.MatchExpressions[0].Key
	matchExprOperator0 := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.LabelSelector.MatchExpressions[0].Operator
	matchExpVal0 := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.LabelSelector.MatchExpressions[0].Values[0]
	topologyKey0 := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.TopologyKey

	weight1 := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[1].Weight
	matchExprKey1 := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[1].PodAffinityTerm.LabelSelector.MatchExpressions[0].Key
	matchExprOperator1 := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[1].PodAffinityTerm.LabelSelector.MatchExpressions[0].Operator
	matchExpVal1 := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[1].PodAffinityTerm.LabelSelector.MatchExpressions[0].Values[0]
	topologyKey1 := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[1].PodAffinityTerm.TopologyKey

	//then - Expect default values for Affinity
	assert.Equal(t, int32(100), weight0)
	assert.Equal(t, "app", matchExprKey0)
	assert.Equal(t, "In", string(matchExprOperator0))
	assert.Equal(t, ApplicationName, matchExpVal0)
	assert.Equal(t, "topology.kubernetes.io/zone", topologyKey0)

	assert.Equal(t, int32(90), weight1)
	assert.Equal(t, "app", matchExprKey1)
	assert.Equal(t, "In", string(matchExprOperator1))
	assert.Equal(t, ApplicationName, matchExpVal1)
	assert.Equal(t, "kubernetes.io/hostname", topologyKey1)
}

func testAffinityExperimentalAffinitySet(t *testing.T, deploymentFunction createDeploymentStatefulSet) {
	//given
	dbSecret := &v1.Secret{}
	cr := &v1alpha1.Keycloak{}

	//If expoeriemntal->affinity is defined by the user, The user defined values
	//are used even if multiAvalabilityZones are enabled i.e. the default affinity settings
	//wont be applied.
	cr.Spec.MultiAvailablityZones.Enabled = true
	cr.Spec.KeycloakDeploymentSpec.Experimental.Affinity = &v1.Affinity{
		PodAntiAffinity: &v1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []v1.WeightedPodAffinityTerm{
				{
					Weight: 95,
					PodAffinityTerm: v1.PodAffinityTerm{
						LabelSelector: &v12.LabelSelector{
							MatchExpressions: []v12.LabelSelectorRequirement{
								{
									Key:      "app",
									Operator: "In",
									Values: []string{
										ApplicationName,
									},
								},
							},
						},
						TopologyKey: "topology.kubernetes.io/zone",
					},
				},
				{
					Weight: 75,
					PodAffinityTerm: v1.PodAffinityTerm{
						LabelSelector: &v12.LabelSelector{
							MatchExpressions: []v12.LabelSelectorRequirement{
								{
									Key:      "app",
									Operator: "In",
									Values: []string{
										ApplicationName,
									},
								},
							},
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			},
		},
	}

	//when
	affinity := deploymentFunction(cr, dbSecret, nil).Spec.Template.Spec.Affinity

	weight0 := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight
	matchExprKey0 := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.LabelSelector.MatchExpressions[0].Key
	matchExprOperator0 := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.LabelSelector.MatchExpressions[0].Operator
	matchExpVal0 := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.LabelSelector.MatchExpressions[0].Values[0]
	topologyKey0 := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.TopologyKey

	weight1 := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[1].Weight
	matchExprKey1 := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[1].PodAffinityTerm.LabelSelector.MatchExpressions[0].Key
	matchExprOperator1 := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[1].PodAffinityTerm.LabelSelector.MatchExpressions[0].Operator
	matchExpVal1 := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[1].PodAffinityTerm.LabelSelector.MatchExpressions[0].Values[0]
	topologyKey1 := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[1].PodAffinityTerm.TopologyKey

	//then - Expect default values for Affinity
	assert.Equal(t, int32(95), weight0)
	assert.Equal(t, "app", matchExprKey0)
	assert.Equal(t, "In", string(matchExprOperator0))
	assert.Equal(t, ApplicationName, matchExpVal0)
	assert.Equal(t, "topology.kubernetes.io/zone", topologyKey0)

	assert.Equal(t, int32(75), weight1)
	assert.Equal(t, "app", matchExprKey1)
	assert.Equal(t, "In", string(matchExprOperator1))
	assert.Equal(t, ApplicationName, matchExpVal1)
	assert.Equal(t, "kubernetes.io/hostname", topologyKey1)
}

func testServiceAccountSet(t *testing.T, deploymentFunction createDeploymentStatefulSet) {
	//given
	dbSecret := &v1.Secret{}
	cr := &v1alpha1.Keycloak{}

	//If serviceAccountName is set in the cr, is should manifest itself in the statefulset
	cr.Spec.KeycloakDeploymentSpec.Experimental.ServiceAccountName = "test"

	//when
	serviceAccountName := deploymentFunction(cr, dbSecret, nil).Spec.Template.Spec.ServiceAccountName

	assert.Equal(t, "test", serviceAccountName)
}

func testServiceAccountReconciledSet(t *testing.T, deploymentFunction createDeploymentStatefulSet, reconciliationFunction reconciledDeployment) {
	//given
	dbSecret := &v1.Secret{}
	cr := &v1alpha1.Keycloak{}
	statefulSet := deploymentFunction(cr, dbSecret, nil)

	//when

	//If serviceAccountName is set in the cr, is should manifest itself in the statefulset
	cr.Spec.KeycloakDeploymentSpec.Experimental.ServiceAccountName = "test2"
	serviceAccountName := reconciliationFunction(cr, statefulSet, dbSecret, nil).Spec.Template.Spec.ServiceAccountName

	assert.Equal(t, "test2", serviceAccountName)
}

func testDisableDeploymentReplicasSyncingFalse(t *testing.T, deploymentFunction createDeploymentStatefulSet, deploymentFunction2 reconciledDeployment) {
	//given
	dbSecret := &v1.Secret{}
	cr := &v1alpha1.Keycloak{
		Spec: v1alpha1.KeycloakSpec{
			Instances: 2,
		},
	}
	statefulSet := deploymentFunction(cr, dbSecret, nil)

	// when
	statefulSet.Spec.Replicas = &[]int32{4}[0]
	replicasCount := deploymentFunction2(cr, statefulSet, dbSecret, nil).Spec.Replicas

	//then
	assert.Equal(t, int32(2), *replicasCount)
}

func testDisableDeploymentReplicasSyncingTrue(t *testing.T, deploymentFunction createDeploymentStatefulSet, deploymentFunction2 reconciledDeployment) {
	//given
	dbSecret := &v1.Secret{}
	cr := &v1alpha1.Keycloak{
		Spec: v1alpha1.KeycloakSpec{
			Instances:              2,
			DisableReplicasSyncing: true,
		},
	}
	statefulSet := deploymentFunction(cr, dbSecret, nil)

	//when
	statefulSet.Spec.Replicas = &[]int32{4}[0]
	replicasCountAfterSyncing := deploymentFunction2(cr, statefulSet, dbSecret, nil).Spec.Replicas

	//then
	assert.Equal(t, int32(4), *replicasCountAfterSyncing)
}
