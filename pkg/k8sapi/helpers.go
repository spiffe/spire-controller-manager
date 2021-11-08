package k8sapi

import (
	"context"

	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ListClusterSPIFFEIDs(ctx context.Context, c client.Client) ([]spirev1alpha1.ClusterSPIFFEID, error) {
	var list spirev1alpha1.ClusterSPIFFEIDList
	if err := c.List(ctx, &list); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func ListClusterFederatedTrustDomains(ctx context.Context, c client.Client) ([]spirev1alpha1.ClusterFederatedTrustDomain, error) {
	var list spirev1alpha1.ClusterFederatedTrustDomainList
	if err := c.List(ctx, &list); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func ListNamespaces(ctx context.Context, c client.Client, namespaceSelector labels.Selector) ([]corev1.Namespace, error) {
	var opts []client.ListOption
	if namespaceSelector != nil {
		opts = append(opts, client.MatchingLabelsSelector{Selector: namespaceSelector})
	}
	list := new(corev1.NamespaceList)
	if err := c.List(ctx, list, opts...); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func ListNamespacePods(ctx context.Context, c client.Client, namespace string, podSelector labels.Selector) ([]corev1.Pod, error) {
	opts := []client.ListOption{
		client.InNamespace(namespace),
	}
	if podSelector != nil {
		opts = append(opts, client.MatchingLabelsSelector{Selector: podSelector})
	}
	list := new(corev1.PodList)
	if err := c.List(ctx, list, opts...); err != nil {
		return nil, err
	}
	return list.Items, nil
}
