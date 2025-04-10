/*
Copyright 2025 the original author or authors.

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

package client

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"reconciler.io/runtime/apis"
	"reconciler.io/runtime/duck"
	"reconciler.io/runtime/reconcilers"
	"sigs.k8s.io/controller-runtime/pkg/client"

	duckv1 "reconciler.io/ducks/api/v1"
)

var ErrNotDuck = errors.New("referenced apiVersion kind is not a duck")

func Get(ctx context.Context, key types.NamespacedName, obj client.Object, duckMeta schema.GroupKind, opts ...client.GetOption) error {
	track := false
	return get(ctx, key, obj, duckMeta, opts, track)
}

func TrackAndGet(ctx context.Context, key types.NamespacedName, obj client.Object, duckMeta schema.GroupKind, opts ...client.GetOption) error {
	track := true
	return get(ctx, key, obj, duckMeta, opts, track)
}

func get(ctx context.Context, key types.NamespacedName, obj client.Object, duckMeta schema.GroupKind, opts []client.GetOption, track bool) error {
	c := reconcilers.RetrieveConfigOrDie(ctx)

	if !duck.IsDuck(obj, c.Scheme()) {
		return ErrNotDuck
	}
	objGVK := obj.GetObjectKind().GroupVersionKind()

	duckObjs := &duckv1.DuckList{
		TypeMeta: metav1.TypeMeta{
			// ducks are always v1
			APIVersion: duckMeta.WithVersion("v1").GroupVersion().String(),
			Kind:       fmt.Sprintf("%sList", duckMeta.Kind),
		},
	}
	if err := c.List(ctx, duckObjs, client.MatchingFields{"spec.group": objGVK.Group, "spec.kind": objGVK.Kind}); err != nil {
		return err
	}
	if len(duckObjs.Items) != 1 {
		return ErrNotDuck
	}
	duckObj := &duckObjs.Items[0]

	if ready := duckObj.GetConditionManager(ctx).GetCondition(duckv1.DuckConditionReady); !apis.ConditionIsTrue(ready) {
		return ErrNotDuck
	}

	// normalize fetched version
	objGVK.Version = duckObj.Spec.Version
	obj.GetObjectKind().SetGroupVersionKind(objGVK)

	if track {
		if err := c.TrackAndGet(ctx, key, obj, opts...); err != nil {
			return err
		}
	} else {
		if err := c.Get(ctx, key, obj, opts...); err != nil {
			return err
		}
	}

	return nil
}
