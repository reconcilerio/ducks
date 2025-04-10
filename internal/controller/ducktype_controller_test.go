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
	"fmt"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	dieapiextensionsv1 "reconciler.io/dies/apis/apiextensions/v1"
	dierbacv1 "reconciler.io/dies/apis/authorization/rbac/v1"
	diemetav1 "reconciler.io/dies/apis/meta/v1"
	"reconciler.io/runtime/reconcilers"
	rtesting "reconciler.io/runtime/testing"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ducksv1 "reconciler.io/ducks/api/v1"
	"reconciler.io/ducks/internal/controller"
)

func TestDuckTypeReconciler(t *testing.T) {
	name := "ducks.example.com"
	request := reconcilers.Request{NamespacedName: types.NamespacedName{Name: name}}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	utilruntime.Must(apiregistrationv1.AddToScheme(scheme))
	utilruntime.Must(ducksv1.AddToScheme(scheme))

	now := metav1.Now().Rfc3339Copy()

	given := ducksv1.DuckTypeBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Name(name)
			d.CreationTimestamp(now)
			d.Generation(1)
			d.Finalizers(ducksv1.GroupVersion.Group)
		}).
		SpecDie(func(d *ducksv1.DuckTypeSpecDie) {
			d.Group("example.com")
			d.Plural("ducks")
			d.Kind("Duck")
		}).
		StatusDie(func(d *ducksv1.DuckTypeStatusDie) {
			d.InitializeConditionsDie(now.Time)
			d.ConditionDie(ducksv1.DuckTypeConditionCustomResourceDefinitionEstablished, func(d *diemetav1.ConditionDie) {
				d.True()
				d.Reason("Established")
			})
			d.ConditionDie(ducksv1.DuckTypeConditionRBAC, func(d *diemetav1.ConditionDie) {
				d.True()
				d.Reason("Defined")
			})
			d.ConditionDie(ducksv1.DuckTypeConditionReady, func(d *diemetav1.ConditionDie) {
				d.True()
				d.Reason("Ready")
			})
			d.ObservedGeneration(1)
		})

	crdGiven := dieapiextensionsv1.CustomResourceDefinitionBlank.
		DieFeedYAMLFile("testdata/duck_crd.yaml").
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Name(name)
			d.CreationTimestamp(now)
			d.Generation(1)
			d.ControlledBy(given, scheme)
		}).
		SpecDie(func(d *dieapiextensionsv1.CustomResourceDefinitionSpecDie) {
			d.Group("example.com")
			d.NamesDie(func(d *dieapiextensionsv1.CustomResourceDefinitionNamesDie) {
				d.Plural("ducks")
				d.Singular("duck")
				d.Kind("Duck")
				d.ListKind("DuckList")
				d.Categories("duck")
			})
		}).
		StatusDie(func(d *dieapiextensionsv1.CustomResourceDefinitionStatusDie) {
			d.AcceptedNamesDie(func(d *dieapiextensionsv1.CustomResourceDefinitionNamesDie) {
				d.Categories("duck")
				d.Kind("Duck")
				d.ListKind("DuckList")
				d.Plural("ducks")
				d.Singular("duck")
			})
			d.StoredVersions("v1")
			d.Conditions(
				apiextensionsv1.CustomResourceDefinitionCondition{
					Type:    "NamesAccepted",
					Status:  apiextensionsv1.ConditionTrue,
					Reason:  "NoConflicts",
					Message: "no conflicts found",
				},
				apiextensionsv1.CustomResourceDefinitionCondition{
					Type:    "Established",
					Status:  apiextensionsv1.ConditionTrue,
					Reason:  "InitialNamesAccepted",
					Message: "the initial names have been accepted",
				},
			)
		})

	viewClusterRoleGiven := dierbacv1.ClusterRoleBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Name(fmt.Sprintf("reconcilerio-ducks-%s-view", name))
			d.CreationTimestamp(now)
			d.Generation(1)
			d.ControlledBy(given, scheme)
			d.AddLabel("ducks.reconciler.io/type", "ducks.example.com")
		}).
		AggregationRuleDie(func(d *dierbacv1.AggregationRuleDie) {
			d.ClusterRoleSelectorsDie(
				diemetav1.LabelSelectorBlank.
					AddMatchLabel("ducks.reconciler.io/type", "ducks.example.com").
					AddMatchLabel("ducks.reconciler.io/role", "view"),
			)
		})

	editClusterRoleGiven := dierbacv1.ClusterRoleBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Name(fmt.Sprintf("reconcilerio-ducks-%s-edit", name))
			d.CreationTimestamp(now)
			d.Generation(1)
			d.ControlledBy(given, scheme)
			d.AddLabel("ducks.reconciler.io/type", "ducks.example.com")
		}).
		AggregationRuleDie(func(d *dierbacv1.AggregationRuleDie) {
			d.ClusterRoleSelectorsDie(
				diemetav1.LabelSelectorBlank.
					AddMatchLabel("ducks.reconciler.io/type", "ducks.example.com").
					AddMatchLabel("ducks.reconciler.io/role", "edit"),
			)
		})

	rts := rtesting.ReconcilerTests{
		"in sync": {
			Request: request,
			StatusSubResourceTypes: []client.Object{
				&ducksv1.DuckType{},
			},
			GivenObjects: []client.Object{
				given,
				crdGiven,
				viewClusterRoleGiven,
				editClusterRoleGiven,
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, tc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		mgr, err := ctrl.NewManager(&rest.Config{}, manager.Options{
			Scheme: scheme,
			NewClient: func(config *rest.Config, options client.Options) (client.Client, error) {
				return c.Client, nil
			},
		})
		if err != nil {
			t.Fatalf("failed to create manager: %s", err)
		}
		return controller.DuckTypeReconciler(c, mgr)
	})
}

