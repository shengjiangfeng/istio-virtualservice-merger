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

package controllers

import (
	"context"
	"fmt"
	"sigs.k8s.io/yaml"

	"github.com/monimesl/istio-virtualservice-merger/api/v1alpha1"
	"github.com/monimesl/operator-helper/oputil"
	"github.com/monimesl/operator-helper/reconciler"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	finalizerName = "istiomerger.monime.sl-finalizer"
)

func Reconcile(ctx reconciler.Context, client versionedclient.Interface, patch *v1alpha1.VirtualServiceMerge, oldpatchref interface{}) error {
	if oldpatchref != nil {
		oldpatch := oldpatchref.(*v1alpha1.VirtualServiceMerge)
		// check if target is different
		oldTargetName, oldTargetNamespace := oldpatch.Spec.Target.Name, oldpatch.Spec.Target.Namespace
		newTargetName, newTargetNamespace := patch.Spec.Target.Name, patch.Spec.Target.Namespace
		if oldTargetNamespace == "" {
			oldTargetNamespace = oldpatch.Namespace
		}
		if newTargetNamespace == "" {
			newTargetNamespace = patch.Namespace
		}
		if oldTargetName != newTargetName || oldTargetNamespace != newTargetNamespace {
			// remove from this object
			ctx.Logger().Info("Virtual service target changed. Removing patch from old target", "virtualservice", oldTargetNamespace+"/"+oldTargetName)
			if err := updateTarget(ctx, client, oldpatch, true); err != nil {
				if kerr.IsNotFound(err) {
					// ignore if virtualservice is not found
					ctx.Logger().Info("Virtual service not found. Nothing to sync.")
				} else {
					return err
				}
			}
		}
	}

	if patch.DeletionTimestamp.IsZero() {
		if !oputil.ContainsWithPrefix(patch.Finalizers, finalizerName) {
			ctx.Logger().Info("Adding the finalizer to the patch",
				"patch", patch.Name, "finalizer", finalizerName)
			patch.Finalizers = append(patch.Finalizers, finalizerName)
			return ctx.Client().Update(context.TODO(), patch)
		}
	} else if oputil.Contains(patch.Finalizers, finalizerName) {
		if err := updateTarget(ctx, client, patch, true); err != nil {
			if kerr.IsNotFound(err) {
				// ignore if virtualservice is not found
				ctx.Logger().Info("Virtual service not found. Nothing to sync.")
			} else {
				return err
			}
		}
		patch.Finalizers = oputil.Remove(finalizerName, patch.Finalizers)
		if err := ctx.Client().Update(context.TODO(), patch); err != nil {
			return fmt.Errorf("VirtualServiceMerge object (%s) update error: %w", patch.Name, err)
		}
		return nil
	}
	if patch.ResourceVersion != patch.Status.HandledRevision {
		if err := updateTarget(ctx, client, patch, false); err != nil {
			if kerr.IsNotFound(err) {
				// ignore if virtualservice is not found
				ctx.Logger().Info("Virtual service not found. Nothing to sync.")
			} else {
				return err
			}
		}
		patch.Status.HandledRevision = patch.ResourceVersion
		if err := ctx.Client().Status().Update(context.TODO(), patch); err != nil {
			return fmt.Errorf("VirtualServiceMerge object (%s) status update error: %w", patch.Name, err)
		}
		return nil
	}
	return nil
}

func updateTarget(ctx reconciler.Context, client versionedclient.Interface, patch *v1alpha1.VirtualServiceMerge, remove bool) error {
	if err := patch.Spec.Target.Validate(); err != nil {
		return fmt.Errorf("virtualservicepatch.Reconcile: %w", err)
	}
	targetName, targetNamespace := patch.Spec.Target.Name, patch.Spec.Target.Namespace
	if targetNamespace == "" {
		targetNamespace = patch.Namespace
	}
	target, err := client.NetworkingV1alpha3().VirtualServices(targetNamespace).
		Get(context.TODO(), targetName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(target)
	fmt.Println("orgin target ")
	fmt.Println(string(data))
	if remove {
		patch.RemoveTcpRoutes(target)
		patch.RemoveTlsRoutes(target)
		patch.RemoveHttpRoutes(ctx, target)
	} else {
		patch.AddTcpRoutes(target)
		patch.AddTlsRoutes(target)
		patch.AddHttpRoutes(ctx, target)
	}
	data1, err := yaml.Marshal(target)
	fmt.Println("patched target ")
	fmt.Println(string(data1))
	if _, err = client.NetworkingV1alpha3().VirtualServices(targetNamespace).
		Update(context.TODO(), target, metav1.UpdateOptions{}); err != nil {
		return err
	}
	return nil
}
