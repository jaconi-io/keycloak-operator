package e2e

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/jaconi-io/keycloak-operator/pkg/common"

	"github.com/pkg/errors"

	keycloakv1alpha1 "github.com/jaconi-io/keycloak-operator/pkg/apis/keycloak/v1alpha1"
	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/types"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	dynclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Condition func(t *testing.T, c kubernetes.Interface) error

type ResponseCondition func(response *http.Response) error

type ClientCondition func(authenticatedClient common.KeycloakInterface) error

func WaitForCondition(t *testing.T, c kubernetes.Interface, cond Condition) error {
	t.Logf("waiting up to %v for condition", pollTimeout)
	var err error
	for start := time.Now(); time.Since(start) < pollTimeout; time.Sleep(pollRetryInterval) {
		err = cond(t, c)
		if err == nil {
			return nil
		}
	}
	return err
}

func WaitForConditionWithClient(t *testing.T, framework *framework.Framework, keycloakCR keycloakv1alpha1.Keycloak, cond ClientCondition) error {
	return WaitForCondition(t, framework.KubeClient, func(t *testing.T, c kubernetes.Interface) error {
		authenticatedClient, err := MakeAuthenticatedClient(keycloakCR)
		if err != nil {
			return err
		}
		return cond(authenticatedClient)
	})
}

func MakeAuthenticatedClient(keycloakCR keycloakv1alpha1.Keycloak) (common.KeycloakInterface, error) {
	keycloakFactory := common.LocalConfigKeycloakFactory{}
	return keycloakFactory.AuthenticatedClient(keycloakCR, true)
}

// Stolen from https://github.com/kubernetes/kubernetes/blob/master/test/e2e/framework/util.go
// Then rewritten to use internal condition statements.
func WaitForStatefulSetReplicasReady(t *testing.T, c kubernetes.Interface, statefulSetName, ns string) error {
	t.Logf("waiting up to %v for StatefulSet %s to have all replicas ready", pollTimeout, statefulSetName)
	return WaitForCondition(t, c, func(t *testing.T, c kubernetes.Interface) error {
		sts, err := c.AppsV1().StatefulSets(ns).Get(context.TODO(), statefulSetName, metav1.GetOptions{})
		if err != nil {
			return errors.Errorf("get StatefulSet %s failed, ignoring for %v: %v", statefulSetName, pollRetryInterval, err)
		}
		if sts.Status.ReadyReplicas == *sts.Spec.Replicas {
			t.Logf("all %d replicas of StatefulSet %s are ready.", sts.Status.ReadyReplicas, statefulSetName)
			return nil
		}
		return errors.Errorf("statefulSet %s found but there are %d ready replicas and %d total replicas", statefulSetName, sts.Status.ReadyReplicas, *sts.Spec.Replicas)
	})
}

func WaitForPodHavingLabels(t *testing.T, c kubernetes.Interface, podName, ns string, labels map[string]string) error {
	t.Logf("waiting up to %v for Pod %s to have all labels expected", pollTimeout, podName)
	return WaitForCondition(t, c, func(t *testing.T, c kubernetes.Interface) error {
		pod, err := c.CoreV1().Pods(ns).Get(context.TODO(), podName, metav1.GetOptions{})
		if err != nil {
			return errors.Errorf("get Pod %s failed, ignoring for %v: %v", podName, pollRetryInterval, err)
		}
		for key := range labels {
			if _, ok := pod.Labels[key]; !ok {
				return errors.Errorf("Pod %s doesn't have %s label, ignoring for %v: %v", podName, key, pollRetryInterval, err)
			}
		}
		return nil
	})
}

func WaitForPersistentVolumeClaimCreated(t *testing.T, c kubernetes.Interface, persistentVolumeClaimName, ns string) error {
	t.Logf("waiting up to %v for PersistentVolumeClaim %s to be created", pollTimeout, persistentVolumeClaimName)
	return WaitForCondition(t, c, func(t *testing.T, c kubernetes.Interface) error {
		pvc, err := c.CoreV1().PersistentVolumeClaims(ns).Get(context.TODO(), persistentVolumeClaimName, metav1.GetOptions{})
		if err != nil {
			return errors.Errorf("get PersistentVolumeClaim %s failed, ignoring for %v: %v", persistentVolumeClaimName, pollRetryInterval, err)
		}
		if pvc.Status.Phase == "Bound" {
			t.Logf("PersistentVolumeClaim is bound")
			return nil
		}
		return errors.Errorf("persistentVolumeClaim %s found but is not bound", persistentVolumeClaimName)
	})
}

func WaitForKeycloakToBeReady(t *testing.T, framework *framework.Framework, namespace string, name string) error {
	keycloakCR := &keycloakv1alpha1.Keycloak{}

	return WaitForCondition(t, framework.KubeClient, func(t *testing.T, c kubernetes.Interface) error {
		err := GetNamespacedObject(framework, namespace, name, keycloakCR)
		if err != nil {
			return err
		}

		if !keycloakCR.Status.Ready {
			keycloakCRParsed, err := json.Marshal(keycloakCR)
			if err != nil {
				return err
			}

			return errors.Errorf("keycloak is not ready \nCurrent CR value: %s", string(keycloakCRParsed))
		}

		return nil
	})
}

