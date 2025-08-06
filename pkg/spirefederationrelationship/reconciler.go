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

package spirefederationrelationship

import (
	"context"
	"sort"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/spiffe/spire-controller-manager/pkg/k8sapi"
	"github.com/spiffe/spire-controller-manager/pkg/reconciler"
	"github.com/spiffe/spire-controller-manager/pkg/spireapi"
	"google.golang.org/grpc/codes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ReconcilerConfig struct {
	TrustDomainClient        spireapi.TrustDomainClient
	K8sClient                client.Client
	ClassName                string
	WatchClassless           bool
	StaticManifestPath       *string
	ExpandEnvStaticManifests bool

	// GCInterval how long to sit idle (i.e. untriggered) before doing
	// another reconcile.
	GCInterval time.Duration
}

func Reconciler(config ReconcilerConfig) reconciler.Reconciler {
	return reconciler.New(reconciler.Config{
		Kind: "federation relationship",
		Reconcile: func(ctx context.Context) {
			Reconcile(ctx, config.TrustDomainClient, config.K8sClient, config.ClassName, config.WatchClassless, config.StaticManifestPath, config.ExpandEnvStaticManifests)
		},
		GCInterval: config.GCInterval,
	})
}

func Reconcile(ctx context.Context, trustDomainClient spireapi.TrustDomainClient, k8sClient client.Client, className string, watchClassless bool, staticManifestPath *string, expandEnvStaticManifests bool) {
	r := &federationRelationshipReconciler{
		trustDomainClient:        trustDomainClient,
		k8sClient:                k8sClient,
		className:                className,
		watchClassless:           watchClassless,
		staticManifestPath:       staticManifestPath,
		expandEnvStaticManifests: expandEnvStaticManifests,
	}
	r.reconcile(ctx)
}

type federationRelationshipReconciler struct {
	trustDomainClient        spireapi.TrustDomainClient
	k8sClient                client.Client
	className                string
	watchClassless           bool
	staticManifestPath       *string
	expandEnvStaticManifests bool
}

func (r *federationRelationshipReconciler) reconcile(ctx context.Context) {
	log := log.FromContext(ctx)

	currentRelationships, err := r.listFederationRelationships(ctx)
	if err != nil {
		log.Error(err, "Failed to list SPIRE federation relationships")
		return
	}

	clusterFederatedTrustDomains, err := r.listClusterFederatedTrustDomains(ctx, r.expandEnvStaticManifests)
	if err != nil {
		log.Error(err, "Failed to list ClusterFederatedTrustDomains")
		return
	}

	var toDelete []spireapi.FederationRelationship
	var toCreate []spireapi.FederationRelationship
	var toUpdate []spireapi.FederationRelationship

	for trustDomain, federationRelationship := range currentRelationships {
		if _, ok := clusterFederatedTrustDomains[trustDomain]; !ok {
			toDelete = append(toDelete, federationRelationship)
		}
	}
	for trustDomain, clusterFederatedTrustDomain := range clusterFederatedTrustDomains {
		currentRelationship, ok := currentRelationships[trustDomain]
		switch {
		case !ok:
			toCreate = append(toCreate, clusterFederatedTrustDomain.FederationRelationship)
		case !currentRelationship.Equal(clusterFederatedTrustDomain.FederationRelationship):
			toUpdate = append(toUpdate, clusterFederatedTrustDomain.FederationRelationship)
		}
	}

	if len(toDelete) > 0 {
		r.deleteFederationRelationships(ctx, toDelete)
	}
	if len(toCreate) > 0 {
		r.createFederationRelationships(ctx, toCreate)
	}
	if len(toUpdate) > 0 {
		r.updateFederationRelationships(ctx, toUpdate)
	}

	// TODO: Status updates
}

func (r *federationRelationshipReconciler) reconcileClass(className string) bool {
	return (className == "" && r.watchClassless) || className == r.className
}

func (r *federationRelationshipReconciler) listFederationRelationships(ctx context.Context) (map[spiffeid.TrustDomain]spireapi.FederationRelationship, error) {
	federationRelationships, err := r.trustDomainClient.ListFederationRelationships(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[spiffeid.TrustDomain]spireapi.FederationRelationship)
	for _, federationRelationship := range federationRelationships {
		out[federationRelationship.TrustDomain] = federationRelationship
	}
	return out, nil
}

