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
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	toolscache "k8s.io/client-go/tools/cache"
	"reconciler.io/runtime/apis"
	"reconciler.io/runtime/reconcilers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	duckv1 "reconciler.io/ducks/api/v1"
)

type Broker interface {
	Subscribe(ctx context.Context) <-chan event.GenericEvent
	TrackedSource(ctx context.Context) source.Source
}

// NewBroker starts informers for all resources of a given duck type
func NewBroker(mgr manager.Manager, duck schema.GroupKind) (Broker, error) {
	broker := &broker{
		name:      fmt.Sprintf("%sBroker", duck.Kind),
		publishCh: make(chan event.GenericEvent, 1),
		subCh:     make(chan chan event.GenericEvent, 1),
		unsubCh:   make(chan chan event.GenericEvent, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	go broker.Start(ctx)

	mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		log := ctrl.Log.WithName(broker.name)
		ctx = logr.NewContext(ctx, log)

		log.Info("Starting")

		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(duck.WithVersion("v1"))
		duckTypeInformer, err := mgr.GetCache().GetInformer(ctx, u, cache.BlockUntilSynced(false))
		if err != nil {
			return err
		}

		var m sync.Mutex
		duckInformers := map[string]cache.Informer{}
		duckRegistrations := map[string]toolscache.ResourceEventHandlerRegistration{}

		var informOn = func(r *duckv1.Duck) {
			log := log.WithValues("duck", r.Name)

			m.Lock()
			defer m.Unlock()

			if _, ok := duckInformers[r.Name]; ok {
				// already informing
				return
			}
			if ready := r.GetConditionManager(ctx).GetCondition(duckv1.DuckConditionReady); !apis.ConditionIsTrue(ready) {
				// not ready
				return
			}

			log.Info("Starting duck informer")

			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(r.Spec.GroupVersionKind())
			duckInformer, err := mgr.GetCache().GetInformer(ctx, u, cache.BlockUntilSynced(false))
			if err != nil {
				log.Error(err, "Unable to start duck informer")
				return
			}
			duckRegistration, err := duckInformer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					broker.Publish(event.GenericEvent{Object: obj.(client.Object)})
				},
				UpdateFunc: func(oldObj, newObj interface{}) {
					broker.Publish(event.GenericEvent{Object: newObj.(client.Object)})
				},
				DeleteFunc: func(obj interface{}) {
					broker.Publish(event.GenericEvent{Object: obj.(client.Object)})
				},
			})
			if err != nil {
				log.Error(err, "Unable to handle duck events")
				return
			}
			duckInformers[r.Name] = duckInformer
			duckRegistrations[r.Name] = duckRegistration
		}

		duckTypeRegistration, err := duckTypeInformer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				u := obj.(*unstructured.Unstructured)
				r := &duckv1.Duck{}
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, r); err != nil {
					panic(err)
				}
				informOn(r)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				u := newObj.(*unstructured.Unstructured)
				r := &duckv1.Duck{}
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, r); err != nil {
					panic(err)
				}
				informOn(r)
			},
			DeleteFunc: func(obj interface{}) {
				name := obj.(*unstructured.Unstructured).GetName()
				log := log.WithValues("duck", name)

				log.Info("Stopping duck informer")

				m.Lock()
				defer m.Unlock()

				if registration, ok := duckRegistrations[name]; ok {
					if err := duckInformers[name].RemoveEventHandler(registration); err != nil {
						log.Error(err, "Unable to stop duck informer")
						return
					}
					delete(duckInformers, name)
					delete(duckRegistrations, name)
				}
			},
		})
		if err != nil {
			return err
		}

		log.Info("Started")
		<-ctx.Done()

		m.Lock()
		defer m.Unlock()

		cancel()

		// remove handlers
		if err := duckTypeInformer.RemoveEventHandler(duckTypeRegistration); err != nil {
			return err
		}
		for name, registration := range duckRegistrations {
			if err := duckInformers[name].RemoveEventHandler(registration); err != nil {
				return err
			}
		}

		return nil
	}))

	return broker, nil
}

// adapted from https://stackoverflow.com/a/49877632
type broker struct {
	name      string
	publishCh chan event.GenericEvent
	subCh     chan chan event.GenericEvent
	unsubCh   chan chan event.GenericEvent
}

func (b *broker) Start(ctx context.Context) error {
	subs := map[chan event.GenericEvent]struct{}{}
	for {
		select {
		case <-ctx.Done():
			return nil
		case msgCh := <-b.subCh:
			subs[msgCh] = struct{}{}
		case msgCh := <-b.unsubCh:
			delete(subs, msgCh)
		case msg := <-b.publishCh:
			for msgCh := range subs {
				// msgCh is buffered, use non-blocking send to protect the broker:
				select {
				case msgCh <- msg:
				default:
				}
			}
		}
	}
}

func (b *broker) Publish(msg event.GenericEvent) {
	b.publishCh <- msg
}

func (b *broker) Subscribe(ctx context.Context) <-chan event.GenericEvent {
	logr.FromContextOrDiscard(ctx).WithName(b.name).Info("Subscribe")

	msgCh := make(chan event.GenericEvent, 5)
	b.subCh <- msgCh
	go func() {
		<-ctx.Done()
		b.unsubCh <- msgCh
	}()
	return msgCh
}

func (b *broker) TrackedSource(ctx context.Context) source.Source {
	return source.Channel(b.Subscribe(ctx), reconcilers.EnqueueTracked(ctx))
}
