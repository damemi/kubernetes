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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/rest"

	cmint "k8s.io/metrics/pkg/apis/custom_metrics"
	cmv1beta1 "k8s.io/metrics/pkg/apis/custom_metrics/v1beta1"
	cmv1beta2 "k8s.io/metrics/pkg/apis/custom_metrics/v1beta2"
	"k8s.io/metrics/pkg/client/custom_metrics/scheme"
)

var (
	// MetricVersions is the set of metric versions accepted by the converter.
	MetricVersions = []schema.GroupVersion{
		cmv1beta2.SchemeGroupVersion,
		cmv1beta1.SchemeGroupVersion,
		cmint.SchemeGroupVersion,
	}

	// metricVersionsToGV is the map of string group-versions
	// accepted by the converter to group-version objects (so
	// we don't have to re-parse)
	metricVersionsToGV map[string]schema.GroupVersion
)

func init() {
	metricVersionsToGV = make(map[string]schema.GroupVersion)
	for _, ver := range MetricVersions {
		metricVersionsToGV[ver.String()] = ver
	}
}

type AvailableMetricsAPIFunc func() (*metav1.APIGroup, error)

type APIVersionsFromDiscovery struct {
	discovery.CachedDiscoveryInterface
}

// maybeFetchVersions fetches the versions, but doesn't try to invalidate on cache misses.
// Explicitly returns false if a cache miss occurred (i.e. our cached discovery info doesn't
// have the metrics API registered).
func (d APIVersionsFromDiscovery) maybeFetchVersions() (*metav1.APIGroup, bool, error) {
	groups, err := d.CachedDiscoveryInterface.ServerGroups()
	if err != nil {
		return nil, false, err
	}

	// Determine the preferred version on the server by first finding the custom metrics group
	var apiGroup *metav1.APIGroup
	for _, group := range groups.Groups {
		if group.Name == cmint.GroupName {
			apiGroup = &group
			break
		}
	}

	if apiGroup == nil {
		return nil, false, nil
	}

	return apiGroup, true, nil
}

func (d APIVersionsFromDiscovery) Versions() (*metav1.APIGroup, error) {
	// check our cached version
	group, present, err := d.maybeFetchVersions()
	if (err == nil && !present) || err == cached.ErrCacheEmpty {
		// we missed, so invalidate to fetch the latest info, and fetch again
		d.Invalidate()
		group, present, err = d.maybeFetchVersions()
	}
	if err != nil {
		return nil, err
	}
	if !present {
		// it wasn't in the latest info, so actually bail with an error
		return nil, fmt.Errorf("no metrics API group registered")
	}

	return group, nil
}

// MetricConverter knows how to convert between external MetricValue versions.
type MetricConverter struct {
	scheme            *runtime.Scheme
	codecs            serializer.CodecFactory
	internalVersioner runtime.GroupVersioner
	metricsVersions   AvailableMetricsAPIFunc
}

func NewMetricConverter(apiVersions AvailableMetricsAPIFunc) *MetricConverter {
	return &MetricConverter{
		scheme:          scheme.Scheme,
		codecs:          serializer.NewCodecFactory(scheme.Scheme),
		metricsVersions: apiVersions,
		internalVersioner: runtime.NewMultiGroupVersioner(
			scheme.SchemeGroupVersion,
			schema.GroupKind{Group: cmint.GroupName, Kind: ""},
			schema.GroupKind{Group: cmv1beta1.GroupName, Kind: ""},
			schema.GroupKind{Group: cmv1beta2.GroupName, Kind: ""},
		),
	}
}

// Scheme returns the scheme used by this metric converter.
func (c *MetricConverter) Scheme() *runtime.Scheme {
	return c.scheme
}

func (c *MetricConverter) Codecs() serializer.CodecFactory {
	return c.codecs
}

func (c *MetricConverter) negotiatePreferredVersion(apiGroup *metav1.APIGroup) (*schema.GroupVersion, error) {
	// Check if a preferred version is set in the APIGroup
	// If not, we need to compare all of the available versions to ours to find a match.
	var preferredVersion *schema.GroupVersion
	if gv, present := metricVersionsToGV[apiGroup.PreferredVersion.GroupVersion]; present && len(apiGroup.PreferredVersion.GroupVersion) != 0 {
		preferredVersion = &gv
	} else {
		for _, version := range apiGroup.Versions {
			if gv, present := metricVersionsToGV[version.GroupVersion]; present {
				preferredVersion = &gv
				break
			}
		}
	}

	if preferredVersion == nil {
		return nil, fmt.Errorf("no known available metric versions found")
	}
	return preferredVersion, nil
}

func (c *MetricConverter) ConvertListOptionsToPreferredVersion(opts *cmint.MetricListOptions) (runtime.Object, error) {
	apiGroup, err := c.metricsVersions()
	if err != nil {
		return nil, err
	}
	preferredVersion, err := c.negotiatePreferredVersion(apiGroup)
	if err != nil {
		return nil, err
	}
	paramObj, err := c.UnsafeConvertToVersionVia(opts, *preferredVersion)
	if err != nil {
		return nil, err
	}
	return paramObj, nil
}

func (c *MetricConverter) ConvertResultToVersion(res rest.Result, gv schema.GroupVersion) (runtime.Object, error) {
	if err := res.Error(); err != nil {
		return nil, err
	}

	metricBytes, err := res.Raw()
	if err != nil {
		return nil, err
	}

	decoder := c.codecs.UniversalDecoder(MetricVersions...)
	rawMetricObj, err := runtime.Decode(decoder, metricBytes)
	if err != nil {
		return nil, err
	}

	metricObj, err := c.UnsafeConvertToVersionVia(rawMetricObj, gv)
	if err != nil {
		return nil, err
	}
	return metricObj, nil
}

// unsafeConvertToVersionVia is like Scheme.UnsafeConvertToVersion, but it does so via an internal version first.
// We use it here to work with the v1beta2 client internally, while preserving backwards compatibility for existing custom metrics adapters
func (c *MetricConverter) UnsafeConvertToVersionVia(obj runtime.Object, externalVersion schema.GroupVersion) (runtime.Object, error) {
	objInt, err := c.scheme.UnsafeConvertToVersion(obj, schema.GroupVersion{Group: externalVersion.Group, Version: runtime.APIVersionInternal})
	if err != nil {
		return nil, fmt.Errorf("failed to convert the given object to the internal version: %v", err)
	}

	objExt, err := c.scheme.UnsafeConvertToVersion(objInt, externalVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to convert the given object back to the external version: %v", err)
	}

	return objExt, err
}
