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

package submanager

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"reconciler.io/runtime/reconcilers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// New returns a sub-manager whose cache operates separately for the specified types.
func New(mgr manager.Manager, opts manager.Options, localTypes ...schema.GroupKind) (manager.Manager, error) {
	opts.NewCache = func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
		local, err := cache.New(config, opts)
		if err != nil {
			return nil, err
		}
		return &Cache{
			client:     reconcilers.NewConfig(mgr, nil, 0).Client,
			localTypes: sets.New(localTypes...),
			local:      local,
			upstream:   mgr.GetCache(),
		}, nil
	}
	if opts.Scheme == nil {
		opts.Scheme = mgr.GetScheme()
	}
	if opts.Controller.SkipNameValidation == nil {
		opts.Controller.SkipNameValidation = ptr.To(true)
	}
	if opts.Metrics.BindAddress == "" {
		opts.Metrics.BindAddress = "0"
	}
	opts.LeaderElection = false

	return ctrl.NewManager(mgr.GetConfig(), opts)
}

var _ cache.Cache = (*Cache)(nil)

type Cache struct {
	client     client.Client
	localTypes sets.Set[schema.GroupKind]
	local      cache.Cache
	upstream   cache.Cache
}

func (c *Cache) isLocal(ctx context.Context, obj runtime.Object) bool {
	gvk, err := c.client.GroupVersionKindFor(obj)
	if err != nil {
		panic(err)
	}
	return c.isLocalKind(gvk)
}
func (c *Cache) isLocalKind(gvk schema.GroupVersionKind) bool {
	return c.localTypes.Has(gvk.GroupKind())
}

func (c *Cache) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if c.isLocal(ctx, obj) {
		return c.local.Get(ctx, key, obj, opts...)
	}
	return c.upstream.Get(ctx, key, obj, opts...)
}

func (c *Cache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if c.isLocal(ctx, list) {
		return c.local.List(ctx, list, opts...)
	}
	return c.upstream.List(ctx, list, opts...)
}

func (c *Cache) GetInformer(ctx context.Context, obj client.Object, opts ...cache.InformerGetOption) (cache.Informer, error) {
	if c.isLocal(ctx, obj) {
		return c.local.GetInformer(ctx, obj, opts...)
	}
	return c.upstream.GetInformer(ctx, obj, opts...)
}

func (c *Cache) GetInformerForKind(ctx context.Context, gvk schema.GroupVersionKind, opts ...cache.InformerGetOption) (cache.Informer, error) {
	if c.isLocalKind(gvk) {
		return c.local.GetInformerForKind(ctx, gvk, opts...)
	}
	return c.upstream.GetInformerForKind(ctx, gvk, opts...)
}

func (c *Cache) RemoveInformer(ctx context.Context, obj client.Object) error {
	if c.isLocal(ctx, obj) {
		return c.local.RemoveInformer(ctx, obj)
	}
	return c.upstream.RemoveInformer(ctx, obj)
}

func (c *Cache) Start(ctx context.Context) error {
	return c.local.Start(ctx)
}

func (c *Cache) WaitForCacheSync(ctx context.Context) bool {
	return c.upstream.WaitForCacheSync(ctx) && c.local.WaitForCacheSync(ctx)
}

func (c *Cache) IndexField(ctx context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error {
	if c.isLocal(ctx, obj) {
		return c.local.IndexField(ctx, obj, field, extractValue)
	}
	return c.upstream.IndexField(ctx, obj, field, extractValue)
}
