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

	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Triggerer interface {
	Trigger()
}

type Reconciler interface {
	Trigger()
	Run(ctx context.Context) error
}

func New(kind string, method func(ctx context.Context), gcInterval time.Duration) Reconciler {
	return &reconciler{
		kind:       kind,
		method:     method,
		gcInterval: gcInterval,
		triggerCh:  make(chan struct{}),
	}
}

type reconciler struct {
	kind       string
	method     func(ctx context.Context)
	gcInterval time.Duration
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

	// Initialize the timer for WAY out.... we'll reset it to the right
	// interval before selecting.
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()

	// Drain the reconcile channel. This isn't strictly necessary but
	// prevents (but not fully) doing an extra reconcile if reconciliation
	// is triggered before the loop is entered.
	r.drain()

	for {
		log.V(2).Info("Starting reconciliation")
		r.method(ctx)
		log.V(2).Info("Reconciliation finished")
		if err := ctx.Err(); err != nil {
			log.Info("Reconciliation canceled")
			return err
		}

		log.V(2).Info("Waiting for next reconciliation")
		timer.Reset(r.gcInterval)
		select {
		case <-ctx.Done():
			log.Info("Reconciliation canceled while waiting")
			return ctx.Err()
		case <-timer.C:
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
