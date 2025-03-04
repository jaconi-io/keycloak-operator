package keycloak

import (
	"testing"

	"github.com/jaconi-io/keycloak-operator/pkg/apis/keycloak/v1alpha1"
	"github.com/jaconi-io/keycloak-operator/pkg/common"
	"github.com/jaconi-io/keycloak-operator/pkg/model"
	kcAssert "github.com/jaconi-io/keycloak-operator/test/assert"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/apps/v1"
)

const extraLabelName = "extra"
const extraLabelValue = "value"

func TestKeycloakMigration_Test_No_Need_For_Migration_On_Empty_Desired_State(t *testing.T) {
	// given
	cr := &v1alpha1.Keycloak{}
	migrator, _ := GetMigrator(cr)
	currentState := common.ClusterState{}
	desiredState := common.DesiredClusterState{}

	// when
	migratedActions, error := migrator.Migrate(cr, &currentState, desiredState)

	// then
	assert.Nil(t, error)
	assert.Equal(t, desiredState, migratedActions)
}

func TestKeycloakMigration_Test_No_Need_For_Migration_On_Missing_Deployment_In_Desired_State(t *testing.T) {
	// given
	cr := &v1alpha1.Keycloak{}
	migrator, _ := GetMigrator(cr)

	keycloakDeployment := model.KeycloakDeployment(cr, nil, nil)
	SetDeployment(keycloakDeployment, 5, "old_image")

	currentState := common.ClusterState{
		KeycloakDeployment: keycloakDeployment,
	}

	desiredState := common.DesiredClusterState{}
	desiredState = append(desiredState, common.GenericUpdateAction{
		Ref: model.KeycloakService(cr),
	})

	// when
	migratedActions, error := migrator.Migrate(cr, &currentState, desiredState)

	// then
	assert.Nil(t, error)
	assert.Equal(t, desiredState, migratedActions)
}

func TestKeycloakMigration_Test_Migrating_Image(t *testing.T) {
	// given
	cr := &v1alpha1.Keycloak{}
	migrator, _ := GetMigrator(cr)

	keycloakCurrentDeployment := model.KeycloakDeployment(cr, model.DatabaseSecret(cr), nil)
	SetDeployment(keycloakCurrentDeployment, 5, "old_image")

	keycloakDesiredDeployment := model.KeycloakDeployment(cr, nil, nil)
	SetDeployment(keycloakDesiredDeployment, 5, "")

	currentState := common.ClusterState{
		KeycloakDeployment: keycloakCurrentDeployment,
	}

	desiredState := common.DesiredClusterState{}
	desiredState = append(desiredState, common.GenericUpdateAction{
		Ref: keycloakDesiredDeployment,
	})

	kcAssert.ReplicasCount(t, desiredState, 5)

	// when
	migratedActions, error := migrator.Migrate(cr, &currentState, desiredState)

	// then
	assert.Nil(t, error)
	assert.Equal(t, desiredState, migratedActions)
	kcAssert.ReplicasCount(t, migratedActions, 0)
}

func TestKeycloakMigration_Test_Migrating_RHSSO_Image(t *testing.T) {
	// given
	cr := &v1alpha1.Keycloak{
		Spec: v1alpha1.KeycloakSpec{
			Profile: model.RHSSOProfile,
		},
	}
	migrator, _ := GetMigrator(cr)

	keycloakCurrentDeployment := model.RHSSODeployment(cr, model.DatabaseSecret(cr), nil)
	SetDeployment(keycloakCurrentDeployment, 5, "old_image")

	keycloakDesiredDeployment := model.RHSSODeployment(cr, model.DatabaseSecret(cr), nil)
	SetDeployment(keycloakDesiredDeployment, 5, "")

	currentState := common.ClusterState{
		KeycloakDeployment: keycloakCurrentDeployment,
	}

	desiredState := common.DesiredClusterState{}
	desiredState = append(desiredState, common.GenericUpdateAction{
		Ref: keycloakDesiredDeployment,
	})

	kcAssert.ReplicasCount(t, desiredState, 5)

	// when
	migratedActions, error := migrator.Migrate(cr, &currentState, desiredState)

	// then
	assert.Nil(t, error)
	assert.Equal(t, desiredState, migratedActions)
	kcAssert.ReplicasCount(t, migratedActions, 0)
}

