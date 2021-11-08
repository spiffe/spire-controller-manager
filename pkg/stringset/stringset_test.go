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

package stringset_test

import (
	"testing"

	"github.com/spiffe/spire-controller-manager/pkg/stringset"
	"github.com/stretchr/testify/require"
)

func TestStringSet(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ss := stringset.StringSet(nil)
		require.False(t, ss.In("foo"))
	})
	t.Run("non-empty", func(t *testing.T) {
		ss := stringset.StringSet([]string{"foo", "bar"})
		require.False(t, ss.In(""))
		require.True(t, ss.In("foo"))
		require.True(t, ss.In("bar"))
		require.False(t, ss.In("baz"))
	})
}
