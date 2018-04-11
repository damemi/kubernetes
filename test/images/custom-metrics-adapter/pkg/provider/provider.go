/*
Copyright 2017 The Kubernetes Authors.

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

package provider

import (
	"fmt"
	"net/http"
	"time"

	"github.com/emicklei/go-restful"
	"github.com/golang/glog"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/metrics/pkg/apis/custom_metrics"

	"github.com/kubernetes-incubator/custom-metrics-apiserver/pkg/provider"
)

type E2EProvider struct {
	client dynamic.ClientPool
	mapper apimeta.RESTMapper

	values map[CustomMetricResource]int64
}

type MetricValue struct {
	Value int64
}

type CustomMetricResource struct {
	provider.CustomMetricInfo

	Name      string
	Namespace string
}

func NewE2EProvider(client dynamic.ClientPool, mapper apimeta.RESTMapper) provider.CustomMetricsProvider {
	return &E2EProvider{
		client: client,
		mapper: mapper,
		values: make(map[CustomMetricResource]int64),
	}
}

func (p *E2EProvider) WebService() *restful.WebService {
	ws := new(restful.WebService)

	ws.Path("/write-metrics")

	// Namespaced resources
	ws.Route(ws.POST("/namespaces/{namespace}/{resourceType}/{name}/{metric}").To(p.updateResource).
		Param(ws.BodyParameter("value", "value to set metric").DataType("integer").DefaultValue("0")))

	// Root-scoped resources
	ws.Route(ws.POST("/{resourceType}/{name}/{metric}").To(p.updateResource).
		Param(ws.BodyParameter("value", "value to set metric").DataType("integer").DefaultValue("0")))

	// Namespaces, where {resourceType} == "namespaces" to match API
	ws.Route(ws.POST("/{resourceType}/{name}/metrics/{metric}").To(p.updateResource).
		Param(ws.BodyParameter("value", "value to set metric").DataType("integer").DefaultValue("0")))
	return ws
}

func (p *E2EProvider) updateResource(request *restful.Request, response *restful.Response) {
	namespace := request.PathParameter("namespace")
	resourceType := request.PathParameter("resourceType")
	namespaced := false
	if len(namespace) > 0 || resourceType == "namespaces" {
		namespaced = true
	}
	name := request.PathParameter("name")
	metricName := request.PathParameter("metric")

	value := &MetricValue{}
	err := request.ReadEntity(&value)
	if err != nil {
		response.WriteErrorString(http.StatusBadRequest, err.Error())
		return
	}

	groupResource := schema.ParseGroupResource(resourceType)

	info := provider.CustomMetricInfo{
		GroupResource: groupResource,
		Metric:        metricName,
		Namespaced:    namespaced,
	}

	info, _, err = info.Normalized(p.mapper)
	if err != nil {
		glog.Errorf("Error normalizing info: %s", err)
	}

	metricInfo := CustomMetricResource{
		CustomMetricInfo: info,
		Name:             name,
		Namespace:        namespace,
	}
	p.values[metricInfo] = value.Value
}

func (p *E2EProvider) valueFor(groupResource schema.GroupResource, metricName, namespace, name string, namespaced bool) (int64, error) {
	info := provider.CustomMetricInfo{
		GroupResource: groupResource,
		Metric:        metricName,
		Namespaced:    namespaced,
	}

	info, _, err := info.Normalized(p.mapper)
	if err != nil {
		glog.Errorf("Error normalizing info: %s", err)
	}

	metricInfo := CustomMetricResource{
		CustomMetricInfo: info,
		Name:             name,
		Namespace:        namespace,
	}

	value, found := p.values[metricInfo]
	if !found {
		return 0, provider.NewMetricNotFoundForError(groupResource, metricName, name)
	}

	return value, nil
}

func (p *E2EProvider) metricFor(value int64, groupResource schema.GroupResource, namespace string, name string, metricName string) (*custom_metrics.MetricValue, error) {
	kind, err := p.mapper.KindFor(groupResource.WithVersion(""))
	if err != nil {
		return nil, err
	}

	return &custom_metrics.MetricValue{
		DescribedObject: custom_metrics.ObjectReference{
			APIVersion: groupResource.Group + "/" + runtime.APIVersionInternal,
			Kind:       kind.Kind,
			Name:       name,
			Namespace:  namespace,
		},
		MetricName: metricName,
		Timestamp:  metav1.Time{time.Now()},
		Value:      *resource.NewMilliQuantity(value*1000, resource.DecimalSI),
	}, nil
}

func (p *E2EProvider) metricsFor(groupResource schema.GroupResource, metricName string, list runtime.Object, namespaced bool) (*custom_metrics.MetricValueList, error) {
	if !apimeta.IsListType(list) {
		return nil, fmt.Errorf("returned object was not a list")
	}

	res := make([]custom_metrics.MetricValue, 0)

	err := apimeta.EachListItem(list, func(item runtime.Object) error {
		objMeta := item.(metav1.Object)
		value, err := p.valueFor(groupResource, metricName, objMeta.GetNamespace(), objMeta.GetName(), namespaced)
		if err != nil {
			if apierr.IsNotFound(err) {
				return nil
			}
			return err
		}

		metric, err := p.metricFor(value, groupResource, objMeta.GetNamespace(), objMeta.GetName(), metricName)
		if err != nil {
			return err
		}
		res = append(res, *metric)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &custom_metrics.MetricValueList{
		Items: res,
	}, nil
}

func (p *E2EProvider) GetRootScopedMetricByName(groupResource schema.GroupResource, name string, metricName string) (*custom_metrics.MetricValue, error) {
	value, err := p.valueFor(groupResource, metricName, "", name, false)
	if err != nil {
		return nil, err
	}
	return p.metricFor(value, groupResource, "", name, metricName)
}

func (p *E2EProvider) GetRootScopedMetricBySelector(groupResource schema.GroupResource, selector labels.Selector, metricName string) (*custom_metrics.MetricValueList, error) {
	// construct a client to list the names of objects matching the label selector
	client, err := p.client.ClientForGroupVersionResource(groupResource.WithVersion(""))
	if err != nil {
		glog.Errorf("unable to construct dynamic client to list matching resource names: %v", err)
		// don't leak implementation details to the user
		return nil, apierr.NewInternalError(fmt.Errorf("unable to list matching resources"))
	}

	// we can construct a this APIResource ourself, since the dynamic client only uses Name and Namespaced
	apiRes := &metav1.APIResource{
		Name:       groupResource.Resource,
		Namespaced: false,
	}

	matchingObjectsRaw, err := client.Resource(apiRes, "").
		List(metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	return p.metricsFor(groupResource, metricName, matchingObjectsRaw, false)
}

func (p *E2EProvider) GetNamespacedMetricByName(groupResource schema.GroupResource, namespace string, name string, metricName string) (*custom_metrics.MetricValue, error) {
	value, err := p.valueFor(groupResource, metricName, namespace, name, true)
	if err != nil {
		return nil, err
	}
	return p.metricFor(value, groupResource, namespace, name, metricName)
}

func (p *E2EProvider) GetNamespacedMetricBySelector(groupResource schema.GroupResource, namespace string, selector labels.Selector, metricName string) (*custom_metrics.MetricValueList, error) {
	// construct a client to list the names of objects matching the label selector
	client, err := p.client.ClientForGroupVersionResource(groupResource.WithVersion(""))
	if err != nil {
		glog.Errorf("unable to construct dynamic client to list matching resource names: %v", err)
		// don't leak implementation details to the user
		return nil, apierr.NewInternalError(fmt.Errorf("unable to list matching resources"))
	}

	// we can construct a this APIResource ourself, since the dynamic client only uses Name and Namespaced
	apiRes := &metav1.APIResource{
		Name:       groupResource.Resource,
		Namespaced: true,
	}

	matchingObjectsRaw, err := client.Resource(apiRes, namespace).
		List(metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	return p.metricsFor(groupResource, metricName, matchingObjectsRaw, true)
}

func (p *E2EProvider) ListAllMetrics() []provider.CustomMetricInfo {
	// Get unique CustomMetricInfos from wrapper CustomMetricResources
	infos := make(map[provider.CustomMetricInfo]struct{})
	for resource := range p.values {
		infos[resource.CustomMetricInfo] = struct{}{}
	}

	// Build slice of CustomMetricInfos to be returns
	metrics := make([]provider.CustomMetricInfo, 0, len(infos))
	for info := range infos {
		metrics = append(metrics, info)
	}

	return metrics
}
