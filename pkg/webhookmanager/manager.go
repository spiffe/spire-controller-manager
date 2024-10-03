/*
Copyright 2022 SPIRE Authors.

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

package webhookmanager

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/spire-controller-manager/pkg/spireapi"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	admissionregistrationapiv1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1"
	"k8s.io/client-go/tools/cache"

	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	x509SVIDTTL = time.Hour * 24
)

type Config struct {
	ID            spiffeid.ID
	KeyPairPath   string
	WebhookName   string
	WebhookClient admissionregistrationapiv1.ValidatingWebhookConfigurationInterface
	SVIDClient    spireapi.SVIDClient
	BundleClient  spireapi.BundleClient
	Clock         clock.WithTicker
}

type Manager struct {
	config Config

	mtx       sync.RWMutex
	rotatedAt time.Time
	expiresAt time.Time
	dnsNames  []string
	caBundle  []byte
}

func New(config Config) *Manager {
	if config.Clock == nil {
		config.Clock = clock.RealClock{}
	}
	return &Manager{
		config: config,
	}
}

func (m *Manager) Init(ctx context.Context) error {
	ctx = withLogName(ctx, "webhook-manager")

	if err := m.refreshBundle(ctx); err != nil {
		return fmt.Errorf("failed to refresh bundle: %w", err)
	}

	webhookConfig, err := m.config.WebhookClient.Get(ctx, m.config.WebhookName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to obtain webhook config: %w", err)
	}

	// Create a temporary cache store to and populate it with our webhook config
	// to pass to the following functions.
	tempStore := cache.NewStore(cache.MetaNamespaceKeyFunc)
	if err := tempStore.Add(webhookConfig); err != nil {
		return fmt.Errorf("failed to populate temporary cache: %w", err)
	}

	if err := m.mintX509SVIDIfNeeded(ctx, tempStore); err != nil {
		return fmt.Errorf("failed to mint SVID: %w", err)
	}

	if err := m.updateWebhookConfigIfNeeded(ctx, tempStore); err != nil {
		return fmt.Errorf("failed to updated webhook config: %w", err)
	}

	return nil
}

func (m *Manager) Start(ctx context.Context) error {
	ctx = withLogName(ctx, "webhook-manager")

	log := log.FromContext(ctx)

	store, webhookChangedCh, cleanup := startInformer(ctx, m.config)
	defer cleanup()

	// Check every second if the SVID has expired or needs to change and
	// backoff up to a minute on failures to mint.
	svidTimer := newBackoffTimer(m.config.Clock, time.Second, time.Minute)

	// Refresh the bundle every 5 seconds, and back off up to a minute
	// on failure.
	bundleTimer := newBackoffTimer(m.config.Clock, 5*time.Second, time.Minute)

	// Evaluate the webhook consistency every 5 seconds and back off up to a
	// minute on failure to update the webhook. Checking consistency uses the
	// cache and does NOT hit the API.
	webhookTimer := newBackoffTimer(m.config.Clock, 5*time.Second, time.Minute)

	for {
		select {
		case <-svidTimer.C():
			if err := m.mintX509SVIDIfNeeded(ctx, store); err != nil {
				log.Error(err, "Failed to mint X509-SVID")
				svidTimer.BackOff()
			} else {
				svidTimer.Reset()
			}
		case <-bundleTimer.C():
			if err := m.refreshBundle(ctx); err != nil {
				log.Error(err, "Failed to refresh bundle")
				bundleTimer.BackOff()
			} else {
				bundleTimer.Reset()
				if err := m.updateWebhookConfigIfNeeded(ctx, store); err != nil {
					log.Error(err, "Failed to update webhook config if needed")
				}
				webhookTimer.Reset()
			}
		case <-webhookTimer.C():
			if err := m.updateWebhookConfigIfNeeded(ctx, store); err != nil {
				log.Error(err, "Failed to update webhook config if needed")
				webhookTimer.BackOff()
			} else {
				webhookTimer.Reset()
			}
		case <-webhookChangedCh:
			if err := m.updateWebhookConfigIfNeeded(ctx, store); err != nil {
				log.Error(err, "Failed to update webhook config if needed")
			}
			// Whether we succeed or fail here, reset the webhook timer.
			webhookTimer.Reset()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (m *Manager) mintX509SVIDIfNeeded(ctx context.Context, store cache.Store) error {
	log := log.FromContext(ctx)

	m.mtx.RLock()
	rotatedAt, expiresAt := m.rotatedAt, m.expiresAt
	currentDNSNames := m.dnsNames
	m.mtx.RUnlock()

	webhookConfig, exists, err := getWebhookConfigFromStore(store, m.config.WebhookName)
	switch {
	case err != nil:
		return err
	case !exists:
		return nil
	}

	dnsNames := webhookDNSNames(webhookConfig)

	var lifetime time.Duration
	var expiresIn time.Duration
	if !rotatedAt.IsZero() {
		lifetime = expiresAt.Sub(rotatedAt)
		expiresIn = expiresAt.Sub(m.config.Clock.Now())
	}

	var reason string
	switch {
	case lifetime == 0:
		reason = "initializing"
	case expiresSoon(lifetime, expiresIn):
		reason = "expires soon"
	case expiresIn < 0:
		reason = "has expired"
	case !dnsNamesEqual(dnsNames, currentDNSNames):
		reason = "stale DNS names"
	default:
		return nil
	}

	log.Info("Minting webhook certificate", "reason", reason, "dnsNames", dnsNames)
	return m.mintX509SVID(ctx, dnsNames)
}

func (m *Manager) mintX509SVID(ctx context.Context, dnsNames []string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate X509-SVID private key: %w", err)
	}

	svid, err := m.config.SVIDClient.MintX509SVID(ctx, spireapi.X509SVIDParams{
		Key:      key,
		ID:       m.config.ID,
		DNSNames: dnsNames,
		TTL:      x509SVIDTTL,
	})
	if err != nil {
		return fmt.Errorf("failed to mint webhook certificate: %w", err)
	}

	data, err := marshalSVID(svid)
	if err != nil {
		return fmt.Errorf("failed to serialize webhook keypair: %w", err)
	}

	if err := os.WriteFile(m.config.KeyPairPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write webhook keypair: %w", err)
	}

	log.FromContext(ctx).Info("Minted webhook certificate")

	m.mtx.Lock()
	m.rotatedAt = m.config.Clock.Now()
	m.expiresAt = svid.ExpiresAt
	m.dnsNames = dnsNames
	m.mtx.Unlock()
	return nil
}

func (m *Manager) updateWebhookConfigIfNeeded(ctx context.Context, store cache.Store) error {
	m.mtx.RLock()
	caBundle := m.caBundle
	m.mtx.RUnlock()

	current, exists, err := getWebhookConfigFromStore(store, m.config.WebhookName)
	switch {
	case err != nil:
		return err
	case !exists:
		return nil
	}

	var modified *admissionregistrationv1.ValidatingWebhookConfiguration
	for i, webhook := range current.Webhooks {
		if bytes.Equal(webhook.ClientConfig.CABundle, caBundle) {
			continue
		}
		if modified == nil {
			modified = current.DeepCopy()
		}
		modified.Webhooks[i].ClientConfig.CABundle = caBundle
	}

	if modified != nil {
		data, err := client.StrategicMergeFrom(current).Data(modified)
		if err != nil {
			return fmt.Errorf("failed to create webhook configuration patch: %w", err)
		}
		if _, err := m.config.WebhookClient.Patch(ctx, m.config.WebhookName, types.StrategicMergePatchType, data, metav1.PatchOptions{}); err != nil {
			return fmt.Errorf("failed to patch webhook configuration: %w", err)
		}
		log.FromContext(ctx).Info("Webhook configuration patched with CABundle")
	}
	return nil
}

func (m *Manager) refreshBundle(ctx context.Context) error {
	bundle, err := m.config.BundleClient.GetBundle(ctx)
	if err != nil {
		return err
	}

	m.mtx.Lock()
	m.caBundle = marshalX509Authorities(bundle.X509Authorities())
	m.mtx.Unlock()
	return nil
}

func marshalX509Authorities(x509Authorities []*x509.Certificate) []byte {
	buf := new(bytes.Buffer)
	_ = encodeCertificates(buf, x509Authorities)
	return buf.Bytes()
}

func marshalSVID(svid *spireapi.X509SVID) ([]byte, error) {
	buf := new(bytes.Buffer)
	_ = encodeCertificates(buf, svid.CertChain)

	keyBytes, err := x509.MarshalPKCS8PrivateKey(svid.Key)
	if err != nil {
		return nil, err
	}

	_ = pem.Encode(buf, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyBytes,
	})

	return buf.Bytes(), nil
}

func encodeCertificates(w io.Writer, certs []*x509.Certificate) error {
	for _, cert := range certs {
		if err := pem.Encode(w, &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		}); err != nil {
			return err
		}
	}
	return nil
}

func withLogName(ctx context.Context, name string) context.Context {
	return log.IntoContext(ctx, log.FromContext(ctx).WithName(name))
}

func serviceDNSName(service *admissionregistrationv1.ServiceReference) (string, bool) {
	switch {
	case service == nil, service.Namespace == "", service.Name == "":
		return "", false
	}
	return fmt.Sprintf("%s.%s.svc", service.Name, service.Namespace), true
}

func webhookDNSNames(webhookConfig *admissionregistrationv1.ValidatingWebhookConfiguration) []string {
	dnsNamesSet := make(map[string]struct{})
	for _, webhook := range webhookConfig.Webhooks {
		if dnsName, ok := serviceDNSName(webhook.ClientConfig.Service); ok {
			dnsNamesSet[dnsName] = struct{}{}
		}
	}
	var dnsNames []string
	for dnsName := range dnsNamesSet {
		dnsNames = append(dnsNames, dnsName)
	}
	sort.Strings(dnsNames)
	return dnsNames
}

// dnsNamesEqual compares to lists of dns names for equality. They are assumed
// to be sorted, as returned by webhookDNSNames.
func dnsNamesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func startInformer(ctx context.Context, config Config) (cache.Store, chan struct{}, func()) {
	ch := make(chan struct{}, 1)

	notify := func() {
		select {
		case ch <- struct{}{}:
		default:
		}
	}

	log := log.FromContext(ctx)
	store, controller := cache.NewInformerWithOptions(cache.InformerOptions{
		ListerWatcher: &cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return config.WebhookClient.List(ctx, options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return config.WebhookClient.Watch(ctx, options)
			},
		},
		ObjectType:   &admissionregistrationv1.ValidatingWebhookConfiguration{},
		ResyncPeriod: time.Hour,
		Handler: cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				o, ok := obj.(*admissionregistrationv1.ValidatingWebhookConfiguration)
				return ok && o.Name == config.WebhookName
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc: func(_ interface{}) {
					log.Info("Received webhook added event")
					notify()
				},
				UpdateFunc: func(_, _ interface{}) {
					log.Info("Received webhook updated event")
					notify()
				},
				DeleteFunc: func(_ interface{}) {
					log.Info("Received webhook deleted event")
					notify()
				},
			},
		},
	})

	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		controller.Run(ctx.Done())
	}()
	return store, ch, wg.Wait
}

func expiresSoon(lifetime, expiresIn time.Duration) bool {
	const day = time.Hour * 24
	const week = day * 7
	const monthish = day * 30
	switch {
	case lifetime > monthish:
		return expiresIn < week
	case lifetime > week:
		return expiresIn < (week / 2)
	case lifetime > day:
		return expiresIn < (day / 2)
	case lifetime > time.Hour:
		return expiresIn < (time.Hour / 2)
	default:
		return expiresIn < (lifetime / 2)
	}
}

func getWebhookConfigFromStore(store cache.Store, name string) (*admissionregistrationv1.ValidatingWebhookConfiguration, bool, error) {
	obj, exists, err := store.GetByKey(name)
	if err != nil {
		return nil, false, fmt.Errorf("failed to obtain webhook config from cache: %w", err)
	}
	if !exists {
		return nil, false, nil
	}

	webhookConfig, ok := obj.(*admissionregistrationv1.ValidatingWebhookConfiguration)
	if !ok {
		return nil, false, fmt.Errorf("cached object is not a webhook config: %T", obj)
	}

	return webhookConfig, true, nil
}