func TestKeycloakMigration_Test_No_Need_Backup_Without_Migration_Backups_Enabled(t *testing.T) {
	TBackup(t, false)
}

func TestKeycloakMigration_Test_Backup_Happens_With_Migration_Backups_Enabled(t *testing.T) {
	TBackup(t, true)
}

func TBackup(t *testing.T, backupEnabled bool) {
	// given
	cr := &v1alpha1.Keycloak{}
	cr.Spec.Migration.Backups.Enabled = backupEnabled
	migrator, _ := GetMigrator(cr)

	keycloakCurrentDeployment := model.KeycloakDeployment(cr, nil, nil)
	SetDeployment(keycloakCurrentDeployment, 0, "old_image")

	keycloakDesiredDeployment := model.KeycloakDeployment(cr, nil, nil)
	SetDeployment(keycloakDesiredDeployment, 0, "")

	currentState := common.ClusterState{
		KeycloakDeployment: keycloakCurrentDeployment,
	}

	desiredState := common.DesiredClusterState{}
	desiredState = append(desiredState, common.GenericUpdateAction{
		Ref: keycloakDesiredDeployment,
	})

	// when
	migratedActions, error := migrator.Migrate(cr, &currentState, desiredState)

	// then
	assert.Nil(t, error)
	if backupEnabled {
		assert.NotEqual(t, desiredState, migratedActions)
	} else {
		assert.Equal(t, desiredState, migratedActions)
	}
}

func TestKeycloakMigration_Test_No_Migration_Happens_With_Rolling_Migrator(t *testing.T) {
	// given
	cr := &v1alpha1.Keycloak{}
	cr.Spec.Migration.MigrationStrategy = v1alpha1.StrategyRolling
	migrator, _ := GetMigrator(cr)

	keycloakCurrentDeployment := model.RHSSODeployment(cr, model.DatabaseSecret(cr), nil)
	SetDeployment(keycloakCurrentDeployment, 5, "old_image")

	keycloakDesiredDeployment := model.RHSSODeployment(cr, model.DatabaseSecret(cr), nil)
	SetDeployment(keycloakDesiredDeployment, 5, "")

	currentState := common.ClusterState{
		KeycloakDeployment: keycloakCurrentDeployment,
	}

	desiredState := common.DesiredClusterState{}
	desiredState = append(desiredState, common.GenericUpdateAction{
		Ref: keycloakDesiredDeployment,
	})

	kcAssert.ReplicasCount(t, desiredState, 5)

	// when
	migratedActions, err := migrator.Migrate(cr, &currentState, desiredState)

	// then
	assert.Nil(t, err)
	assert.Equal(t, desiredState, migratedActions)
	kcAssert.ReplicasCount(t, migratedActions, 5)
}

func TestKeycloakMigration_Test_Stateful_Set_Selector_Error_With_Rolling_Migrator(t *testing.T) {
	// given
	cr := &v1alpha1.Keycloak{}
	cr.Spec.Migration.MigrationStrategy = v1alpha1.StrategyRolling
	cr.Spec.Instances = 3
	migrator, _ := GetMigrator(cr)

	keycloakCurrentDeployment := model.KeycloakDeployment(cr, model.DatabaseSecret(cr), nil)
	keycloakCurrentDeployment.Spec.Selector.MatchLabels[extraLabelName] = extraLabelValue
	SetDeployment(keycloakCurrentDeployment, 3, "")

	keycloakDesiredDeployment := model.KeycloakDeployment(cr, model.DatabaseSecret(cr), nil)
	SetDeployment(keycloakDesiredDeployment, 3, "")

	currentState := common.ClusterState{
		KeycloakDeployment: keycloakCurrentDeployment,
	}

	desiredState := common.DesiredClusterState{}
	desiredState = append(desiredState, common.GenericUpdateAction{
		Ref: keycloakDesiredDeployment,
	})

	kcAssert.ReplicasCount(t, desiredState, 3)

	// when
	migratedActions, err := migrator.Migrate(cr, &currentState, desiredState)

	// then
	assert.EqualError(t, err, "statefulSet Selector mismatch; please use Recreate migration strategy")
	assert.Nil(t, migratedActions)
}

