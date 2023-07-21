package namespace_test

import (
	"regexp"
	"testing"

	"github.com/spiffe/spire-controller-manager/pkg/namespace"
	"github.com/stretchr/testify/require"
)

func TestIsIgnored(t *testing.T) {
	ignoredNamespaces := []*regexp.Regexp{
		regexp.MustCompile("s([a-z]+)re"),
		regexp.MustCompile("default"),
	}

	tests := []struct {
		namespace string
		expected  bool
	}{
		{"spire", true},
		{"default", true},
		{"spiffe", false},
		{"kubernetes", false},
	}

	for _, test := range tests {
		actual := namespace.IsIgnored(ignoredNamespaces, test.namespace)
		require.Equalf(t, test.expected, actual, "IsIgnored(%s, %s): expected does not equal actual",
			ignoredNamespaces, test.namespace)
	}
}
