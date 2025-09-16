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

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"reconciler.io/runtime/apis"
	"reconciler.io/runtime/duck"
	"reconciler.io/runtime/reconcilers"
	"sigs.k8s.io/controller-runtime/pkg/client"

	duckv1 "reconciler.io/ducks/api/v1"
)

type Client interface {
	client.Client
	TrackAndGet(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
	TrackAndList(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
}

func New(duckType string, client reconcilers.Config) Client {
	return &duckClient{
		client: client,
		duckType: &duckv1.DuckType{
			ObjectMeta: metav1.ObjectMeta{
				Name: duckType,
			},
		},
	}
}

var (
	ErrUnknownDuckType  = errors.New("unknown duck type")
	ErrUnknownDuck      = errors.New("unknown duck")
	ErrDuckTypeNotReady = errors.New("duck type is not ready")
)

type duckClient struct {
	client   Client
	duckType *duckv1.DuckType
}

func (c *duckClient) ducks(ctx context.Context, duckGK schema.GroupKind, track bool) ([]duckv1.Duck, error) {
	duckType := c.duckType.DeepCopy()
	if err := c.client.Get(ctx, client.ObjectKeyFromObject(duckType), duckType); err != nil {
		if apierrs.IsNotFound(err) {
			return nil, ErrUnknownDuckType
		}
		return nil, err
	}
	if err := duckType.Default(ctx, duckType); err != nil {
		return nil, err
	}
	if ready := duckType.GetConditionManager(ctx).GetCondition(duckv1.DuckTypeConditionReady); !apis.ConditionIsTrue(ready) {
		return nil, ErrDuckTypeNotReady
	}

	duckList := &duckv1.DuckList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: schema.GroupVersion{Group: duckType.Spec.Group, Version: "v1"}.String(),
			Kind:       duckType.Spec.ListKind,
		},
	}
	if track {
		if err := c.client.TrackAndList(ctx, duckList); err != nil {
			return nil, err
		}
	} else {
		if err := c.client.List(ctx, duckList); err != nil {
			return nil, err
		}
	}

	ducks := []duckv1.Duck{}
	for _, duck := range duckList.Items {
		if ready := duck.GetConditionManager(ctx).GetCondition(duckv1.DuckConditionReady); !apis.ConditionIsTrue(ready) {
			// ignore ducks that are not ready, most likely the API does not exist
			continue
		}
		if duckGK.Empty() {
			ducks = append(ducks, duck)
		} else if duckGK.Group == duck.Spec.Group && duckGK.Kind == duck.Spec.Kind {
			ducks = append(ducks, duck)
		} else if duckGK.Group == duck.Spec.Group && duckGK.Kind == fmt.Sprintf("%sList", duck.Spec.Kind) {
			ducks = append(ducks, duck)
		}
	}

	if !duckGK.Empty() && len(ducks) == 0 {
		return nil, ErrUnknownDuck
	}

	return ducks, nil
}

func (c *duckClient) duck(ctx context.Context, duckGK schema.GroupKind, track bool) (*duckv1.Duck, error) {
	ducks, err := c.ducks(ctx, duckGK, track)
	if err != nil {
		return nil, err
	}
	if len(ducks) != 1 {
		return nil, ErrUnknownDuck
	}
	return &ducks[0], nil
}

func (c *duckClient) setDuckVersion(ctx context.Context, obj client.Object, track bool) error {
	gvk := obj.GetObjectKind().GroupVersionKind()
	duck, err := c.duck(ctx, gvk.GroupKind(), track)
	if err != nil {
		return err
	}
	gvk.Version = duck.Spec.Version
	obj.GetObjectKind().SetGroupVersionKind(gvk)

	return nil
}

func (c *duckClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if err := c.setDuckVersion(ctx, obj, false); err != nil {
		return err
	}
	return c.client.Get(ctx, key, obj, opts...)
}

func (c *duckClient) TrackAndGet(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if err := c.setDuckVersion(ctx, obj, true); err != nil {
		return err
	}
	return c.client.TrackAndGet(ctx, key, obj, opts...)
}

func (c *duckClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	ducks, err := c.ducks(ctx, list.GetObjectKind().GroupVersionKind().GroupKind(), false)
	if err != nil {
		return err
	}
	aggregateList := &unstructured.UnstructuredList{}
	for _, duck := range ducks {
		duckList := &unstructured.UnstructuredList{}
		duckList.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Group:   duck.Spec.Group,
			Version: duck.Spec.Version,
			Kind:    fmt.Sprintf("%sList", duck.Spec.Kind),
		})
		if err := c.client.List(ctx, duckList, opts...); err != nil {
			return err
		}
		aggregateList.Items = append(aggregateList.Items, duckList.Items...)
	}
	return duck.Convert(aggregateList, list)
}

func (c *duckClient) TrackAndList(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	ducks, err := c.ducks(ctx, list.GetObjectKind().GroupVersionKind().GroupKind(), true)
	if err != nil {
		return err
	}
	aggregate := &unstructured.UnstructuredList{}
	for _, duck := range ducks {
		duckList := &unstructured.UnstructuredList{}
		duckList.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Group:   duck.Spec.Group,
			Version: duck.Spec.Version,
			Kind:    fmt.Sprintf("%sList", duck.Spec.Kind),
		})
		if err := c.client.TrackAndList(ctx, duckList, opts...); err != nil {
			return err
		}
		aggregate.Items = append(aggregate.Items, duckList.Items...)
	}
	return duck.Convert(aggregate, list)
}

func (c *duckClient) Watch(ctx context.Context, list client.ObjectList, opts ...client.ListOption) (watch.Interface, error) {
	panic("Watch is not implemented for duck types")
}

func (c *duckClient) Apply(ctx context.Context, obj runtime.ApplyConfiguration, opts ...client.ApplyOption) error {
	return c.client.Apply(ctx, obj, opts...)
}

func (c *duckClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return c.client.Create(ctx, obj, opts...)
}

func (c *duckClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return c.client.Delete(ctx, obj, opts...)
}

func (c *duckClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return c.client.Update(ctx, obj, opts...)
}

func (c *duckClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return c.client.Patch(ctx, obj, patch, opts...)
}

func (c *duckClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	ducks, err := c.ducks(ctx, obj.GetObjectKind().GroupVersionKind().GroupKind(), false)
	if err != nil {
		return err
	}
	for _, duck := range ducks {
		obj := obj.DeepCopyObject().(client.Object)
		obj.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Group:   duck.Spec.Group,
			Version: duck.Spec.Version,
			Kind:    duck.Spec.Kind,
		})
		if err := c.client.DeleteAllOf(ctx, obj, opts...); err != nil {
			return err
		}
	}
	return nil
}

func (c *duckClient) Status() client.SubResourceWriter {
	panic("Status sub resource client is not implemented for duck types")
}

func (c *duckClient) SubResource(subResource string) client.SubResourceClient {
	panic("SubResource client is not implemented for duck types")
}

func (c *duckClient) Scheme() *runtime.Scheme {
	return c.client.Scheme()
}

func (c *duckClient) RESTMapper() meta.RESTMapper {
	return c.client.RESTMapper()
}

func (c *duckClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	obj = obj.DeepCopyObject()
	if err := c.setDuckVersion(context.TODO(), obj.(client.Object), false); err != nil {
		return schema.GroupVersionKind{}, err
	}
	return obj.GetObjectKind().GroupVersionKind(), nil
}

func (c *duckClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return c.client.IsObjectNamespaced(obj)
}
