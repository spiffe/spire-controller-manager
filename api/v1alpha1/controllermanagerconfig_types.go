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

package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	configv1alpha1 "k8s.io/component-base/config/v1alpha1"
)

//+kubebuilder:object:root=true

// ControllerManagerConfig is the Schema for the controller manager configuration
type ControllerManagerConfig struct {
	metav1.TypeMeta `json:",inline"`

	// ControllerManagerConfigurationSpec returns the contfigurations for controllers
	ControllerManagerConfigurationSpec `json:",inline"`

	// ClusterName is the cluster name
	ClusterName string `json:"clusterName"`

	// ClusterDomain is the cluster domain, ie cluster.local
	ClusterDomain string `json:"clusterDomain"`

	// TrustDomain is the name of the SPIFFE trust domain
	TrustDomain string `json:"trustDomain"`

	// IgnoreNamespaces are the namespaces to ignore
	IgnoreNamespaces []string `json:"ignoreNamespaces"`

	// ValidatingWebhookConfigurationName selects the webhook configuration to manage.
	// Defaults to spire-controller-manager-webhook.
	ValidatingWebhookConfigurationName string `json:"validatingWebhookConfigurationName"`

	// GCInterval is how often SPIRE state is reconciled when the controller
	// is otherwise idle. This impacts how quickly SPIRE state will converge
	// after CRDs are removed or SPIRE state is mutated out from underneath
	// the controller.
	GCInterval time.Duration `json:"gcInterval"`

	// SPIREServerSocketPath is the path to the SPIRE Server API socket
	SPIREServerSocketPath string `json:"spireServerSocketPath"`

	// LogLevel is the log level for the controller manager
	LogLevel string `json:"logLevel"`
}

// ControllerManagerConfigurationSpec defines the desired state of GenericControllerManagerConfiguration.
type ControllerManagerConfigurationSpec struct {
	// SyncPeriod determines the minimum frequency at which watched resources are
	// reconciled. A lower period will correct entropy more quickly, but reduce
	// responsiveness to change if there are many watched resources. Change this
	// value only if you know what you are doing. Defaults to 10 hours if unset.
	// there will a 10 percent jitter between the SyncPeriod of all controllers
	// so that all controllers will not send list requests simultaneously.
	// +optional
	SyncPeriod *metav1.Duration `json:"syncPeriod,omitempty"`

	// LeaderElection is the LeaderElection config to be used when configuring
	// the manager.Manager leader election.
	// +optional
	LeaderElection *configv1alpha1.LeaderElectionConfiguration `json:"leaderElection,omitempty"`

	// CacheNamespace if specified restricts the manager's cache to watch objects in
	// the desired namespace. Defaults to all namespaces.
	// Deprecated: use cacheNamespaces instead
	//
	// Note: If a namespace is specified, controllers can still Watch for a
	// cluster-scoped resource (e.g Node).  For namespaced resources the cache
	// will only hold objects from the desired namespace.
	// +optional
	CacheNamespace string `json:"cacheNamespace,omitempty"`

	// CacheNamespaces if specified restricts the manager's cache to watch objects in
	// the desired namespaces. Defaults to all namespaces.
	// +optional
	CacheNamespaces map[string]*NamespaceConfig `json:"cacheNamespaces,omitempty"`

	// GracefulShutdownTimeout is the duration given to runnable to stop before the manager actually returns on stop.
	// To disable graceful shutdown, set to time.Duration(0)
	// To use graceful shutdown without timeout, set to a negative duration, e.G. time.Duration(-1)
	// The graceful shutdown is skipped for safety reasons in case the leader election lease is lost.
	GracefulShutdownTimeout *metav1.Duration `json:"gracefulShutDown,omitempty"`

	// Controller contains global configuration options for controllers
	// registered within this manager.
	// +optional
	Controller *ControllerConfigurationSpec `json:"controller,omitempty"`

	// Metrics contains the controller metrics configuration
	// +optional
	Metrics ControllerMetrics `json:"metrics,omitempty"`

	// Health contains the controller health configuration
	// +optional
	Health ControllerHealth `json:"health,omitempty"`

	// Webhook contains the controllers webhook configuration
	// +optional
	Webhook ControllerWebhook `json:"webhook,omitempty"`

	// ClassName contains the name of a class to watch CRs for. Others will be ignored.
	// If unset all will be watched.
	// +optional
	ClassName string `json:"className,omitempty"`

	// If WatchClassless is set and ClassName is set, any CR without a ClassName
	// specified will also be handled by this controller.
	// +optional
	WatchClassless bool `json:"watchClassless,omitempty"`

	// If specified, uses a different parent id template for linking pods to nodes
	// +optional
	ParentIDTemplate string `json:"parentIDTemplate,omitempty"`

	// If specified, only syncs the specified CR types. Defaults to all.
	// +optional
	Reconcile *ReconcileConfig `json:"reconcile,omitempty"`

	// If specified, prefixes each entry id with `<prefix>.`. Entries without the Prefix will be ignored (except ones marked for cleanup, see EntryIDPrefixCleanup).
	// +optiional
	EntryIDPrefix string `json:"entryIDPrefix,omitempty"`

	// If specified, entries with the specified prefix will be removed. If set to "" it will clean up all unprefixed entries.
	// It can not be set to the same value as EntryIDPrefix.
	// Generally useful when switching from nonprefixed to prefixed, or between two different prefixes.
	// +optiional
	EntryIDPrefixCleanup *string `json:"entryIDPrefixCleanup,omitempty"`

	// When configured, read yaml objects from the specified path rather then from Kubernetes.
	StaticManifestPath *string `json:"staticManifestPath,omitempty"`
}

