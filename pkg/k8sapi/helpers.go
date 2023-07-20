/*
Copyright 2021 SPIRE Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package k8sapi

import (
	"context"

	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ListClusterStaticEntries(ctx context.Context, c client.Client) ([]spirev1alpha1.ClusterStaticEntry, error) {
	var list spirev1alpha1.ClusterStaticEntryList
	if err := c.List(ctx, &list); err != nil {
		return nil, err
	}
	return list.Items, nil
}

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