func TestDuckReconciler(t *testing.T) {
	name := "duckinstances.example.com"
	request := reconcilers.Request{NamespacedName: types.NamespacedName{Name: name}}
	duckMeta := metav1.TypeMeta{
		APIVersion: "example.com/v1",
		Kind:       "Duck",
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	utilruntime.Must(apiregistrationv1.AddToScheme(scheme))
	utilruntime.Must(ducksv1.AddToScheme(scheme))

	givenAPIResources := []*metav1.APIResourceList{
		{
			TypeMeta:     duckMeta,
			GroupVersion: duckMeta.APIVersion,
			APIResources: []metav1.APIResource{
				{
					Name:         "ducks",
					SingularName: "duck",
					Namespaced:   false,
					Group:        "example.com",
					Version:      "v1",
					Kind:         "Duck",
				},
				{
					Name:         "duckinstances",
					SingularName: "duckinstance",
					Namespaced:   true,
					Group:        "example.com",
					Version:      "v1",
					Kind:         "DuckInstance",
				},
			},
		},
	}

	now := metav1.Now().Rfc3339Copy()

	given := ducksv1.DuckBlank.
		TypeMetadata(duckMeta).
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Name(name)
			d.CreationTimestamp(now)
			d.Generation(1)
			d.Finalizers(ducksv1.GroupVersion.Group)
		}).
		SpecDie(func(d *ducksv1.DuckSpecDie) {
			d.Group("example.com")
			d.Version("v1")
			d.Kind("DuckInstance")
		}).
		StatusDie(func(d *ducksv1.DuckStatusDie) {
			d.InitializeConditionsDie(now.Time)
			d.ConditionDie(ducksv1.DuckConditionAvailable, func(d *diemetav1.ConditionDie) {
				d.True()
				d.Reason("Available")
			})
			d.ConditionDie(ducksv1.DuckConditionRBAC, func(d *diemetav1.ConditionDie) {
				d.True()
				d.Reason("Defined")
			})
			d.ConditionDie(ducksv1.DuckConditionReady, func(d *diemetav1.ConditionDie) {
				d.True()
				d.Reason("Ready")
			})
			d.ObservedGeneration(1)
		})

	viewClusterRoleGiven := dierbacv1.ClusterRoleBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Name(fmt.Sprintf("reconcilerio-ducks-%s-%s-view", "ducks.example.com", name))
			d.CreationTimestamp(now)
			d.Generation(1)
			d.ControlledBy(given, scheme)
			d.AddLabel("ducks.reconciler.io/type", "ducks.example.com")
			d.AddLabel("ducks.reconciler.io/role", "view")
		}).
		RulesDie(
			dierbacv1.PolicyRuleBlank.
				AddAPIGroups("example.com").
				AddAResources("duckinstances").
				AddVerbs("get", "list", "watch"),
		)

	editClusterRoleGiven := dierbacv1.ClusterRoleBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Name(fmt.Sprintf("reconcilerio-ducks-%s-%s-edit", "ducks.example.com", name))
			d.CreationTimestamp(now)
			d.Generation(1)
			d.ControlledBy(given, scheme)
			d.AddLabel("ducks.reconciler.io/type", "ducks.example.com")
			d.AddLabel("ducks.reconciler.io/role", "edit")
		}).
		RulesDie(
			dierbacv1.PolicyRuleBlank.
				AddAPIGroups("example.com").
				AddAResources("duckinstances").
				AddVerbs("get", "list", "watch", "patch"),
		)

	rts := rtesting.ReconcilerTests{
		"in sync": {
			Request: request,
			StatusSubResourceTypes: []client.Object{
				&ducksv1.Duck{
					TypeMeta: duckMeta,
				},
			},
			GivenAPIResources: givenAPIResources,
			GivenObjects: []client.Object{
				given,
				viewClusterRoleGiven,
				editClusterRoleGiven,
			},
			ExpectTracks: []rtesting.TrackRequest{
				rtesting.NewTrackRequest(&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "duckinstances.example.com"}}, given, scheme),
				rtesting.NewTrackRequest(&apiregistrationv1.APIService{ObjectMeta: metav1.ObjectMeta{Name: "v1.example.com"}}, given, scheme),
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, tc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		return controller.DuckReconciler(c, duckMeta)
	})
}