// ReconcileConfig configuration used to enable/disable syncing various types
type ReconcileConfig struct {
	// ClusterSpiffeIds enable syncing of clusterspiffeids
	// +optional
	ClusterSPIFFEIDs bool `json:"clusterSPIFFEIDs,omitempty"`

	// ClusterFederatedTrustDomains enable syncing of clusterfederatedtrustdomains
	// +optional
	ClusterFederatedTrustDomains bool `json:"clusterFederatedTrustDomains,omitempty"`

	// ClusterStaticEntries enable syncing of clusterstaticentries
	// +optional
	ClusterStaticEntries bool `json:"clusterStaticEntries,omitempty"`
}

// NamespaceConfig configuration used to filter cached namespaces
type NamespaceConfig struct {
	// LabelSelectors map of Labels selectors
	// +optional
	LabelSelectors map[string]string `json:"labelSelectors,omitempty"`

	// FieldSelectors map of Fields selectors
	// +optional
	FieldSelectors map[string]string `json:"fieldSelectors,omitempty"`
}

// ControllerConfigurationSpec defines the global configuration for
// controllers registered with the manager.
type ControllerConfigurationSpec struct {
	// GroupKindConcurrency is a map from a Kind to the number of concurrent reconciliation
	// allowed for that controller.
	//
	// When a controller is registered within this manager using the builder utilities,
	// users have to specify the type the controller reconciles in the For(...) call.
	// If the object's kind passed matches one of the keys in this map, the concurrency
	// for that controller is set to the number specified.
	//
	// The key is expected to be consistent in form with GroupKind.String(),
	// e.g. ReplicaSet in apps group (regardless of version) would be `ReplicaSet.apps`.
	//
	// +optional
	GroupKindConcurrency map[string]int `json:"groupKindConcurrency,omitempty"`

	// CacheSyncTimeout refers to the time limit set to wait for syncing caches.
	// Defaults to 2 minutes if not set.
	// +optional
	CacheSyncTimeout *time.Duration `json:"cacheSyncTimeout,omitempty"`

	// RecoverPanic indicates if panics should be recovered.
	// +optional
	RecoverPanic *bool `json:"recoverPanic,omitempty"`
}

// ControllerMetrics defines the metrics configs.
type ControllerMetrics struct {
	// BindAddress is the TCP address that the controller should bind to
	// for serving prometheus metrics.
	// It can be set to "0" to disable the metrics serving.
	// +optional
	BindAddress string `json:"bindAddress,omitempty"`
}

// ControllerHealth defines the health configs.
type ControllerHealth struct {
	// HealthProbeBindAddress is the TCP address that the controller should bind to
	// for serving health probes
	// It can be set to "0" or "" to disable serving the health probe.
	// +optional
	HealthProbeBindAddress string `json:"healthProbeBindAddress,omitempty"`

	// ReadinessEndpointName, defaults to "readyz"
	// +optional
	ReadinessEndpointName string `json:"readinessEndpointName,omitempty"`

	// LivenessEndpointName, defaults to "healthz"
	// +optional
	LivenessEndpointName string `json:"livenessEndpointName,omitempty"`
}

// ControllerWebhook defines the webhook server for the controller.
type ControllerWebhook struct {
	// Port is the port that the webhook server serves at.
	// It is used to set webhook.Server.Port.
	// +optional
	Port *int `json:"port,omitempty"`

	// Host is the hostname that the webhook server binds to.
	// It is used to set webhook.Server.Host.
	// +optional
	Host string `json:"host,omitempty"`

	// CertDir is the directory that contains the server key and certificate.
	// if not set, webhook server would look up the server key and certificate in
	// {TempDir}/k8s-webhook-server/serving-certs. The server key and certificate
	// must be named tls.key and tls.crt, respectively.
	// +optional
	CertDir string `json:"certDir,omitempty"`
}

func init() {
	SchemeBuilder.Register(&ControllerManagerConfig{})
}
