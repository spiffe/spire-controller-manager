package v1alpha1

import (
	"errors"
	"fmt"
	"os"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

func LoadOptionsFromFile(path string, scheme *runtime.Scheme, options *ctrl.Options, config *ControllerManagerConfig, expandEnv bool) error {
	if err := loadFile(path, scheme, config, expandEnv); err != nil {
		return err
	}

	return addOptionsFromConfigSpec(options, config.ControllerManagerConfigurationSpec)
}

func loadFile(path string, scheme *runtime.Scheme, config *ControllerManagerConfig, expandEnv bool) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not read file at %s: %w", path, err)
	}

	if expandEnv {
		content = []byte(os.ExpandEnv(string(content)))
	}

	codecs := serializer.NewCodecFactory(scheme)

	// Regardless of if the bytes are of any external version,
	// it will be read successfully and converted into the internal version
	if err = runtime.DecodeInto(codecs.UniversalDecoder(), content, config); err != nil {
		return fmt.Errorf("could not decode file into runtime.Object: %w", err)
	}

	return nil
}

func addOptionsFromConfigSpec(o *ctrl.Options, configSpec ControllerManagerConfigurationSpec) error {
	setLeaderElectionConfig(o, configSpec)

	if o.Cache.SyncPeriod == nil && configSpec.SyncPeriod != nil {
		o.Cache.SyncPeriod = &configSpec.SyncPeriod.Duration
	}

	if len(o.Cache.DefaultNamespaces) == 0 {
		switch {
		case configSpec.CacheNamespace != "" && len(configSpec.CacheNamespaces) > 0:
			return errors.New("cacheNamespace or cacheNamespaces can be used, but not both")
		case configSpec.CacheNamespace != "":
			o.Cache.DefaultNamespaces = map[string]cache.Config{
				configSpec.CacheNamespace: {},
			}

		case len(configSpec.CacheNamespaces) > 0:
			o.Cache.DefaultNamespaces = make(map[string]cache.Config, len(configSpec.CacheNamespaces))
			for namespace, opts := range configSpec.CacheNamespaces {
				cacheConfig := cache.Config{}
				if opts != nil {
					if len(opts.LabelSelectors) > 0 {
						cacheConfig.LabelSelector = labels.SelectorFromSet(opts.LabelSelectors)
					}
					if len(opts.FieldSelectors) > 0 {
						cacheConfig.FieldSelector = fields.SelectorFromSet(opts.FieldSelectors)
					}
				}

				o.Cache.DefaultNamespaces[namespace] = cacheConfig
			}
		}
	}

	if o.Metrics.BindAddress == "" && configSpec.Metrics.BindAddress != "" {
		o.Metrics.BindAddress = configSpec.Metrics.BindAddress
	}

	if o.HealthProbeBindAddress == "" && configSpec.Health.HealthProbeBindAddress != "" {
		o.HealthProbeBindAddress = configSpec.Health.HealthProbeBindAddress
	}

	if o.ReadinessEndpointName == "" && configSpec.Health.ReadinessEndpointName != "" {
		o.ReadinessEndpointName = configSpec.Health.ReadinessEndpointName
	}

	if o.LivenessEndpointName == "" && configSpec.Health.LivenessEndpointName != "" {
		o.LivenessEndpointName = configSpec.Health.LivenessEndpointName
	}

	if configSpec.Controller != nil {
		if o.Controller.CacheSyncTimeout == 0 && configSpec.Controller.CacheSyncTimeout != nil {
			o.Controller.CacheSyncTimeout = *configSpec.Controller.CacheSyncTimeout
		}

		if len(o.Controller.GroupKindConcurrency) == 0 && len(configSpec.Controller.GroupKindConcurrency) > 0 {
			o.Controller.GroupKindConcurrency = configSpec.Controller.GroupKindConcurrency
		}
	}

	return nil
}

func setLeaderElectionConfig(o *ctrl.Options, obj ControllerManagerConfigurationSpec) {
	if obj.LeaderElection == nil {
		// The source does not have any configuration; noop
		return
	}

	if !o.LeaderElection && obj.LeaderElection.LeaderElect != nil {
		o.LeaderElection = *obj.LeaderElection.LeaderElect
	}

	if o.LeaderElectionResourceLock == "" && obj.LeaderElection.ResourceLock != "" {
		o.LeaderElectionResourceLock = obj.LeaderElection.ResourceLock
	}

	if o.LeaderElectionNamespace == "" && obj.LeaderElection.ResourceNamespace != "" {
		o.LeaderElectionNamespace = obj.LeaderElection.ResourceNamespace
	}

	if o.LeaderElectionID == "" && obj.LeaderElection.ResourceName != "" {
		o.LeaderElectionID = obj.LeaderElection.ResourceName
	}

	if o.LeaseDuration == nil && !reflect.DeepEqual(obj.LeaderElection.LeaseDuration, metav1.Duration{}) {
		o.LeaseDuration = &obj.LeaderElection.LeaseDuration.Duration
	}

	if o.RenewDeadline == nil && !reflect.DeepEqual(obj.LeaderElection.RenewDeadline, metav1.Duration{}) {
		o.RenewDeadline = &obj.LeaderElection.RenewDeadline.Duration
	}

	if o.RetryPeriod == nil && !reflect.DeepEqual(obj.LeaderElection.RetryPeriod, metav1.Duration{}) {
		o.RetryPeriod = &obj.LeaderElection.RetryPeriod.Duration
	}
}
