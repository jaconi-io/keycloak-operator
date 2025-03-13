package common

import (
	"github.com/pkg/errors"
	v12 "k8s.io/api/apps/v1"
	v13 "k8s.io/api/batch/v1"
)

const (
	ConditionStatusSuccess = "True"
)

func IsStatefulSetReady(statefulSet *v12.StatefulSet) (bool, error) {
	if statefulSet == nil {
		return false, nil
	}
	// Check the correct number of replicas match and are ready
	numOfReplicasMatch := *statefulSet.Spec.Replicas == statefulSet.Status.Replicas
	allReplicasReady := statefulSet.Status.Replicas == statefulSet.Status.ReadyReplicas
	revisionsMatch := statefulSet.Status.CurrentRevision == statefulSet.Status.UpdateRevision

	return numOfReplicasMatch && allReplicasReady && revisionsMatch, nil
}

func IsDeploymentReady(deployment *v12.Deployment) (bool, error) {
	if deployment == nil {
		return false, nil
	}
	// A deployment has an array of conditions
	for _, condition := range deployment.Status.Conditions {
		// One failure condition exists, if this exists, return the Reason
		if condition.Type == v12.DeploymentReplicaFailure {
			return false, errors.New(condition.Reason)
			// A successful deployment will have the progressing condition type as true
		} else if condition.Type == v12.DeploymentProgressing && condition.Status != ConditionStatusSuccess {
			return false, nil
		}
	}
	return true, nil
}

func IsJobReady(job *v13.Job) (bool, error) {
	if job == nil {
		return false, nil
	}

	return job.Status.Succeeded == 1, nil
}
