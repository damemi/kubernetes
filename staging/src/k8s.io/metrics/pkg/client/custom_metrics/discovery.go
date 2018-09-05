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
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"

	cmint "k8s.io/metrics/pkg/apis/custom_metrics"
)

var (
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

func NewAvailableAPIsGetter(client discovery.DiscoveryInterface) AvailableAPIsGetter {
	return &apiVersionsFromDiscovery{
		client: client,
	}
}

type apiVersionsFromDiscovery struct {
	client discovery.DiscoveryInterface

	// just cache the group directly since the discovery interface doesn't yet allow
	// asking for a single API group's versions.
	prefVersion *schema.GroupVersion
	mu          sync.RWMutex
}

// maybeFetchVersions fetches the versions, but doesn't try to invalidate on cache misses.
// Explicitly returns false if a cache miss occurred (i.e. our cached discovery info doesn't
// have the metrics API registered).
func (d *apiVersionsFromDiscovery) fetchVersions() (*metav1.APIGroup, error) {
	// TODO(directxman12): amend the discovery interface to ask for a particular group (/apis/foo)
	groups, err := d.client.ServerGroups()
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("no custom metrics API (%s) registered", cmint.GroupName)
	}

	return apiGroup, nil
}

func (d *apiVersionsFromDiscovery) chooseVersion(apiGroup *metav1.APIGroup) (schema.GroupVersion, error) {
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
		return schema.GroupVersion{}, fmt.Errorf("no known available metric versions found")
	}
	return *preferredVersion, nil
}

func (d *apiVersionsFromDiscovery) PreferredVersion() (schema.GroupVersion, error) {
	d.mu.RLock()
	if d.prefVersion != nil {
		// if we've already got one, proceeed with that
		defer d.mu.RUnlock()
		return *d.prefVersion, nil
	}
	d.mu.RUnlock()

	d.mu.Lock()
	defer d.mu.Unlock()

	// double check, someone might have beaten us to it
	if d.prefVersion != nil {
		return *d.prefVersion, nil
	}

	// populate our cache
	groupInfo, err := d.fetchVersions()
	if err != nil {
		return schema.GroupVersion{}, err
	}
	prefVersion, err := d.chooseVersion(groupInfo)
	if err != nil {
		return schema.GroupVersion{}, err
	}

	d.prefVersion = &prefVersion
	return *d.prefVersion, nil
}

func (d *apiVersionsFromDiscovery) Invalidate() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.prefVersion = nil
}
