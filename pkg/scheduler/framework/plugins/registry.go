/*
Copyright 2019 The Kubernetes Authors.

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

package plugins

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/defaultbinder"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/defaultpodtopologyspread"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/imagelocality"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/interpodaffinity"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/nodeaffinity"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/nodelabel"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/nodename"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/nodeports"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/nodepreferavoidpods"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/noderesources"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/nodeunschedulable"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/nodevolumelimits"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/podtopologyspread"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/queuesort"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/serviceaffinity"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/tainttoleration"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/volumebinding"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/volumerestrictions"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/volumezone"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
)

type registryOptions struct {
	ignoredResources []string
}

// Option configures a registry
type Option func(*registryOptions)

func WithIgnoredResources(r ...string) Option {
	return func(o *registryOptions) {
		o.ignoredResources = r
	}
}

// NewInTreeRegistry builds the registry with all the in-tree plugins.
// A scheduler that runs out of tree plugins can register additional plugins
// through the WithFrameworkOutOfTreeRegistry option.
func NewInTreeRegistry(opts ...Option) framework.Registry {
	var options registryOptions
	for _, opt := range opts {
		opt(&options)
	}
	return framework.Registry{
		defaultpodtopologyspread.Name: defaultpodtopologyspread.New,
		imagelocality.Name:            imagelocality.New,
		tainttoleration.Name:          tainttoleration.New,
		nodename.Name:                 nodename.New,
		nodeports.Name:                nodeports.New,
		nodepreferavoidpods.Name:      nodepreferavoidpods.New,
		nodeaffinity.Name:             nodeaffinity.New,
		podtopologyspread.Name:        podtopologyspread.New,
		nodeunschedulable.Name:        nodeunschedulable.New,
		noderesources.FitName: func(plArgs *runtime.Unknown, handle framework.FrameworkHandle) (framework.Plugin, error) {
			args := &noderesources.FitArgs{}
			if err := framework.DecodeInto(plArgs, args); err != nil {
				return nil, err
			}
			ignoredResources := sets.NewString(append(args.IgnoredResources, options.ignoredResources...)...)
			args.IgnoredResources = ignoredResources.List()
			newArgs, err := json.Marshal(args)
			if err != nil {
				return nil, err
			}
			plugin, err := noderesources.NewFit(&runtime.Unknown{Raw: newArgs}, handle)
			if err != nil {
				return nil, err
			}
			return plugin, nil
		},
		noderesources.BalancedAllocationName:       noderesources.NewBalancedAllocation,
		noderesources.MostAllocatedName:            noderesources.NewMostAllocated,
		noderesources.LeastAllocatedName:           noderesources.NewLeastAllocated,
		noderesources.RequestedToCapacityRatioName: noderesources.NewRequestedToCapacityRatio,
		noderesources.ResourceLimitsName:           noderesources.NewResourceLimits,
		volumebinding.Name:                         volumebinding.New,
		volumerestrictions.Name:                    volumerestrictions.New,
		volumezone.Name:                            volumezone.New,
		nodevolumelimits.CSIName:                   nodevolumelimits.NewCSI,
		nodevolumelimits.EBSName:                   nodevolumelimits.NewEBS,
		nodevolumelimits.GCEPDName:                 nodevolumelimits.NewGCEPD,
		nodevolumelimits.AzureDiskName:             nodevolumelimits.NewAzureDisk,
		nodevolumelimits.CinderName:                nodevolumelimits.NewCinder,
		interpodaffinity.Name:                      interpodaffinity.New,
		nodelabel.Name:                             nodelabel.New,
		serviceaffinity.Name:                       serviceaffinity.New,
		queuesort.Name:                             queuesort.New,
		defaultbinder.Name:                         defaultbinder.New,
	}
}
