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

package spireapi

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCloser struct {
	err    error
	closed bool
}

func (f *fakeCloser) Close() error {
	f.closed = true
	return f.err
}

func TestMultiCloserClosesAll(t *testing.T) {
	a := &fakeCloser{}
	b := &fakeCloser{}

	err := multiCloser{a, b}.Close()
	require.NoError(t, err)
	assert.True(t, a.closed)
	assert.True(t, b.closed)
}

func TestMultiCloserReturnsFirstErrorButClosesAll(t *testing.T) {
	errA := errors.New("close a failed")
	errB := errors.New("close b failed")
	a := &fakeCloser{err: errA}
	b := &fakeCloser{err: errB}

	err := multiCloser{a, b}.Close()
	assert.Equal(t, errA, err)
	assert.True(t, a.closed)
	assert.True(t, b.closed)
}

func TestGetCallOptionsNilConfig(t *testing.T) {
	assert.Empty(t, getCallOptions(nil))
}

func TestGetCallOptionsMaxCallRecvMsgSize(t *testing.T) {
	opts := getCallOptions(&GrpcConfig{MaxCallRecvMsgSize: 1024})
	assert.Len(t, opts, 1)
}