func (r *federationRelationshipReconciler) listClusterFederatedTrustDomains(ctx context.Context, expandEnv bool) (map[spiffeid.TrustDomain]*clusterFederatedTrustDomainState, error) {
	log := log.FromContext(ctx)

	var clusterFederatedTrustDomains []spirev1alpha1.ClusterFederatedTrustDomain
	var err error
	if r.k8sClient != nil {
		clusterFederatedTrustDomains, err = k8sapi.ListClusterFederatedTrustDomains(ctx, r.k8sClient)
	} else {
		clusterFederatedTrustDomains, err = spirev1alpha1.ListClusterFederatedTrustDomains(ctx, *r.staticManifestPath, expandEnv)
	}
	if err != nil {
		return nil, err
	}

	// Sort the cluster federated trust domains by creation date. This provides
	// stable sorting with trust domain conflict detection below, without which
	// the reconciliation could lead to waffling between which CRD to reconcile
	// against the SPIRE federated relationships.
	sortClusterFederatedTrustDomainsByCreationDate(clusterFederatedTrustDomains)

	out := make(map[spiffeid.TrustDomain]*clusterFederatedTrustDomainState, len(clusterFederatedTrustDomains))
	for i := range clusterFederatedTrustDomains {
		if !(r.reconcileClass(clusterFederatedTrustDomains[i].Spec.ClassName)) {
			continue
		}
		log := log.WithValues(clusterFederatedTrustDomainLogKey, objectName(&clusterFederatedTrustDomains[i]))

		federationRelationship, err := spirev1alpha1.ParseClusterFederatedTrustDomainSpec(&clusterFederatedTrustDomains[i].Spec)
		if err != nil {
			log.Error(err, "Ignoring invalid ClusterFederatedTrustDomain")
			continue
		}

		state := &clusterFederatedTrustDomainState{
			ClusterFederatedTrustDomain: clusterFederatedTrustDomains[i],
			FederationRelationship:      *federationRelationship,
		}

		if existing, ok := out[federationRelationship.TrustDomain]; ok {
			log.Info("Ignoring ClusterFederatedTrustDomain with conflicting trust domain",
				conflictWithKey, objectName(&existing.ClusterFederatedTrustDomain))
			continue
		}

		out[federationRelationship.TrustDomain] = state
	}
	return out, nil
}

func (r *federationRelationshipReconciler) createFederationRelationships(ctx context.Context, federationRelationships []spireapi.FederationRelationship) {
	log := log.FromContext(ctx)

	statuses, err := r.trustDomainClient.CreateFederationRelationships(ctx, federationRelationships)
	if err != nil {
		log.Error(err, "Failed to create federation relationships")
		return
	}

	for i, status := range statuses {
		switch status.Code {
		case codes.OK:
			log.Info("Created federation relationship", federationRelationshipFields(federationRelationships[i])...)
		default:
			log.Error(status.Err(), "Failed to create federation relationship", federationRelationshipFields(federationRelationships[i])...)
		}
	}
}

func (r *federationRelationshipReconciler) updateFederationRelationships(ctx context.Context, federationRelationships []spireapi.FederationRelationship) {
	log := log.FromContext(ctx)

	statuses, err := r.trustDomainClient.UpdateFederationRelationships(ctx, federationRelationships)
	if err != nil {
		log.Error(err, "Failed to update federation relationships")
		return
	}

	for i, status := range statuses {
		switch status.Code {
		case codes.OK:
			log.Info("Updated federation relationship", federationRelationshipFields(federationRelationships[i])...)
		default:
			log.Error(status.Err(), "Failed to update federation relationship", federationRelationshipFields(federationRelationships[i])...)
		}
	}
}

func (r *federationRelationshipReconciler) deleteFederationRelationships(ctx context.Context, federationRelationships []spireapi.FederationRelationship) {
	log := log.FromContext(ctx)

	statuses, err := r.trustDomainClient.DeleteFederationRelationships(ctx, trustDomainIDsFromFederationRelationships(federationRelationships))
	if err != nil {
		log.Error(err, "Failed to delete federation relationships")
		return
	}

	for i, status := range statuses {
		switch status.Code {
		case codes.OK:
			log.Info("Deleted federation relationship", federationRelationshipFields(federationRelationships[i])...)
		default:
			log.Error(status.Err(), "Failed to delete federation relationship", federationRelationshipFields(federationRelationships[i])...)
		}
	}
}

func trustDomainIDsFromFederationRelationships(frs []spireapi.FederationRelationship) []spiffeid.TrustDomain {
	out := make([]spiffeid.TrustDomain, 0, len(frs))
	for _, fr := range frs {
		out = append(out, fr.TrustDomain)
	}
	return out
}

type clusterFederatedTrustDomainState struct {
	ClusterFederatedTrustDomain spirev1alpha1.ClusterFederatedTrustDomain
	FederationRelationship      spireapi.FederationRelationship
	NextStatus                  spirev1alpha1.ClusterFederatedTrustDomainStatus
}

func sortClusterFederatedTrustDomainsByCreationDate(cftds []spirev1alpha1.ClusterFederatedTrustDomain) {
	sort.Slice(cftds, func(a, b int) bool {
		if cftds[a].CreationTimestamp.Time.Before(cftds[b].CreationTimestamp.Time) {
			return true
		}
		if cftds[a].CreationTimestamp.Time.After(cftds[b].CreationTimestamp.Time) {
			return false
		}
		// Creation timestamps, however unlikely, are equal. Let's tie-break
		// using the UID.
		return cftds[a].UID < cftds[b].UID
	})
}
