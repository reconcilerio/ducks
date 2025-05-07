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

package reconcilers

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"reconciler.io/runtime/reconcilers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"reconciler.io/ducks/internal/submanager"
)

var _ reconcilers.SubReconciler[client.Object] = (*SubManagerReconciler[client.Object])(nil)

type SubManagerReconciler[Type client.Object] struct {
	// Name used to identify this reconciler.  Defaults to `SubManagerReconciler`.  Ideally unique, but
	// not required to be so.
	//
	// +optional
	Name string

	// Setup performs initialization on the manager and builder this reconciler
	// will run with. It's common to setup field indexes and watch resources.
	//
	// +optional
	Setup func(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error

	SyncPeriod      *time.Duration
	AssertFinalizer string

	LocalTypes          func(ctx context.Context, resource Type) ([]schema.GroupKind, error)
	SetupWithSubManager func(ctx context.Context, mgr ctrl.Manager, resource Type) error

	initOnce sync.Once
	mgr      ctrl.Manager
	managers map[types.UID]subManagerEntry
}

type subManagerEntry struct {
	done   <-chan error
	cancel context.CancelFunc
}

func (r *SubManagerReconciler[T]) SetupWithManager(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
	if r.mgr == nil {
		r.mgr = mgr
	}
	r.init()

	log := logr.FromContextOrDiscard(ctx).
		WithName(r.Name)
	ctx = logr.NewContext(ctx, log)

	if err := r.Validate(ctx); err != nil {
		return err
	}
	if r.Setup == nil {
		return nil
	}
	return r.Setup(ctx, mgr, bldr)
}

func (r *SubManagerReconciler[T]) init() {
	r.initOnce.Do(func() {
		if r.Name == "" {
			r.Name = "SubManagerReconciler"
		}
		if r.SyncPeriod == nil {
			r.SyncPeriod = ptr.To(10 * time.Hour)
		}
		if r.mgr == nil {
			// TODO default mgr
			panic("SubManagerReconciler: SetupWithManager must be called before Reconcile")
		}

		r.managers = map[types.UID]subManagerEntry{}
	})
}

func (r *SubManagerReconciler[T]) Validate(ctx context.Context) error {
	r.init()

	// require AssertFinalizer
	if r.AssertFinalizer == "" {
		return fmt.Errorf("SubManagerReconciler %q must provide AssertFinalizer", r.Name)
	}

	// require LocalTypes
	if r.LocalTypes == nil {
		return fmt.Errorf("SubManagerReconciler %q must implement LocalTypes", r.Name)
	}

	// require SetupWithSubManager
	if r.SetupWithSubManager == nil {
		return fmt.Errorf("SubManagerReconciler %q must implement SetupWithSubManager", r.Name)
	}

	return nil
}

func (r *SubManagerReconciler[T]) Reconcile(ctx context.Context, resource T) (reconcilers.Result, error) {
	r.init()

	if resource.GetDeletionTimestamp() != nil {
		return r.shutdown(ctx, resource)
	}

	if !slices.Contains(resource.GetFinalizers(), r.AssertFinalizer) {
		return reconcilers.Result{}, fmt.Errorf("resource must contain finalizer %q", r.AssertFinalizer)
	}

	if _, ok := r.managers[resource.GetUID()]; ok {
		// already running
		return reconcilers.Result{}, nil
	}

	return r.start(ctx, resource)
}

func (r *SubManagerReconciler[T]) start(ctx context.Context, resource T) (reconcilers.Result, error) {
	localTypes, err := r.LocalTypes(ctx, resource)
	if err != nil {
		return reconcile.Result{}, nil
	}

	mgr, err := submanager.New(r.mgr,
		manager.Options{
			Cache: cache.Options{
				SyncPeriod:                  r.SyncPeriod,
				ReaderFailOnMissingInformer: true,
			},
		},
		localTypes...,
	)
	if err != nil {
		return reconcile.Result{}, err
	}

	ctx, cancel := context.WithCancel(ctx)

	if err := r.SetupWithSubManager(ctx, mgr, resource); err != nil {
		cancel()
		return reconcile.Result{}, err
	}

	done := make(chan error)
	r.managers[resource.GetUID()] = subManagerEntry{done, cancel}
	go func(ctx context.Context, mgr ctrl.Manager, done chan<- error) {
		err := mgr.Start(ctx)
		done <- err
	}(ctx, mgr, done)

	return reconcilers.Result{}, nil
}

func (r *SubManagerReconciler[T]) shutdown(ctx context.Context, resource T) (reconcilers.Result, error) {
	manager, ok := r.managers[resource.GetUID()]
	if ok {
		manager.cancel()
		// block until shutdown is complete
		if err := <-manager.done; err != nil {
			logr.FromContextOrDiscard(ctx).Error(err, "problem running submanager")
		}
		delete(r.managers, resource.GetUID())
	}

	return reconcile.Result{}, nil
}
