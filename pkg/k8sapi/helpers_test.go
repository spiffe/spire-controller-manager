package k8sapi_test

import (
	"context"
	"errors"
	"testing"

	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/spiffe/spire-controller-manager/pkg/k8sapi"
	"github.com/spiffe/spire-controller-manager/pkg/test/k8stest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	errList = errors.New("list error")
)

func TestListClusterSPIFFEIDs(t *testing.T) {
	foo := spirev1alpha1.ClusterSPIFFEID{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
	}

	t.Run("list fails", func(t *testing.T) {
		client := FailList(k8stest.NewClientBuilder(t).Build())
		actual, err := k8sapi.ListClusterSPIFFEIDs(context.Background(), client)
		assert.EqualError(t, err, errList.Error())
		assert.Empty(t, actual)
	})

	t.Run("list empty", func(t *testing.T) {
		client := k8stest.NewClientBuilder(t).Build()
		actual, err := k8sapi.ListClusterSPIFFEIDs(context.Background(), client)
		assert.NoError(t, err)
		assert.Empty(t, actual)
	})

	t.Run("list not empty", func(t *testing.T) {
		client := k8stest.NewClientBuilder(t).WithRuntimeObjects(&foo).Build()
		actual, err := k8sapi.ListClusterSPIFFEIDs(context.Background(), client)
		assert.NoError(t, err)
		assert.Equal(t, []spirev1alpha1.ClusterSPIFFEID{foo}, actual)
	})
}

func TestListClusterFederatedTrustDomains(t *testing.T) {
	foo := spirev1alpha1.ClusterFederatedTrustDomain{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
	}

	t.Run("list fails", func(t *testing.T) {
		client := FailList(k8stest.NewClientBuilder(t).Build())
		actual, err := k8sapi.ListClusterFederatedTrustDomains(context.Background(), client)
		assert.EqualError(t, err, errList.Error())
		assert.Empty(t, actual)
	})

	t.Run("list empty", func(t *testing.T) {
		client := k8stest.NewClientBuilder(t).Build()
		actual, err := k8sapi.ListClusterFederatedTrustDomains(context.Background(), client)
		assert.NoError(t, err)
		assert.Empty(t, actual)
	})

	t.Run("list not empty", func(t *testing.T) {
		client := k8stest.NewClientBuilder(t).WithRuntimeObjects(&foo).Build()
		actual, err := k8sapi.ListClusterFederatedTrustDomains(context.Background(), client)
		assert.NoError(t, err)
		assert.Equal(t, []spirev1alpha1.ClusterFederatedTrustDomain{foo}, actual)
	})
}

func TestListNamespaces(t *testing.T) {
	ns1 := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "ns1", Labels: map[string]string{"widget": "foo"}},
	}
	ns2 := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "ns2", Labels: map[string]string{"widget": "bar"}},
	}

	t.Run("list fails", func(t *testing.T) {
		client := FailList(k8stest.NewClientBuilder(t).Build())
		actual, err := k8sapi.ListNamespaces(context.Background(), client, nil)
		assert.EqualError(t, err, errList.Error())
		assert.Empty(t, actual)
	})

	t.Run("list empty", func(t *testing.T) {
		client := fake.NewClientBuilder().Build()
		actual, err := k8sapi.ListNamespaces(context.Background(), client, nil)
		assert.NoError(t, err)
		assert.Empty(t, actual)
	})

	t.Run("list not empty", func(t *testing.T) {
		client := fake.NewClientBuilder().WithRuntimeObjects(&ns1, &ns2).Build()
		actual, err := k8sapi.ListNamespaces(context.Background(), client, nil)
		assert.NoError(t, err)
		assert.Equal(t, []corev1.Namespace{ns1, ns2}, actual)
	})

	t.Run("list filtered by labels", func(t *testing.T) {
		selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: ns2.Labels})
		require.NoError(t, err)

		client := fake.NewClientBuilder().WithRuntimeObjects(&ns1, &ns2).Build()
		actual, err := k8sapi.ListNamespaces(context.Background(), client, selector)
		assert.NoError(t, err)
		assert.Equal(t, []corev1.Namespace{ns2}, actual)
	})
}

func TestListNamespacePods(t *testing.T) {
	pod1 := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "pod1", Labels: map[string]string{"widget": "foo"}},
	}
	pod2 := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "pod2", Labels: map[string]string{"widget": "bar"}},
	}
	pod3 := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns2", Name: "pod3", Labels: map[string]string{"widget": "bar"}},
	}

	objects := []runtime.Object{&pod1, &pod2, &pod3}

	t.Run("list fails", func(t *testing.T) {
		client := FailList(k8stest.NewClientBuilder(t).Build())
		actual, err := k8sapi.ListNamespacePods(context.Background(), client, "ns1", nil)
		assert.EqualError(t, err, errList.Error())
		assert.Empty(t, actual)
	})

	t.Run("list empty", func(t *testing.T) {
		client := fake.NewClientBuilder().Build()
		actual, err := k8sapi.ListNamespacePods(context.Background(), client, "ns1", nil)
		assert.NoError(t, err)
		assert.Empty(t, actual)
	})

	t.Run("list not empty", func(t *testing.T) {
		client := fake.NewClientBuilder().WithRuntimeObjects(objects...).Build()
		actual, err := k8sapi.ListNamespacePods(context.Background(), client, "ns1", nil)
		assert.NoError(t, err)
		assert.Equal(t, []corev1.Pod{pod1, pod2}, actual)
	})

	t.Run("list filtered by labels", func(t *testing.T) {
		selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: pod2.Labels})
		require.NoError(t, err)

		client := fake.NewClientBuilder().WithRuntimeObjects(objects...).Build()
		actual, err := k8sapi.ListNamespacePods(context.Background(), client, "ns1", selector)
		assert.NoError(t, err)
		assert.Equal(t, []corev1.Pod{pod2}, actual)
	})
}

func FailList(c client.Client) client.Client {
	return failList{Client: c}
}

type failList struct {
	client.Client
}

func (c failList) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return errList
}
