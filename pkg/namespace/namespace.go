package namespace

import (
	"regexp"
)

func IsIgnored(ignoredNamespaces []*regexp.Regexp, namespace string) bool {
	for _, regex := range ignoredNamespaces {
		if regex.MatchString(namespace) {
			return true
		}
	}

	return false
}
