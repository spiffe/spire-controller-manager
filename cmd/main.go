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

package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	k8sMetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/spiffe/spire-controller-manager/internal/controller"
	"github.com/spiffe/spire-controller-manager/pkg/metrics"
	"github.com/spiffe/spire-controller-manager/pkg/reconciler"
	"github.com/spiffe/spire-controller-manager/pkg/spireapi"
	"github.com/spiffe/spire-controller-manager/pkg/spireentry"
	"github.com/spiffe/spire-controller-manager/pkg/spirefederationrelationship"
	"github.com/spiffe/spire-controller-manager/pkg/webhookmanager"
	//+kubebuilder:scaffold:imports
)

type Config struct {
	ctrlConfig            spirev1alpha1.ControllerManagerConfig
	options               ctrl.Options
	ignoreNamespacesRegex []*regexp.Regexp
	parentIDTemplate      *template.Template
	reconcile             spirev1alpha1.ReconcileConfig
}

const (
	defaultSPIREServerSocketPath = "/spire-server/api.sock"
	defaultGCInterval            = 10 * time.Second
	k8sDefaultService            = "kubernetes.default.svc"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(spirev1alpha1.AddToScheme(scheme))

	k8sMetrics.Registry.MustRegister(
		metrics.PromCounters[metrics.StaticEntryFailures],
	)
	//+kubebuilder:scaffold:scheme
}

func main() {
	mainConfig, err := parseConfig()
	if err != nil {
		setupLog.Error(err, "error parsing configuration")
		os.Exit(1)
	}

	if err := run(mainConfig); err != nil {
		os.Exit(1)
	}
}

func addDotSuffix(val string) string {
	if val != "" && !strings.HasSuffix(val, ".") {
		val += "."
	}
	return val
}

