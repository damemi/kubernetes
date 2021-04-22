/*
Copyright 2016 The Kubernetes Authors.

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

// Package app implements a server that runs a set of active
// components.  This includes replication controllers, service endpoints and
// nodes.
//
package app

import (
	"context"
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime/schema"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/kubernetes/pkg/controller/cronjob"
	"k8s.io/kubernetes/pkg/controller/job"
	kubefeatures "k8s.io/kubernetes/pkg/features"
)

func startJobController(ctx context.Context, controllerContext ControllerContext) (http.Handler, bool, error) {
	if !controllerContext.AvailableResources[schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}] {
		return nil, false, nil
	}
	go job.NewController(
		controllerContext.InformerFactory.Core().V1().Pods(),
		controllerContext.InformerFactory.Batch().V1().Jobs(),
		controllerContext.ClientBuilder.ClientOrDie("job-controller"),
	).Run(int(controllerContext.ComponentConfig.JobController.ConcurrentJobSyncs), controllerContext.Stop)
	return nil, true, nil
}

func startCronJobController(ctx context.Context, controllerContext ControllerContext) (http.Handler, bool, error) {
	if !controllerContext.AvailableResources[schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}] {
		return nil, false, nil
	}
	if utilfeature.DefaultFeatureGate.Enabled(kubefeatures.CronJobControllerV2) {
		cj2c, err := cronjob.NewControllerV2(controllerContext.InformerFactory.Batch().V1().Jobs(),
			controllerContext.InformerFactory.Batch().V1().CronJobs(),
			controllerContext.ClientBuilder.ClientOrDie("cronjob-controller"),
		)
		if err != nil {
			return nil, true, fmt.Errorf("error creating CronJob controller V2: %v", err)
		}
		go cj2c.Run(int(controllerContext.ComponentConfig.CronJobController.ConcurrentCronJobSyncs), controllerContext.Stop)
		return nil, true, nil
	}
	cjc, err := cronjob.NewController(
		controllerContext.ClientBuilder.ClientOrDie("cronjob-controller"),
	)
	if err != nil {
		return nil, true, fmt.Errorf("error creating CronJob controller: %v", err)
	}
	go cjc.Run(controllerContext.Stop)
	return nil, true, nil
}
