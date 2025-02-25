/*
Copyright 2025 Rancher Labs, Inc.

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

// Code generated by main. DO NOT EDIT.

package v3

import (
	"context"
	"sync"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/shepherd/pkg/wrangler/pkg/generic"
	"github.com/rancher/wrangler/v2/pkg/apply"
	"github.com/rancher/wrangler/v2/pkg/condition"
	"github.com/rancher/wrangler/v2/pkg/kv"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ProjectAlertController interface for managing ProjectAlert resources.
type ProjectAlertController interface {
	generic.ControllerInterface[*v3.ProjectAlert, *v3.ProjectAlertList]
}

// ProjectAlertClient interface for managing ProjectAlert resources in Kubernetes.
type ProjectAlertClient interface {
	generic.ClientInterface[*v3.ProjectAlert, *v3.ProjectAlertList]
}

// ProjectAlertCache interface for retrieving ProjectAlert resources in memory.
type ProjectAlertCache interface {
	generic.CacheInterface[*v3.ProjectAlert]
}

// ProjectAlertStatusHandler is executed for every added or modified ProjectAlert. Should return the new status to be updated
type ProjectAlertStatusHandler func(obj *v3.ProjectAlert, status v3.AlertStatus) (v3.AlertStatus, error)

// ProjectAlertGeneratingHandler is the top-level handler that is executed for every ProjectAlert event. It extends ProjectAlertStatusHandler by a returning a slice of child objects to be passed to apply.Apply
type ProjectAlertGeneratingHandler func(obj *v3.ProjectAlert, status v3.AlertStatus) ([]runtime.Object, v3.AlertStatus, error)

// RegisterProjectAlertStatusHandler configures a ProjectAlertController to execute a ProjectAlertStatusHandler for every events observed.
// If a non-empty condition is provided, it will be updated in the status conditions for every handler execution
func RegisterProjectAlertStatusHandler(ctx context.Context, controller ProjectAlertController, condition condition.Cond, name string, handler ProjectAlertStatusHandler) {
	statusHandler := &projectAlertStatusHandler{
		client:    controller,
		condition: condition,
		handler:   handler,
	}
	controller.AddGenericHandler(ctx, name, generic.FromObjectHandlerToHandler(statusHandler.sync))
}

// RegisterProjectAlertGeneratingHandler configures a ProjectAlertController to execute a ProjectAlertGeneratingHandler for every events observed, passing the returned objects to the provided apply.Apply.
// If a non-empty condition is provided, it will be updated in the status conditions for every handler execution
func RegisterProjectAlertGeneratingHandler(ctx context.Context, controller ProjectAlertController, apply apply.Apply,
	condition condition.Cond, name string, handler ProjectAlertGeneratingHandler, opts *generic.GeneratingHandlerOptions) {
	statusHandler := &projectAlertGeneratingHandler{
		ProjectAlertGeneratingHandler: handler,
		apply:                         apply,
		name:                          name,
		gvk:                           controller.GroupVersionKind(),
	}
	if opts != nil {
		statusHandler.opts = *opts
	}
	controller.OnChange(ctx, name, statusHandler.Remove)
	RegisterProjectAlertStatusHandler(ctx, controller, condition, name, statusHandler.Handle)
}

type projectAlertStatusHandler struct {
	client    ProjectAlertClient
	condition condition.Cond
	handler   ProjectAlertStatusHandler
}

// sync is executed on every resource addition or modification. Executes the configured handlers and sends the updated status to the Kubernetes API
func (a *projectAlertStatusHandler) sync(key string, obj *v3.ProjectAlert) (*v3.ProjectAlert, error) {
	if obj == nil {
		return obj, nil
	}

	origStatus := obj.Status.DeepCopy()
	obj = obj.DeepCopy()
	newStatus, err := a.handler(obj, obj.Status)
	if err != nil {
		// Revert to old status on error
		newStatus = *origStatus.DeepCopy()
	}

	if a.condition != "" {
		if errors.IsConflict(err) {
			a.condition.SetError(&newStatus, "", nil)
		} else {
			a.condition.SetError(&newStatus, "", err)
		}
	}
	if !equality.Semantic.DeepEqual(origStatus, &newStatus) {
		if a.condition != "" {
			// Since status has changed, update the lastUpdatedTime
			a.condition.LastUpdated(&newStatus, time.Now().UTC().Format(time.RFC3339))
		}

		var newErr error
		obj.Status = newStatus
		newObj, newErr := a.client.UpdateStatus(obj)
		if err == nil {
			err = newErr
		}
		if newErr == nil {
			obj = newObj
		}
	}
	return obj, err
}

type projectAlertGeneratingHandler struct {
	ProjectAlertGeneratingHandler
	apply apply.Apply
	opts  generic.GeneratingHandlerOptions
	gvk   schema.GroupVersionKind
	name  string
	seen  sync.Map
}

// Remove handles the observed deletion of a resource, cascade deleting every associated resource previously applied
func (a *projectAlertGeneratingHandler) Remove(key string, obj *v3.ProjectAlert) (*v3.ProjectAlert, error) {
	if obj != nil {
		return obj, nil
	}

	obj = &v3.ProjectAlert{}
	obj.Namespace, obj.Name = kv.RSplit(key, "/")
	obj.SetGroupVersionKind(a.gvk)

	if a.opts.UniqueApplyForResourceVersion {
		a.seen.Delete(key)
	}

	return nil, generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects()
}

// Handle executes the configured ProjectAlertGeneratingHandler and pass the resulting objects to apply.Apply, finally returning the new status of the resource
func (a *projectAlertGeneratingHandler) Handle(obj *v3.ProjectAlert, status v3.AlertStatus) (v3.AlertStatus, error) {
	if !obj.DeletionTimestamp.IsZero() {
		return status, nil
	}

	objs, newStatus, err := a.ProjectAlertGeneratingHandler(obj, status)
	if err != nil {
		return newStatus, err
	}
	if !a.isNewResourceVersion(obj) {
		return newStatus, nil
	}

	err = generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects(objs...)
	if err != nil {
		return newStatus, err
	}
	a.storeResourceVersion(obj)
	return newStatus, nil
}

// isNewResourceVersion detects if a specific resource version was already successfully processed.
// Only used if UniqueApplyForResourceVersion is set in generic.GeneratingHandlerOptions
func (a *projectAlertGeneratingHandler) isNewResourceVersion(obj *v3.ProjectAlert) bool {
	if !a.opts.UniqueApplyForResourceVersion {
		return true
	}

	// Apply once per resource version
	key := obj.Namespace + "/" + obj.Name
	previous, ok := a.seen.Load(key)
	return !ok || previous != obj.ResourceVersion
}

// storeResourceVersion keeps track of the latest resource version of an object for which Apply was executed
// Only used if UniqueApplyForResourceVersion is set in generic.GeneratingHandlerOptions
func (a *projectAlertGeneratingHandler) storeResourceVersion(obj *v3.ProjectAlert) {
	if !a.opts.UniqueApplyForResourceVersion {
		return
	}

	key := obj.Namespace + "/" + obj.Name
	a.seen.Store(key, obj.ResourceVersion)
}
