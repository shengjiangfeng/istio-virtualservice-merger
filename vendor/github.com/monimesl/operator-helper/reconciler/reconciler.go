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
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"log"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Configure let the added reconcilers to configure themselves
func Configure(manager ctrl.Manager, reconcilers ...Reconciler) error {
	ctx := NewContext(manager)
	for _, r := range reconcilers {
		log.Printf("configuring the reconciler: %T\n", r)
		if err := r.Configure(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Reconciler presents the interface to be
// implemented by a controllers-runtime controllers
type Reconciler interface {
	reconcile.Reconciler

	// Configure configures the reconciler
	Configure(ctx Context) error
}

// Defaulting defines interface for the kubernetes object that provides default spec and status
type Defaulting interface {
	runtime.Object
	metav1.Object

	// SetSpecDefaults set the default of the object spec and returns true if any set otherwise false
	SetSpecDefaults() bool

	// SetStatusDefaults set the default of the object status and returns true if any set otherwise false
	SetStatusDefaults() bool
}

// KubeRuntimeObject defines interface of the kubernetes object to reconcile
type KubeRuntimeObject interface {
	runtime.Object
	metav1.Object
}

// Context represents a context of the Reconciler
type Context interface {

	// NewControllerBuilder returns a new builder to create a controllers
	NewControllerBuilder() *builder.Builder

	// Client returns the underlying client
	Client() client.Client

	// Scheme returns the underlying scheme
	Scheme() *runtime.Scheme

	// Logger returns the underlying logger
	Logger() logr.Logger

	// Run checks if the reconciliation can be done and call the reconcile function to do so
	Run(req reconcile.Request, runtimeObject KubeRuntimeObject, reconcile func(deleted bool) error) (reconcile.Result, error)

	// SetOwnershipReference set ownership of the controlled object to the owner
	SetOwnershipReference(owner metav1.Object, controlled metav1.Object) error

	// GetResource is a helper to method to get a resource and do something about its availability
	GetResource(key client.ObjectKey, object client.Object, foundCallback func() (err error), notFoundCallback func() (err error)) error
}
