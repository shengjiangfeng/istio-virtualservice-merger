/*
 * Copyright 2021 - now, the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *       https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package reconciler

import (
	"context"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

var (
	_        Context = &contextImpl{}
	instance Context
)

func GetContext() Context {
	if instance == nil {
		panic("No context instance. Call NewContext(mgr manager.Manager) to create the instance")
	}
	return instance
}

// NewContext creates a new reconciler Context
func NewContext(mgr manager.Manager) Context {
	instance = &contextImpl{manager: mgr}
	return instance
}

type contextImpl struct {
	manager manager.Manager
}

func (c *contextImpl) Logger() logr.Logger {
	return c.manager.GetLogger()
}

func (c *contextImpl) Client() client.Client {
	return c.manager.GetClient()
}

func (c *contextImpl) Scheme() *runtime.Scheme {
	return c.manager.GetScheme()
}

func (c *contextImpl) NewControllerBuilder() *builder.Builder {
	return ctrl.NewControllerManagedBy(c.manager)
}

func (c *contextImpl) SetOwnershipReference(owner metav1.Object, controlled metav1.Object) error {
	c.Logger().Info("Setting ownership reference to an object",
		"object", controlled.GetName(), "owner", owner.GetName())
	return controllerutil.SetControllerReference(owner, controlled, c.Scheme())
}

func (c *contextImpl) GetResource(key client.ObjectKey, object client.Object, foundCallback func() (err error), notFoundCallback func() (err error)) (err error) {
	if foundCallback == nil && notFoundCallback == nil {
		panic("Cannot have both un/found callbacks be nil")
	}
	err = c.Client().Get(context.TODO(), key, object)
	if err == nil && foundCallback != nil {
		return foundCallback()
	} else if errors.IsNotFound(err) {
		if notFoundCallback == nil {
			return nil
		}
		return notFoundCallback()
	}
	return
}

func (c *contextImpl) Run(req reconcile.Request, object KubeRuntimeObject, reconcile func(deleted bool) error) (reconcile.Result, error) {
	startTime := time.Now()
	start(req, c.Logger())
	defer end(req, startTime, c.Logger())
	if err := c.Client().Get(context.TODO(), req.NamespacedName, object); err != nil {
		if errors.IsNotFound(err) {
			// The runtime object is not found. Kubernetes will automatically
			// garbage collect all owned resources - return but do not requeue
			return complete(req, c.Logger())
		}
		// Read error; requeue here
		return errored(err, req, c.Logger())
	}
	if delTime := object.GetDeletionTimestamp(); delTime != nil {
		c.Logger().Info("The request object has been scheduled for delete",
			"Timestamp", time.Until(delTime.Time).Seconds())
		// The runtime object is marked for deletion - return but do not requeue
		if err := reconcile(true); err != nil {
			return errored(err, req, c.Logger())
		}
		return complete(req, c.Logger())
	}

	if df, ok := object.(Defaulting); ok && df.SetSpecDefaults() {
		c.Logger().Info("Setting the default spec of the request object")
		if err := c.Client().Update(context.TODO(), object); err != nil {
			return errored(err, req, c.Logger())
		}
	}
	if df, ok := object.(Defaulting); ok && df.SetStatusDefaults() {
		c.Logger().Info("Setting the default status of the request object")
		if err := c.Client().Status().Update(context.TODO(), object); err != nil {
			return errored(err, req, c.Logger())
		}
	}
	if err := reconcile(false); err != nil {
		return errored(err, req, c.Logger())
	}
	return complete(req, c.Logger())
}

func complete(req reconcile.Request, logger logr.Logger) (reconcile.Result, error) {
	logger.Info("[Complete] Reconciliation",
		"Request.Namespace",
		req.NamespacedName, "Request.Name",
		req.Name,
	)
	return reconcile.Result{Requeue: false}, nil
}

func errored(err error, req reconcile.Request, logger logr.Logger) (reconcile.Result, error) {
	logger.Error(err, "[Error] Reconciliation",
		"Request.Namespace",
		req.NamespacedName, "Request.Name",
		req.Name,
	)
	return reconcile.Result{Requeue: true}, err
}

func start(req reconcile.Request, logger logr.Logger) {
	logger.Info("[Start] Reconciliation",
		"Request.Namespace",
		req.Namespace, "Request.Name",
		req.Name,
	)
}

func end(req reconcile.Request, startTime time.Time, logger logr.Logger) {
	duration := time.Since(startTime).Seconds()
	logger.Info("[End] Reconciliation",
		"Request.Namespace",
		req.NamespacedName, "Request.Name",
		req.Name, "durationSec", duration,
	)
}
