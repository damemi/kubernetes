/*
Copyright 2018 The Kubernetes Authors.

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

package custom_metrics

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	cmint "k8s.io/metrics/pkg/apis/custom_metrics"
	cmv1beta1 "k8s.io/metrics/pkg/apis/custom_metrics/v1beta1"
	cmv1beta2 "k8s.io/metrics/pkg/apis/custom_metrics/v1beta2"
)

// makeAPIGroup makes a new metrics API group with the given versions.
func makeAPIGroup(prefVer string, versions ...string) *metav1.APIGroup {
	discoVersions := make([]metav1.GroupVersionForDiscovery, len(versions))
	for i, ver := range versions {
		discoVersions[i] = metav1.GroupVersionForDiscovery{
			Version: ver,
			GroupVersion: schema.GroupVersion{
				Group:   cmint.SchemeGroupVersion.Group,
				Version: ver,
			}.String(),
		}
	}

	var discoPrefVer metav1.GroupVersionForDiscovery
	if prefVer != "" {
		discoPrefVer = metav1.GroupVersionForDiscovery{
			Version: prefVer,
			GroupVersion: schema.GroupVersion{
				Group:   cmint.SchemeGroupVersion.Group,
				Version: prefVer,
			}.String(),
		}
	}

	return &metav1.APIGroup{
		Name:             cmint.SchemeGroupVersion.Group,
		Versions:         discoVersions,
		PreferredVersion: discoPrefVer,
	}
}

func TestMetricConverter(t *testing.T) {
	testCases := []struct {
		name     string
		group    *metav1.APIGroup
		expected runtime.Object
	}{
		{
			name:  "Use preferred version when set",
			group: makeAPIGroup("v1beta1", "v1beta2"),
			expected: &cmv1beta1.MetricListOptions{
				TypeMeta:            metav1.TypeMeta{Kind: "MetricListOptions", APIVersion: cmv1beta1.SchemeGroupVersion.String()},
				MetricLabelSelector: "foo",
			},
		},
		{
			name:  "Use first available version when no preferred is set",
			group: makeAPIGroup("", "v1beta2", "v1beta1"),
			expected: &cmv1beta2.MetricListOptions{
				TypeMeta:            metav1.TypeMeta{Kind: "MetricListOptions", APIVersion: cmv1beta2.SchemeGroupVersion.String()},
				MetricLabelSelector: "foo",
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			metricConverter := NewMetricConverter(func() (*metav1.APIGroup, error) { return test.group, nil })
			opts := &cmint.MetricListOptions{MetricLabelSelector: "foo"}
			res, err := metricConverter.ConvertListOptionsToPreferredVersion(opts)
			require.NoError(t, err)
			require.Equal(t, test.expected, res)
		})
	}
}
