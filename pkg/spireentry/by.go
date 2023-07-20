package spireentry

import (
	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

type byObject interface {
	GetObjectKind() schema.ObjectKind

	GetUID() types.UID
	GetCreationTimestamp() metav1.Time
	GetDeletionTimestamp() *metav1.Time

	IncrementEntriesToSet()
	IncrementEntriesMasked()
	IncrementEntrySuccess()
	IncrementEntryFailures()
}

type ClusterStaticEntry struct {
	spirev1alpha1.ClusterStaticEntry
	NextStatus spirev1alpha1.ClusterStaticEntryStatus
}

func (by *ClusterStaticEntry) IncrementEntriesToSet() {
}

func (by *ClusterStaticEntry) IncrementEntriesMasked() {
	by.NextStatus.Masked = true
}

func (by *ClusterStaticEntry) IncrementEntrySuccess() {
	by.NextStatus.Set = true
}

func (by *ClusterStaticEntry) IncrementEntryFailures() {
}

type ClusterSPIFFEID struct {
	spirev1alpha1.ClusterSPIFFEID
	NextStatus spirev1alpha1.ClusterSPIFFEIDStatus
}

func (by *ClusterSPIFFEID) IncrementEntriesToSet() {
	by.NextStatus.Stats.EntriesToSet++
}

func (by *ClusterSPIFFEID) IncrementEntriesMasked() {
	by.NextStatus.Stats.EntriesMasked++
}

func (by *ClusterSPIFFEID) IncrementEntrySuccess() {
}

func (by *ClusterSPIFFEID) IncrementEntryFailures() {
	by.NextStatus.Stats.EntryFailures++
}
