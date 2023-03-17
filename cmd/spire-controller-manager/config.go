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

package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"strings"
	"time"

	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	defaultSPIREServerSocketPath = "/spire-server/api.sock"
	defaultGCInterval            = 10 * time.Second
	k8sDefaultService            = "kubernetes.default.svc"
)

func parseConfig() (spirev1alpha1.ControllerManagerConfig, ctrl.Options, error) {
	var configFileFlag string
	var spireAPISocketFlag string
	flag.StringVar(&configFileFlag, "config", "",
		"The controller will load its initial configuration from this file. "+
			"Omit this flag to use the default configuration values. "+
			"Command-line flags override configuration from this file.")
	flag.StringVar(&spireAPISocketFlag, "spire-api-socket", "", "The path to the SPIRE API socket (deprecated; use the config file)")

	// Parse log flags
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Set default values
	ctrlConfig := spirev1alpha1.ControllerManagerConfig{
		IgnoreNamespaces:                   []string{"kube-system", "kube-public", "spire-system"},
		GCInterval:                         defaultGCInterval,
		ValidatingWebhookConfigurationName: "spire-controller-manager-webhook",
	}

	options := ctrl.Options{Scheme: scheme}
	if configFileFlag != "" {
		var err error
		options, err = options.AndFrom(ctrl.ConfigFile().AtPath(configFileFlag).OfKind(&ctrlConfig))
		if err != nil {
			return ctrlConfig, options, fmt.Errorf("unable to load the config file: %w", err)
		}
	}
	// Determine the SPIRE Server socket path
	switch {
	case ctrlConfig.SPIREServerSocketPath == "" && spireAPISocketFlag == "":
		// Neither is set. Use the default.
		ctrlConfig.SPIREServerSocketPath = defaultSPIREServerSocketPath
	case ctrlConfig.SPIREServerSocketPath != "" && spireAPISocketFlag == "":
		// Configuration file value is set. Use it.
	case ctrlConfig.SPIREServerSocketPath == "" && spireAPISocketFlag != "":
		// Deprecated flag value is set. Use it but warn.
		ctrlConfig.SPIREServerSocketPath = spireAPISocketFlag
		setupLog.Error(nil, "The spire-api-socket flag is deprecated and will be removed in a future release; use the configuration file instead")
	case ctrlConfig.SPIREServerSocketPath != "" && spireAPISocketFlag != "":
		// Both are set. Warn and ignore the deprecated flag.
		setupLog.Error(nil, "Ignoring deprecated spire-api-socket flag which will be removed in a future release")
	}

	// Attempt to auto detect cluster domain if it wasn't specified
	if ctrlConfig.ClusterDomain == "" {
		clusterDomain, err := autoDetectClusterDomain()
		if err != nil {
			setupLog.Error(err, "unable to autodetect cluster domain")
		}

		ctrlConfig.ClusterDomain = clusterDomain
	}

	setupLog.Info("Config loaded",
		"cluster name", ctrlConfig.ClusterName,
		"cluster domain", ctrlConfig.ClusterDomain,
		"trust domain", ctrlConfig.TrustDomain,
		"ignore namespaces", ctrlConfig.IgnoreNamespaces,
		"gc interval", ctrlConfig.GCInterval,
		"spire server socket path", ctrlConfig.SPIREServerSocketPath)

	switch {
	case ctrlConfig.TrustDomain == "":
		setupLog.Error(nil, "trust domain is required configuration")
		return ctrlConfig, options, errors.New("trust domain is required configuration")
	case ctrlConfig.ClusterName == "":
		return ctrlConfig, options, errors.New("cluster name is required configuration")
	case ctrlConfig.ValidatingWebhookConfigurationName == "":
		return ctrlConfig, options, errors.New("validating webhook configuration name is required configuration")
	case options.CertDir != "":
		setupLog.Info("certDir configuration is ignored", "certDir", options.CertDir)
	}

	return ctrlConfig, options, nil
}

func autoDetectClusterDomain() (string, error) {
	cname, err := net.LookupCNAME(k8sDefaultService)
	if err != nil {
		return "", fmt.Errorf("unable to lookup CNAME: %w", err)
	}

	clusterDomain, err := parseClusterDomainCNAME(cname)
	if err != nil {
		return "", fmt.Errorf("unable to parse CNAME \"%s\": %w", cname, err)
	}

	return clusterDomain, nil
}

func parseClusterDomainCNAME(cname string) (string, error) {
	clusterDomain := strings.TrimPrefix(cname, k8sDefaultService+".")
	if clusterDomain == cname {
		return "", errors.New("CNAME did not have expected prefix")
	}

	// Trim off optional trailing dot
	clusterDomain = strings.TrimSuffix(clusterDomain, ".")
	if clusterDomain == "" {
		return "", errors.New("CNAME did not have a cluster domain")
	}

	return clusterDomain, nil
}
