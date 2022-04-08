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

package reconciler_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/spiffe/spire-controller-manager/pkg/reconciler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	testclock "k8s.io/utils/clock/testing"
)

func TestReconciler(t *testing.T) {
	clock := new(testclock.FakeClock)

	calledCh := make(chan struct{})
	checkIfCalled := func() bool {
		select {
		case <-calledCh:
			return true
		default:
			return false
		}
	}
	r := reconciler.New(reconciler.Config{
		Kind: "test",
		Reconcile: func(ctx context.Context) {
			t.Log("Reconcile called")
			select {
			case <-ctx.Done():
				assert.Fail(t, "Reconcile called after test closed")
			case calledCh <- struct{}{}:
				t.Log("Indicated that reconcile was called")
			}
		},
		GCInterval: time.Second,
		Clock:      clock,
	})

	errCh := make(chan error)
	t.Cleanup(func() {
		err := <-errCh
		assert.True(t, errors.Is(err, context.Canceled), "expected canceled error; got %f", err)
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		errCh <- r.Run(ctx)
	}()

	t.Log("Wait until the initial reconcile call")
	require.Eventually(t, checkIfCalled, time.Minute, time.Millisecond*10)

	t.Log("Wait until run is waiting")
	require.Eventually(t, clock.HasWaiters, time.Minute, time.Millisecond*10)

	t.Log("Step the clock")
	clock.Step(time.Second)

	t.Log("Wait until the GC reconcile call")
	require.Eventually(t, checkIfCalled, time.Minute, time.Millisecond*10)

	t.Log("Wait until run is waiting")
	require.Eventually(t, clock.HasWaiters, time.Minute, time.Millisecond*10)

	t.Log("Trigger reconciliation")
	r.Trigger()

	t.Log("Wait until the trigger reconcile call")
	require.Eventually(t, checkIfCalled, time.Minute, time.Millisecond*10)
}
