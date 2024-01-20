//go:build !ignore_autogenerated

/*
Copyright 2023 SPIRE Authors.

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	configv1alpha1 "k8s.io/component-base/config/v1alpha1"
	timex "time"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BundleEndpointProfile) DeepCopyInto(out *BundleEndpointProfile) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BundleEndpointProfile.
func (in *BundleEndpointProfile) DeepCopy() *BundleEndpointProfile {
	if in == nil {
		return nil
	}
	out := new(BundleEndpointProfile)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterFederatedTrustDomain) DeepCopyInto(out *ClusterFederatedTrustDomain) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterFederatedTrustDomain.
func (in *ClusterFederatedTrustDomain) DeepCopy() *ClusterFederatedTrustDomain {
	if in == nil {
		return nil
	}
	out := new(ClusterFederatedTrustDomain)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterFederatedTrustDomain) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterFederatedTrustDomainList) DeepCopyInto(out *ClusterFederatedTrustDomainList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterFederatedTrustDomain, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterFederatedTrustDomainList.
func (in *ClusterFederatedTrustDomainList) DeepCopy() *ClusterFederatedTrustDomainList {
	if in == nil {
		return nil
	}
	out := new(ClusterFederatedTrustDomainList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterFederatedTrustDomainList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterFederatedTrustDomainSpec) DeepCopyInto(out *ClusterFederatedTrustDomainSpec) {
	*out = *in
	out.BundleEndpointProfile = in.BundleEndpointProfile
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterFederatedTrustDomainSpec.
func (in *ClusterFederatedTrustDomainSpec) DeepCopy() *ClusterFederatedTrustDomainSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterFederatedTrustDomainSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterFederatedTrustDomainStatus) DeepCopyInto(out *ClusterFederatedTrustDomainStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterFederatedTrustDomainStatus.
func (in *ClusterFederatedTrustDomainStatus) DeepCopy() *ClusterFederatedTrustDomainStatus {
	if in == nil {
		return nil
	}
	out := new(ClusterFederatedTrustDomainStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterSPIFFEID) DeepCopyInto(out *ClusterSPIFFEID) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterSPIFFEID.
func (in *ClusterSPIFFEID) DeepCopy() *ClusterSPIFFEID {
	if in == nil {
		return nil
	}
	out := new(ClusterSPIFFEID)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterSPIFFEID) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterSPIFFEIDList) DeepCopyInto(out *ClusterSPIFFEIDList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterSPIFFEID, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterSPIFFEIDList.
func (in *ClusterSPIFFEIDList) DeepCopy() *ClusterSPIFFEIDList {
	if in == nil {
		return nil
	}
	out := new(ClusterSPIFFEIDList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterSPIFFEIDList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterSPIFFEIDSpec) DeepCopyInto(out *ClusterSPIFFEIDSpec) {
	*out = *in
	out.TTL = in.TTL
	out.JWTTTL = in.JWTTTL
	if in.DNSNameTemplates != nil {
		in, out := &in.DNSNameTemplates, &out.DNSNameTemplates
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.WorkloadSelectorTemplates != nil {
		in, out := &in.WorkloadSelectorTemplates, &out.WorkloadSelectorTemplates
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.FederatesWith != nil {
		in, out := &in.FederatesWith, &out.FederatesWith
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NamespaceSelector != nil {
		in, out := &in.NamespaceSelector, &out.NamespaceSelector
		*out = new(v1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.PodSelector != nil {
		in, out := &in.PodSelector, &out.PodSelector
		*out = new(v1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterSPIFFEIDSpec.
func (in *ClusterSPIFFEIDSpec) DeepCopy() *ClusterSPIFFEIDSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterSPIFFEIDSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterSPIFFEIDStats) DeepCopyInto(out *ClusterSPIFFEIDStats) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterSPIFFEIDStats.
func (in *ClusterSPIFFEIDStats) DeepCopy() *ClusterSPIFFEIDStats {
	if in == nil {
		return nil
	}
	out := new(ClusterSPIFFEIDStats)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterSPIFFEIDStatus) DeepCopyInto(out *ClusterSPIFFEIDStatus) {
	*out = *in
	out.Stats = in.Stats
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterSPIFFEIDStatus.
func (in *ClusterSPIFFEIDStatus) DeepCopy() *ClusterSPIFFEIDStatus {
	if in == nil {
		return nil
	}
	out := new(ClusterSPIFFEIDStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterStaticEntry) DeepCopyInto(out *ClusterStaticEntry) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterStaticEntry.
func (in *ClusterStaticEntry) DeepCopy() *ClusterStaticEntry {
	if in == nil {
		return nil
	}
	out := new(ClusterStaticEntry)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterStaticEntry) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterStaticEntryList) DeepCopyInto(out *ClusterStaticEntryList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterStaticEntry, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterStaticEntryList.
func (in *ClusterStaticEntryList) DeepCopy() *ClusterStaticEntryList {
	if in == nil {
		return nil
	}
	out := new(ClusterStaticEntryList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterStaticEntryList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterStaticEntrySpec) DeepCopyInto(out *ClusterStaticEntrySpec) {
	*out = *in
	if in.Selectors != nil {
		in, out := &in.Selectors, &out.Selectors
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.FederatesWith != nil {
		in, out := &in.FederatesWith, &out.FederatesWith
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	out.X509SVIDTTL = in.X509SVIDTTL
	out.JWTSVIDTTL = in.JWTSVIDTTL
	if in.DNSNames != nil {
		in, out := &in.DNSNames, &out.DNSNames
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterStaticEntrySpec.
func (in *ClusterStaticEntrySpec) DeepCopy() *ClusterStaticEntrySpec {
	if in == nil {
		return nil
	}
	out := new(ClusterStaticEntrySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterStaticEntryStatus) DeepCopyInto(out *ClusterStaticEntryStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterStaticEntryStatus.
func (in *ClusterStaticEntryStatus) DeepCopy() *ClusterStaticEntryStatus {
	if in == nil {
		return nil
	}
	out := new(ClusterStaticEntryStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControllerConfigurationSpec) DeepCopyInto(out *ControllerConfigurationSpec) {
	*out = *in
	if in.GroupKindConcurrency != nil {
		in, out := &in.GroupKindConcurrency, &out.GroupKindConcurrency
		*out = make(map[string]int, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.CacheSyncTimeout != nil {
		in, out := &in.CacheSyncTimeout, &out.CacheSyncTimeout
		*out = new(timex.Duration)
		**out = **in
	}
	if in.RecoverPanic != nil {
		in, out := &in.RecoverPanic, &out.RecoverPanic
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControllerConfigurationSpec.
func (in *ControllerConfigurationSpec) DeepCopy() *ControllerConfigurationSpec {
	if in == nil {
		return nil
	}
	out := new(ControllerConfigurationSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControllerHealth) DeepCopyInto(out *ControllerHealth) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControllerHealth.
func (in *ControllerHealth) DeepCopy() *ControllerHealth {
	if in == nil {
		return nil
	}
	out := new(ControllerHealth)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControllerManagerConfig) DeepCopyInto(out *ControllerManagerConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ControllerManagerConfigurationSpec.DeepCopyInto(&out.ControllerManagerConfigurationSpec)
	if in.IgnoreNamespaces != nil {
		in, out := &in.IgnoreNamespaces, &out.IgnoreNamespaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControllerManagerConfig.
func (in *ControllerManagerConfig) DeepCopy() *ControllerManagerConfig {
	if in == nil {
		return nil
	}
	out := new(ControllerManagerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ControllerManagerConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControllerManagerConfigurationSpec) DeepCopyInto(out *ControllerManagerConfigurationSpec) {
	*out = *in
	if in.SyncPeriod != nil {
		in, out := &in.SyncPeriod, &out.SyncPeriod
		*out = new(v1.Duration)
		**out = **in
	}
	if in.LeaderElection != nil {
		in, out := &in.LeaderElection, &out.LeaderElection
		*out = new(configv1alpha1.LeaderElectionConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.CacheNamespaces != nil {
		in, out := &in.CacheNamespaces, &out.CacheNamespaces
		*out = make(map[string]*NamespaceConfig, len(*in))
		for key, val := range *in {
			var outVal *NamespaceConfig
			if val == nil {
				(*out)[key] = nil
			} else {
				inVal := (*in)[key]
				in, out := &inVal, &outVal
				*out = new(NamespaceConfig)
				(*in).DeepCopyInto(*out)
			}
			(*out)[key] = outVal
		}
	}
	if in.GracefulShutdownTimeout != nil {
		in, out := &in.GracefulShutdownTimeout, &out.GracefulShutdownTimeout
		*out = new(v1.Duration)
		**out = **in
	}
	if in.Controller != nil {
		in, out := &in.Controller, &out.Controller
		*out = new(ControllerConfigurationSpec)
		(*in).DeepCopyInto(*out)
	}
	out.Metrics = in.Metrics
	out.Health = in.Health
	in.Webhook.DeepCopyInto(&out.Webhook)
	if in.SyncTypes != nil {
		in, out := &in.SyncTypes, &out.SyncTypes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControllerManagerConfigurationSpec.
func (in *ControllerManagerConfigurationSpec) DeepCopy() *ControllerManagerConfigurationSpec {
	if in == nil {
		return nil
	}
	out := new(ControllerManagerConfigurationSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControllerMetrics) DeepCopyInto(out *ControllerMetrics) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControllerMetrics.
func (in *ControllerMetrics) DeepCopy() *ControllerMetrics {
	if in == nil {
		return nil
	}
	out := new(ControllerMetrics)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControllerWebhook) DeepCopyInto(out *ControllerWebhook) {
	*out = *in
	if in.Port != nil {
		in, out := &in.Port, &out.Port
		*out = new(int)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControllerWebhook.
func (in *ControllerWebhook) DeepCopy() *ControllerWebhook {
	if in == nil {
		return nil
	}
	out := new(ControllerWebhook)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespaceConfig) DeepCopyInto(out *NamespaceConfig) {
	*out = *in
	if in.LabelSelectors != nil {
		in, out := &in.LabelSelectors, &out.LabelSelectors
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.FieldSelectors != nil {
		in, out := &in.FieldSelectors, &out.FieldSelectors
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespaceConfig.
func (in *NamespaceConfig) DeepCopy() *NamespaceConfig {
	if in == nil {
		return nil
	}
	out := new(NamespaceConfig)
	in.DeepCopyInto(out)
	return out
}