func WaitForRealmToBeReady(t *testing.T, framework *framework.Framework, namespace string) error {
	keycloakRealmCR := &keycloakv1alpha1.KeycloakRealm{}

	return WaitForCondition(t, framework.KubeClient, func(t *testing.T, c kubernetes.Interface) error {
		err := GetNamespacedObject(framework, namespace, testKeycloakRealmCRName, keycloakRealmCR)
		if err != nil {
			return err
		}

		if !keycloakRealmCR.Status.Ready {
			keycloakRealmCRParsed, err := json.Marshal(keycloakRealmCR)
			if err != nil {
				return err
			}

			return errors.Errorf("keycloakRealm is not ready \nCurrent CR value: %s", string(keycloakRealmCRParsed))
		}

		return nil
	})
}

func WaitForClientToBeReady(t *testing.T, framework *framework.Framework, namespace string, name string) error {
	keycloakClientCR := &keycloakv1alpha1.KeycloakClient{}

	return WaitForCondition(t, framework.KubeClient, func(t *testing.T, c kubernetes.Interface) error {
		err := GetNamespacedObject(framework, namespace, name, keycloakClientCR)
		if err != nil {
			return err
		}

		if !keycloakClientCR.Status.Ready {
			keycloakRealmCRParsed, err := json.Marshal(keycloakClientCR)
			if err != nil {
				return err
			}

			return errors.Errorf("keycloakClient is not ready \nCurrent CR value: %s", string(keycloakRealmCRParsed))
		}

		return nil
	})
}

func WaitForClientToBeFailing(t *testing.T, framework *framework.Framework, namespace string, name string) error {
	keycloakClientCR := &keycloakv1alpha1.KeycloakClient{}

	return WaitForCondition(t, framework.KubeClient, func(t *testing.T, c kubernetes.Interface) error {
		err := GetNamespacedObject(framework, namespace, name, keycloakClientCR)
		if err != nil {
			return err
		}

		if keycloakClientCR.Status.Phase != keycloakv1alpha1.PhaseFailing {
			keycloakRealmCRParsed, err := json.Marshal(keycloakClientCR)
			if err != nil {
				return err
			}

			return errors.Errorf("keycloakClient is not failing \nCurrent CR value: %s", string(keycloakRealmCRParsed))
		}

		return nil
	})
}

func WaitForUserToBeReady(t *testing.T, framework *framework.Framework, namespace string) error {
	keycloakUserCR := &keycloakv1alpha1.KeycloakUser{}

	return WaitForCondition(t, framework.KubeClient, func(t *testing.T, c kubernetes.Interface) error {
		err := GetNamespacedObject(framework, namespace, testKeycloakUserCRName, keycloakUserCR)
		if err != nil {
			return err
		}

		if keycloakUserCR.Status.Phase != keycloakv1alpha1.UserPhaseReconciled {
			keycloakRealmCRParsed, err := json.Marshal(keycloakUserCR)
			if err != nil {
				return err
			}

			return errors.Errorf("keycloakRealm is not ready \nCurrent CR value: %s", string(keycloakRealmCRParsed))
		}

		return nil
	})
}

func WaitForResponse(t *testing.T, framework *framework.Framework, url string, condition ResponseCondition) error {
	return WaitForCondition(t, framework.KubeClient, func(t *testing.T, c kubernetes.Interface) error {
		response, err := http.Get(url)
		if err != nil {
			return err
		}
		defer response.Body.Close()

		err = condition(response)
		if err != nil {
			return err
		}

		return nil
	})
}

func WaitForSuccessResponseToContain(t *testing.T, framework *framework.Framework, url string, expectedString string) error {
	return WaitForResponse(t, framework, url, func(response *http.Response) error {
		if response.StatusCode != 200 {
			return errors.Errorf("invalid response from url %s (%v)", url, response.Status)
		}

		responseData, err := io.ReadAll(response.Body)
		if err != nil {
			return err
		}
		responseString := string(responseData)

		assert.Contains(t, responseString, expectedString)

		return nil
	})
}

func WaitForSuccessResponse(t *testing.T, framework *framework.Framework, url string) error {
	return WaitForResponse(t, framework, url, func(response *http.Response) error {
		if response.StatusCode != 200 {
			return errors.Errorf("invalid response from url %s (%v)", url, response.Status)
		}
		return nil
	})
}

func Create(f *framework.Framework, obj runtime.Object, ctx *framework.Context) error {
	return f.Client.Create(context.TODO(), obj, &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
}

func Get(f *framework.Framework, key dynclient.ObjectKey, obj runtime.Object) error {
	return f.Client.Get(context.TODO(), key, obj)
}

func GetNamespacedObject(f *framework.Framework, namespace string, objectName string, outputObject runtime.Object) error {
	key := types.NamespacedName{
		Namespace: namespace,
		Name:      objectName,
	}

	return Get(f, key, outputObject)
}

func Update(f *framework.Framework, obj runtime.Object) error {
	return f.Client.Update(context.TODO(), obj)
}

func Delete(f *framework.Framework, obj runtime.Object) error {
	return f.Client.Delete(context.TODO(), obj)
}

func CreateLabel(namespace string) map[string]string {
	return map[string]string{"app": "kc-in-" + namespace}
}

func CreateExternalLabel(namespace string) map[string]string {
	return map[string]string{"app": "ext-kc-in-" + namespace}
}

func GetSuccessfulResponseBody(url string) ([]byte, error) {
	client := &http.Client{}
	response, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	ret, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
