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

package controller_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	diemetav1 "reconciler.io/dies/apis/meta/v1"
	"reconciler.io/runtime/reconcilers"
	rtesting "reconciler.io/runtime/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ducksv1 "reconciler.io/ducks/api/v1"
	"reconciler.io/ducks/internal/controller"
)

func TestClusterDuckReconciler(t *testing.T) {
	namespace := "test-namespace"
	name := "my-duck"
	request := reconcilers.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(ducksv1.AddToScheme(scheme))

	now := metav1.Now().Rfc3339Copy()

	given := ducksv1.ClusterDuckBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
			d.Generation(1)
		}).
		StatusDie(func(d *ducksv1.ClusterDuckStatusDie) {
			d.InitializeConditionsDie(now.Time)
			d.ObservedGeneration(1)
		})

	rts := rtesting.ReconcilerTests{
		"in sync": {
			Request: request,
			StatusSubResourceTypes: []client.Object{
				&ducksv1.ClusterDuck{},
			},
			GivenObjects: []client.Object{
				given,
			},
			Now: now.Time,
		},
	}

	rts.Run(t, scheme, func(t *testing.T, tc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		return controller.ClusterDuckReconciler(c)
	})
}
