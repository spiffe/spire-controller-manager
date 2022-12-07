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

package reconciler

import (
	"context"
	"fmt"
	"time"

	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const EndpointUID string = "subsets.addresses.targetRef.uid"

type Triggerer interface {
	Trigger()
}

type Reconciler interface {
	Trigger()
	Run(ctx context.Context) error
}

type Config struct {
	Kind       string
	Reconcile  func(ctx context.Context)
	GCInterval time.Duration
	Clock      clock.Clock
}

func New(config Config) Reconciler {
	if config.Clock == nil {
		config.Clock = clock.RealClock{}
	}
	return &reconciler{
		kind:       config.Kind,
		reconcile:  config.Reconcile,
		gcInterval: config.GCInterval,
		clock:      config.Clock,
		triggerCh:  make(chan struct{}),
	}
}

type reconciler struct {
	kind       string
	reconcile  func(ctx context.Context)
	gcInterval time.Duration
	clock      clock.Clock
	triggerCh  chan struct{}
}

func (r *reconciler) Trigger() {
	select {
	case r.triggerCh <- struct{}{}:
	default:
	}
}

func (r *reconciler) Run(ctx context.Context) error {
	ctx = withLogName(ctx, fmt.Sprintf("%s-reconciler", r.kind))
	log := log.FromContext(ctx)

	// Drain the trigger channel. This isn't strictly necessary but
	// prevents (but not fully) doing an extra reconcile if reconciliation
	// is triggered before the loop is entered.
	r.drain()

	var timer clock.Timer
	for {
		log.V(2).Info("Starting reconciliation")
		r.reconcile(ctx)
		log.V(2).Info("Reconciliation finished")

		log.V(2).Info("Waiting for next reconciliation")

		if timer == nil {
			timer = r.clock.NewTimer(r.gcInterval)
			defer timer.Stop()
		} else {
			timer.Reset(r.gcInterval)
		}

		select {
		case <-ctx.Done():
			log.Info("Reconciliation canceled")
			return ctx.Err()
		case <-timer.C():
			log.V(2).Info("Performing periodic reconciliation")
		case <-r.triggerCh:
			log.V(2).Info("Performing triggered reconciliation")
		}
	}
}

func (r *reconciler) drain() {
	select {
	case <-r.triggerCh:
	default:
	}
}

func withLogName(ctx context.Context, name string) context.Context {
	return log.IntoContext(ctx, log.FromContext(ctx).WithName(name))
}
