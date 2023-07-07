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

package spireentry

import (
	"io"
	"strings"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/spire-controller-manager/pkg/spireapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	clusterStaticEntryLogKey = "clusterStaticEntry"
	clusterSPIFFEIDLogKey    = "clusterSPIFFEID"
	namespaceLogKey          = "namespace"
	podLogKey                = "pod"
	idKey                    = "id"
	parentIDKey              = "parentID"
	spiffeIDKey              = "spiffeID"
	selectorsKey             = "selectors"
	x509SVIDTTLKey           = "x509SVIDTTL"
	jwtSVIDTTLKey            = "jwtSVIDTTL"
	federatesWithKey         = "federatesWith"
	dnsNamesKey              = "dnsNames"
	adminKey                 = "admin"
	downstreamKey            = "downstream"
	hintKey                  = "hint"
)

func objectName(o metav1.Object) string {
	return (types.NamespacedName{
		Namespace: o.GetNamespace(),
		Name:      o.GetName(),
	}).String()
}

func entryLogFields(entry spireapi.Entry) []interface{} {
	return []interface{}{
		idKey, entry.ID,
		parentIDKey, entry.ParentID.String(),
		spiffeIDKey, entry.SPIFFEID.String(),
		x509SVIDTTLKey, entry.X509SVIDTTL.String(),
		jwtSVIDTTLKey, entry.JWTSVIDTTL.String(),
		selectorsKey, stringFromSelectors(entry.Selectors),
		federatesWithKey, stringFromTrustDomains(entry.FederatesWith),
		dnsNamesKey, stringList(entry.DNSNames),
		adminKey, entry.Admin,
		downstreamKey, entry.Downstream,
		hintKey, entry.Hint,
	}
}

func stringFromTrustDomains(tds []spiffeid.TrustDomain) string {
	return renderList(len(tds), func(i int, w io.StringWriter) {
		_, _ = w.WriteString(tds[i].String())
	})
}

func stringFromSelectors(selectors []spireapi.Selector) string {
	return renderList(len(selectors), func(i int, w io.StringWriter) {
		_, _ = w.WriteString(selectors[i].Type)
		_, _ = w.WriteString(":")
		_, _ = w.WriteString(selectors[i].Value)
	})
}

func stringList(ss []string) string {
	return renderList(len(ss), func(i int, w io.StringWriter) {
		_, _ = w.WriteString(ss[i])
	})
}

func renderList(n int, fn func(i int, w io.StringWriter)) string {
	var builder strings.Builder
	builder.WriteRune('[')
	for i := 0; i < n; i++ {
		if i > 0 {
			builder.WriteRune(',')
		}
		fn(i, &builder)
	}
	builder.WriteRune(']')
	return builder.String()
}