func parseConfig() (Config, error) {
	var retval Config
	var configFileFlag string
	var spireAPISocketFlag string
	var expandEnvFlag bool
	flag.StringVar(&configFileFlag, "config", "",
		"The controller will load its initial configuration from this file. "+
			"Omit this flag to use the default configuration values. "+
			"Command-line flags override configuration from this file.")
	flag.StringVar(&spireAPISocketFlag, "spire-api-socket", "", "The path to the SPIRE API socket (deprecated; use the config file)")
	flag.BoolVar(&expandEnvFlag, "expand-env", false, "Expand environment variables in SPIRE Controller Manager config file")

	// Parse log flags
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Set default values
	retval.ctrlConfig = spirev1alpha1.ControllerManagerConfig{
		IgnoreNamespaces:                   []string{"kube-system", "kube-public", "spire-system"},
		GCInterval:                         defaultGCInterval,
		ValidatingWebhookConfigurationName: "spire-controller-manager-webhook",
	}

	retval.options = ctrl.Options{Scheme: scheme}

	if configFileFlag != "" {
		if err := spirev1alpha1.LoadOptionsFromFile(configFileFlag, scheme, &retval.options, &retval.ctrlConfig, expandEnvFlag); err != nil {
			return retval, fmt.Errorf("unable to load the config file: %w", err)
		}

		for _, ignoredNamespace := range retval.ctrlConfig.IgnoreNamespaces {
			regex, err := regexp.Compile(ignoredNamespace)
			if err != nil {
				return retval, fmt.Errorf("unable to compile ignore namespaces regex: %w", err)
			}

			retval.ignoreNamespacesRegex = append(retval.ignoreNamespacesRegex, regex)
		}
	}
	// Determine the SPIRE Server socket path
	switch {
	case retval.ctrlConfig.SPIREServerSocketPath == "" && spireAPISocketFlag == "":
		// Neither is set. Use the default.
		retval.ctrlConfig.SPIREServerSocketPath = defaultSPIREServerSocketPath
	case retval.ctrlConfig.SPIREServerSocketPath != "" && spireAPISocketFlag == "":
		// Configuration file value is set. Use it.
	case retval.ctrlConfig.SPIREServerSocketPath == "" && spireAPISocketFlag != "":
		// Deprecated flag value is set. Use it but warn.
		retval.ctrlConfig.SPIREServerSocketPath = spireAPISocketFlag
		setupLog.Error(nil, "The spire-api-socket flag is deprecated and will be removed in a future release; use the configuration file instead")
	case retval.ctrlConfig.SPIREServerSocketPath != "" && spireAPISocketFlag != "":
		// Both are set. Warn and ignore the deprecated flag.
		setupLog.Error(nil, "Ignoring deprecated spire-api-socket flag which will be removed in a future release")
	}

	// Attempt to auto detect cluster domain if it wasn't specified
	if retval.ctrlConfig.ClusterDomain == "" {
		clusterDomain, err := autoDetectClusterDomain()
		if err != nil {
			setupLog.Error(err, "unable to autodetect cluster domain")
		}

		retval.ctrlConfig.ClusterDomain = clusterDomain
	}

	if retval.ctrlConfig.ParentIDTemplate != "" {
		var err error
		retval.parentIDTemplate, err = template.New("customParentIDTemplate").Parse(retval.ctrlConfig.ParentIDTemplate)
		if err != nil {
			return retval, fmt.Errorf("unable to parse parent ID template: %w", err)
		}
	}

	if retval.ctrlConfig.Reconcile == nil {
		retval.reconcile.ClusterSPIFFEIDs = true
		retval.reconcile.ClusterFederatedTrustDomains = true
		retval.reconcile.ClusterStaticEntries = true
	} else {
		retval.reconcile = *retval.ctrlConfig.Reconcile
	}

	retval.ctrlConfig.EntryIDPrefix = addDotSuffix(retval.ctrlConfig.EntryIDPrefix)

	printCleanup := "<unset>"
	if retval.ctrlConfig.EntryIDPrefixCleanup != nil {
		printCleanup = *retval.ctrlConfig.EntryIDPrefixCleanup
		*retval.ctrlConfig.EntryIDPrefixCleanup = addDotSuffix(*retval.ctrlConfig.EntryIDPrefixCleanup)
		if retval.ctrlConfig.EntryIDPrefix != "" && retval.ctrlConfig.EntryIDPrefix == *retval.ctrlConfig.EntryIDPrefixCleanup {
			return retval, fmt.Errorf("if entryIDPrefixCleanup is specified, it can not be the same value as entryIDPrefix")
		}
	}

	setupLog.Info("Config loaded",
		"cluster name", retval.ctrlConfig.ClusterName,
		"cluster domain", retval.ctrlConfig.ClusterDomain,
		"trust domain", retval.ctrlConfig.TrustDomain,
		"ignore namespaces", retval.ctrlConfig.IgnoreNamespaces,
		"gc interval", retval.ctrlConfig.GCInterval,
		"spire server socket path", retval.ctrlConfig.SPIREServerSocketPath,
		"class name", retval.ctrlConfig.ClassName,
		"handle crs without class name", retval.ctrlConfig.WatchClassless,
		"reconcile ClusterSPIFFEIDs", retval.reconcile.ClusterSPIFFEIDs,
		"reconcile ClusterFederatedTrustDomains", retval.reconcile.ClusterFederatedTrustDomains,
		"reconcile ClusterStaticEntries", retval.reconcile.ClusterStaticEntries,
		"entryIDPrefix", retval.ctrlConfig.EntryIDPrefix,
		"entryIDPrefixCleanup", printCleanup)

	switch {
	case retval.ctrlConfig.TrustDomain == "":
		setupLog.Error(nil, "trust domain is required configuration")
		return retval, errors.New("trust domain is required configuration")
	case retval.ctrlConfig.ClusterName == "":
		return retval, errors.New("cluster name is required configuration")
	case retval.ctrlConfig.ValidatingWebhookConfigurationName == "":
		return retval, errors.New("validating webhook configuration name is required configuration")
	case retval.ctrlConfig.ControllerManagerConfigurationSpec.Webhook.CertDir != "":
		setupLog.Info("certDir configuration is ignored", "certDir", retval.ctrlConfig.ControllerManagerConfigurationSpec.Webhook.CertDir)
	}

	return retval, nil
}

