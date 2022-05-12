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
	"flag"
	"os"
	"path/filepath"
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
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/spiffe/go-spiffe/v2/spiffeid"

	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/spiffe/spire-controller-manager/controllers"
	"github.com/spiffe/spire-controller-manager/pkg/spireapi"
	"github.com/spiffe/spire-controller-manager/pkg/spireentry"
	"github.com/spiffe/spire-controller-manager/pkg/spirefederationrelationship"
	"github.com/spiffe/spire-controller-manager/pkg/webhookmanager"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(spirev1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var configFile string
	var spireAPISocket string
	flag.StringVar(&configFile, "config", "",
		"The controller will load its initial configuration from this file. "+
			"Omit this flag to use the default configuration values. "+
			"Command-line flags override configuration from this file.")
	flag.StringVar(&spireAPISocket, "spire-api-socket", "/spire-server/api.sock",
		"The path to the SPIRE API socket")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	var err error
	ctrlConfig := spirev1alpha1.ControllerManagerConfig{
		IgnoreNamespaces:                   []string{"kube-system", "kube-public", "spire-system"},
		GCInterval:                         10 * time.Second,
		ValidatingWebhookConfigurationName: "spire-controller-manager-webhook",
	}

	options := ctrl.Options{Scheme: scheme}
	if configFile != "" {
		options, err = options.AndFrom(ctrl.ConfigFile().AtPath(configFile).OfKind(&ctrlConfig))
		if err != nil {
			setupLog.Error(err, "unable to load the config file")
			os.Exit(1)
		}
	}

	setupLog.Info("Config loaded",
		"cluster name", ctrlConfig.ClusterName,
		"trust domain", ctrlConfig.TrustDomain,
		"ignore namespaces", ctrlConfig.IgnoreNamespaces,
		"gc interval", ctrlConfig.GCInterval,
		"spire socket", spireAPISocket)

	switch {
	case ctrlConfig.TrustDomain == "":
		setupLog.Error(err, "trust domain is required configuration")
		os.Exit(1)
	case ctrlConfig.ClusterName == "":
		setupLog.Error(err, "cluster name is required configuration")
		os.Exit(1)
	case ctrlConfig.ValidatingWebhookConfigurationName == "":
		setupLog.Error(err, "validating webhook configuration name is required configuration")
		os.Exit(1)
	case options.CertDir != "":
		setupLog.Info("certDir configuration is ignored", "certDir", options.CertDir)
	}

	// It's unfortunate that we have to keep credentials on disk so that the
	// manager can load them:
	// TODO: upstream a change to the WebhookServer so it can use callbacks to
	// obtain the certificates so we don't have to touch disk.
	certDir, err := os.MkdirTemp("", "spire-controller-manager-")
	if err != nil {
		setupLog.Error(err, "failed to create temporary cert directory", "certDir", options.CertDir)
		os.Exit(1)
	}
	defer func() {
		if err := os.RemoveAll(options.CertDir); err != nil {
			setupLog.Error(err, "failed to remove temporary cert directory", "certDir", options.CertDir)
			os.Exit(1)
		}
	}()

	// webhook server credentials are stored in a single file to keep rotation
	// simple.
	const keyPairName = "keypair.pem"
	options.WebhookServer = &webhook.Server{
		CertDir:  certDir,
		CertName: keyPairName,
		KeyName:  keyPairName,
	}

	ctx := ctrl.SetupSignalHandler()

	trustDomain, err := spiffeid.TrustDomainFromString(ctrlConfig.TrustDomain)
	if err != nil {
		setupLog.Error(err, "invalid trust domain name")
		os.Exit(1)
	}
	spireClient, err := spireapi.DialSocket(ctx, spireAPISocket)
	if err != nil {
		setupLog.Error(err, "unable to dial SPIRE Server socket")
		os.Exit(1)
	}
	defer spireClient.Close()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// We need a direct client to query and patch up the webhook. We can't use
	// the controller runtime client for this because we can't start the manager
	// without the webhook credentials being in place, and the webhook credentials
	// need the DNS name of the webhook service from the configuration.
	config, err := rest.InClusterConfig()
	if err != nil {
		setupLog.Error(err, "failed to get in cluster configuration")
		os.Exit(1)
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		setupLog.Error(err, "failed to create an API client")
		os.Exit(1)
	}

	webhookID, _ := spiffeid.FromPath(trustDomain, "/spire-controller-manager-webhook")
	webhookManager := webhookmanager.New(webhookmanager.Config{
		ID:            webhookID,
		KeyPairPath:   filepath.Join(certDir, keyPairName),
		WebhookName:   ctrlConfig.ValidatingWebhookConfigurationName,
		WebhookClient: clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
		SVIDClient:    spireClient,
		BundleClient:  spireClient,
	})

	if err := webhookManager.Init(ctx); err != nil {
		setupLog.Error(err, "failed to mint initial webhook certificate")
		os.Exit(1)
	}

	entryReconciler := spireentry.Reconciler(spireentry.ReconcilerConfig{
		TrustDomain:      trustDomain,
		ClusterName:      ctrlConfig.ClusterName,
		K8sClient:        mgr.GetClient(),
		EntryClient:      spireClient,
		IgnoreNamespaces: ctrlConfig.IgnoreNamespaces,
		GCInterval:       ctrlConfig.GCInterval,
	})

	federationRelationshipReconciler := spirefederationrelationship.Reconciler(spirefederationrelationship.ReconcilerConfig{
		K8sClient:         mgr.GetClient(),
		TrustDomainClient: spireClient,
		GCInterval:        ctrlConfig.GCInterval,
	})

	if err = (&controllers.ClusterSPIFFEIDReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		Triggerer: entryReconciler,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterSPIFFEID")
		os.Exit(1)
	}
	if err = (&controllers.ClusterFederatedTrustDomainReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		Triggerer: federationRelationshipReconciler,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterFederatedTrustDomain")
		os.Exit(1)
	}
	if err = (&spirev1alpha1.ClusterFederatedTrustDomain{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ClusterFederatedTrustDomain")
		os.Exit(1)
	}
	if err = (&spirev1alpha1.ClusterSPIFFEID{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ClusterSPIFFEID")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err = (&controllers.PodReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		Triggerer:        entryReconciler,
		IgnoreNamespaces: ctrlConfig.IgnoreNamespaces,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Pod")
		os.Exit(1)
	}

	if err = mgr.Add(manager.RunnableFunc(entryReconciler.Run)); err != nil {
		setupLog.Error(err, "unable to manage entry reconciler")
		os.Exit(1)
	}

	if err = mgr.Add(manager.RunnableFunc(federationRelationshipReconciler.Run)); err != nil {
		setupLog.Error(err, "unable to manage federation relationship reconciler")
		os.Exit(1)
	}

	if err = mgr.Add(webhookManager); err != nil {
		setupLog.Error(err, "unable to manage federation relationship reconciler")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