func TestKeycloakMigration_Test_Stateful_Set_Downscaled_On_Selector_Mismatch(t *testing.T) {
	// given
	cr := &v1alpha1.Keycloak{}
	cr.Spec.Instances = 3
	migrator, _ := GetMigrator(cr)

	keycloakCurrentDeployment := model.KeycloakDeployment(cr, model.DatabaseSecret(cr), nil)
	keycloakCurrentDeployment.Spec.Selector.MatchLabels[extraLabelName] = extraLabelValue
	SetDeployment(keycloakCurrentDeployment, 3, "old_image")

	keycloakDesiredDeployment := model.KeycloakDeployment(cr, model.DatabaseSecret(cr), nil)
	SetDeployment(keycloakDesiredDeployment, 3, "")

	currentState := common.ClusterState{
		KeycloakDeployment: keycloakCurrentDeployment,
	}

	desiredState := common.DesiredClusterState{}
	desiredState = append(desiredState, common.GenericUpdateAction{
		Ref: keycloakDesiredDeployment,
	})

	kcAssert.ReplicasCount(t, desiredState, 3)

	// when
	migratedActions, err := migrator.Migrate(cr, &currentState, desiredState)

	// then
	assert.Nil(t, err)
	assert.Equal(t, desiredState, migratedActions)
	kcAssert.ReplicasCount(t, migratedActions, 0)
	kcAssert.KcDeploymentUpdated(t, migratedActions, keycloakDesiredDeployment, true)
	kcAssert.KcDeploymentRecreated(t, migratedActions, keycloakDesiredDeployment, false)
	// migrator actually modifies desired deployment
	assert.Equal(t, "old_image", keycloakDesiredDeployment.Spec.Template.Spec.Containers[0].Image)
}

func TestKeycloakMigration_Test_Stateful_Set_Recreated_After_Downscale(t *testing.T) {
	// given
	cr := &v1alpha1.Keycloak{}
	cr.Spec.Instances = 3
	migrator, _ := GetMigrator(cr)

	keycloakCurrentDeployment := model.KeycloakDeployment(cr, model.DatabaseSecret(cr), nil)
	keycloakCurrentDeployment.Spec.Selector.MatchLabels[extraLabelName] = extraLabelValue
	SetDeployment(keycloakCurrentDeployment, 3, "old_image")

	keycloakDesiredDeployment := model.KeycloakDeployment(cr, model.DatabaseSecret(cr), nil)
	keycloakCurrentDeployment.Status.Replicas = 0

	currentState := common.ClusterState{
		KeycloakDeployment: keycloakCurrentDeployment,
	}

	desiredState := common.DesiredClusterState{}
	desiredState = append(desiredState, common.GenericUpdateAction{
		Ref: keycloakDesiredDeployment,
	})

	kcAssert.ReplicasCount(t, desiredState, 3)

	// when
	migratedActions, err := migrator.Migrate(cr, &currentState, desiredState)

	// then
	assert.Nil(t, err)
	assert.NotEqual(t, desiredState, migratedActions)
	kcAssert.ReplicasCountRecreate(t, migratedActions, 3)
	kcAssert.KcDeploymentUpdated(t, migratedActions, keycloakDesiredDeployment, false)
	kcAssert.KcDeploymentRecreated(t, migratedActions, keycloakDesiredDeployment, true)
	// migrator actually modifies desired deployment
	assert.NotEqual(t, "old_image", keycloakDesiredDeployment.Spec.Template.Spec.Containers[0].Image)
}

func SetDeployment(deployment *v1.StatefulSet, replicasCount int32, image string) {
	deployment.Spec.Replicas = &[]int32{replicasCount}[0]
	deployment.Status.Replicas = replicasCount
	if image != "" {
		deployment.Spec.Template.Spec.Containers[0].Image = image
	}
}