func run(mainConfig Config) (err error) {
	webhookEnabled := os.Getenv("ENABLE_WEBHOOKS") != "false"

	trustDomain, err := spiffeid.TrustDomainFromString(mainConfig.ctrlConfig.TrustDomain)
	if err != nil {
		setupLog.Error(err, "invalid trust domain name")
		return err
	}

	ctx := ctrl.SetupSignalHandler()

	setupLog.Info("Dialing SPIRE Server socket")
	spireClient, err := spireapi.DialSocket(mainConfig.ctrlConfig.SPIREServerSocketPath)
	if err != nil {
		setupLog.Error(err, "unable to dial SPIRE Server socket")
		return err
	}
	defer spireClient.Close()

	// It's unfortunate that we have to keep credentials on disk so that the
	// manager can load them. Webhook server credentials are stored in a single
	// file to keep rotation simple.
	// TODO: upstream a change to the WebhookServer so it can use callbacks to
	// obtain the certificates so we don't have to touch disk.
	var webhookRunnable manager.Runnable
	if webhookEnabled {
		const keyPairName = "keypair.pem"
		certDir, err := os.MkdirTemp("", "spire-controller-manager-")
		if err != nil {
			setupLog.Error(err, "failed to create temporary cert directory")
			return err
		}
		defer func() {
			if err := os.RemoveAll(certDir); err != nil {
				setupLog.Error(err, "failed to remove temporary cert directory", "certDir", certDir)
				os.Exit(1)
			}
		}()
		mainConfig.options.WebhookServer = webhook.NewServer(webhook.Options{
			CertDir:  certDir,
			CertName: keyPairName,
			KeyName:  keyPairName,
			TLSOpts: []func(*tls.Config){
				func(s *tls.Config) {
					s.MinVersion = tls.VersionTLS12
				},
			},
		})
		// We need a direct client to query and patch up the webhook. We can't use
		// the controller runtime client for this because we can't start the manager
		// without the webhook credentials being in place, and the webhook credentials
		// need the DNS name of the webhook service from the configuration.
		config, err := rest.InClusterConfig()
		if err != nil {
			setupLog.Error(err, "failed to get in cluster configuration")
			return err
		}
		// creates the clientset
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			setupLog.Error(err, "failed to create an API client")
			return err
		}

		webhookManager := webhookmanager.New(webhookmanager.Config{
			ID:            spiffeid.RequireFromPath(trustDomain, "/spire-controller-manager-webhook"),
			KeyPairPath:   filepath.Join(certDir, keyPairName),
			WebhookName:   mainConfig.ctrlConfig.ValidatingWebhookConfigurationName,
			WebhookClient: clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
			SVIDClient:    spireClient,
			BundleClient:  spireClient,
		})

		if err := webhookManager.Init(ctx); err != nil {
			setupLog.Error(err, "failed to mint initial webhook certificate")
			return err
		}

		webhookRunnable = webhookManager
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mainConfig.options)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return err
	}

	var entryReconciler reconciler.Reconciler
	if mainConfig.reconcile.ClusterSPIFFEIDs || mainConfig.reconcile.ClusterStaticEntries {
		entryReconciler = spireentry.Reconciler(spireentry.ReconcilerConfig{
			TrustDomain:          trustDomain,
			ClusterName:          mainConfig.ctrlConfig.ClusterName,
			ClusterDomain:        mainConfig.ctrlConfig.ClusterDomain,
			K8sClient:            mgr.GetClient(),
			EntryClient:          spireClient,
			IgnoreNamespaces:     mainConfig.ignoreNamespacesRegex,
			GCInterval:           mainConfig.ctrlConfig.GCInterval,
			ClassName:            mainConfig.ctrlConfig.ClassName,
			WatchClassless:       mainConfig.ctrlConfig.WatchClassless,
			ParentIDTemplate:     mainConfig.parentIDTemplate,
			Reconcile:            mainConfig.reconcile,
			EntryIDPrefix:        mainConfig.ctrlConfig.EntryIDPrefix,
			EntryIDPrefixCleanup: mainConfig.ctrlConfig.EntryIDPrefixCleanup,
		})
	}

	var federationRelationshipReconciler reconciler.Reconciler
	if mainConfig.reconcile.ClusterFederatedTrustDomains {
		federationRelationshipReconciler = spirefederationrelationship.Reconciler(spirefederationrelationship.ReconcilerConfig{
			K8sClient:         mgr.GetClient(),
			TrustDomainClient: spireClient,
			GCInterval:        mainConfig.ctrlConfig.GCInterval,
			ClassName:         mainConfig.ctrlConfig.ClassName,
			WatchClassless:    mainConfig.ctrlConfig.WatchClassless,
		})
		if err = (&controller.ClusterFederatedTrustDomainReconciler{
			Client:    mgr.GetClient(),
			Scheme:    mgr.GetScheme(),
			Triggerer: federationRelationshipReconciler,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "ClusterFederatedTrustDomain")
			return err
		}
	}

	if mainConfig.reconcile.ClusterSPIFFEIDs {
		if err = (&controller.ClusterSPIFFEIDReconciler{
			Client:    mgr.GetClient(),
			Scheme:    mgr.GetScheme(),
			Triggerer: entryReconciler,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "ClusterSPIFFEID")
			return err
		}
	}
	if mainConfig.reconcile.ClusterStaticEntries {
		if err = (&controller.ClusterStaticEntryReconciler{
			Client:    mgr.GetClient(),
			Scheme:    mgr.GetScheme(),
			Triggerer: entryReconciler,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "ClusterStaticEntry")
			return err
		}
	}
	if webhookEnabled {
		if err = (&spirev1alpha1.ClusterFederatedTrustDomain{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "ClusterFederatedTrustDomain")
			return err
		}
		if err = (&spirev1alpha1.ClusterSPIFFEID{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "ClusterSPIFFEID")
			return err
		}
	}
	//+kubebuilder:scaffold:builder

	if mainConfig.reconcile.ClusterSPIFFEIDs {
		if err = (&controller.PodReconciler{
			Client:           mgr.GetClient(),
			Scheme:           mgr.GetScheme(),
			Triggerer:        entryReconciler,
			IgnoreNamespaces: mainConfig.ignoreNamespacesRegex,
		}).SetupWithManager(ctx, mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Pod")
			return err
		}
		if err = (&controller.EndpointsReconciler{
			Client:           mgr.GetClient(),
			Scheme:           mgr.GetScheme(),
			Triggerer:        entryReconciler,
			IgnoreNamespaces: mainConfig.ignoreNamespacesRegex,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Endpoints")
			return err
		}
	}

	if entryReconciler != nil {
		if err = mgr.Add(manager.RunnableFunc(entryReconciler.Run)); err != nil {
			setupLog.Error(err, "unable to manage entry reconciler")
			return err
		}
	}

	if federationRelationshipReconciler != nil {
		if err = mgr.Add(manager.RunnableFunc(federationRelationshipReconciler.Run)); err != nil {
			setupLog.Error(err, "unable to manage federation relationship reconciler")
			return err
		}
	}

	if webhookRunnable != nil {
		if err = mgr.Add(webhookRunnable); err != nil {
			setupLog.Error(err, "unable to manage federation relationship reconciler")
			return err
		}
	}
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		return err
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}

	return nil
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
