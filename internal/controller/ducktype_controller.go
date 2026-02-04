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

package controller

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/utils/ptr"
	"reconciler.io/runtime/reconcilers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"

	duckv1 "reconciler.io/ducks/api/v1"
	duckreconcilers "reconciler.io/ducks/reconcilers"
)

// +kubebuilder:rbac:groups=duck.reconciler.io,resources=ducktypes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=duck.reconciler.io,resources=ducktypes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=duck.reconciler.io,resources=ducktypes/finalizers,verbs=update
// +kubebuilder:rbac:groups=core;events.k8s.io,resources=events,verbs=get;list;watch;create;update;patch;delete

func DuckTypeReconciler(c reconcilers.Config) *reconcilers.ResourceReconciler[*duckv1.DuckType] {
	return &reconcilers.ResourceReconciler[*duckv1.DuckType]{
		Reconciler: &reconcilers.WithFinalizer[*duckv1.DuckType]{
			Finalizer: fmt.Sprintf("%s/reconciler", duckv1.GroupVersion.Group),

			Reconciler: reconcilers.Sequence[*duckv1.DuckType]{
				DuckClusterRoleChildSetReconciler(),
				DuckCustomResourceDefinitionChildReconciler(),
				DuckSubReconciler(),
			},
		},

		Config: c,
	}
}

// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete

func DuckClusterRoleChildSetReconciler() reconcilers.SubReconciler[*duckv1.DuckType] {
	return &reconcilers.ChildSetReconciler[*duckv1.DuckType, *rbacv1.ClusterRole, *rbacv1.ClusterRoleList]{
		DesiredChildren: func(ctx context.Context, resource *duckv1.DuckType) ([]*rbacv1.ClusterRole, error) {
			children := []*rbacv1.ClusterRole{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("reconcilerio-ducks-%s-view", resource.Name),
						Labels: map[string]string{
							"ducks.reconciler.io/type": resource.Name,
						},
					},
					Rules: []rbacv1.PolicyRule{},
					AggregationRule: &rbacv1.AggregationRule{
						ClusterRoleSelectors: []metav1.LabelSelector{
							{
								MatchLabels: map[string]string{
									"ducks.reconciler.io/type": resource.Name,
									"ducks.reconciler.io/role": "view",
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("reconcilerio-ducks-%s-edit", resource.Name),
						Labels: map[string]string{
							"ducks.reconciler.io/type": resource.Name,
						},
					},
					Rules: []rbacv1.PolicyRule{},
					AggregationRule: &rbacv1.AggregationRule{
						ClusterRoleSelectors: []metav1.LabelSelector{
							{
								MatchLabels: map[string]string{
									"ducks.reconciler.io/type": resource.Name,
									"ducks.reconciler.io/role": "edit",
								},
							},
						},
					},
				},
			}

			return children, nil
		},
		IdentifyChild: func(child *rbacv1.ClusterRole) string {
			return child.Name
		},
		ChildObjectManager: &reconcilers.UpdatingObjectManager[*rbacv1.ClusterRole]{
			HarmonizeImmutableFields: func(current, desired *rbacv1.ClusterRole) {
				if desired.AggregationRule != nil {
					// rules for aggregate roles are managed upstream
					desired.Rules = current.Rules
				}
			},
			MergeBeforeUpdate: func(current, desired *rbacv1.ClusterRole) {
				current.Labels = desired.Labels
				current.Rules = desired.Rules
				current.AggregationRule = desired.AggregationRule
			},
		},
		ReflectChildrenStatusOnParentWithError: func(ctx context.Context, parent *duckv1.DuckType, result reconcilers.ChildSetResult[*rbacv1.ClusterRole]) error {
			if err := result.AggregateError(); err != nil {
				if apierrs.IsInvalid(err) {
					parent.GetConditionManager(ctx).MarkFalse(duckv1.DuckTypeConditionRBAC, "Invalid", "%s", apierrs.ReasonForError(err))
				} else if apierrs.IsAlreadyExists(err) {
					parent.GetConditionManager(ctx).MarkFalse(duckv1.DuckTypeConditionRBAC, "AlreadyExists", "%s", apierrs.ReasonForError(err))
				} else {
					parent.GetConditionManager(ctx).MarkUnknown(duckv1.DuckTypeConditionRBAC, "Unknown", "")
					return err
				}
				return nil
			}

			parent.GetConditionManager(ctx).MarkTrue(duckv1.DuckTypeConditionRBAC, "Defined", "")

			return nil
		},
	}
}

// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;create;update;patch;delete

func DuckCustomResourceDefinitionChildReconciler() reconcilers.SubReconciler[*duckv1.DuckType] {
	return &reconcilers.ChildReconciler[*duckv1.DuckType, *apiextensionsv1.CustomResourceDefinition, *apiextensionsv1.CustomResourceDefinitionList]{
		DesiredChild: func(ctx context.Context, resource *duckv1.DuckType) (*apiextensionsv1.CustomResourceDefinition, error) {
			child := &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: resource.Name,
				},
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Group: resource.Spec.Group,
					Scope: apiextensionsv1.ClusterScoped,
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Plural:     resource.Spec.Plural,
						Singular:   resource.Spec.Singular,
						Kind:       resource.Spec.Kind,
						ListKind:   resource.Spec.ListKind,
						Categories: []string{"duck"},
					},
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{
							Name:    "v1",
							Served:  true,
							Storage: true,
							SelectableFields: []apiextensionsv1.SelectableField{
								{JSONPath: ".spec.group"},
								{JSONPath: ".spec.kind"},
							},
							Subresources: &apiextensionsv1.CustomResourceSubresources{
								Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
							},
							AdditionalPrinterColumns: []apiextensionsv1.CustomResourceColumnDefinition{
								{
									JSONPath: `.status.conditions[?(@.type=="Ready")].status`,
									Name:     "Ready",
									Type:     "string",
								},
								{
									JSONPath: `.status.conditions[?(@.type=="Ready")].reason`,
									Name:     "Reason",
									Type:     "string",
								},
								{
									JSONPath: `.metadata.creationTimestamp`,
									Name:     "Age",
									Type:     "date",
								},
							},
							Schema: &apiextensionsv1.CustomResourceValidation{
								OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"apiVersion": {
											Type: "string",
										},
										"kind": {
											Type: "string",
										},
										"metadata": {
											Type: "object",
										},
										"spec": {
											Properties: map[string]apiextensionsv1.JSONSchemaProps{
												"group": {
													Type: "string",
												},
												"version": {
													Type: "string",
												},
												"kind": {
													Type: "string",
												},
											},
											Required: []string{
												"group",
												"version",
												"kind",
											},
											Type: "object",
										},
										"status": {
											Properties: map[string]apiextensionsv1.JSONSchemaProps{
												"conditions": {
													Items: &apiextensionsv1.JSONSchemaPropsOrArray{
														Schema: &apiextensionsv1.JSONSchemaProps{
															Properties: map[string]apiextensionsv1.JSONSchemaProps{
																"lastTransitionTime": {
																	Format: "date-time",
																	Type:   "string",
																},
																"message": {
																	MaxLength: ptr.To(int64(32768)),
																	Type:      "string",
																},
																"observedGeneration": {
																	Format:  "int64",
																	Minimum: ptr.To(float64(0)),
																	Type:    "integer",
																},
																"reason": {
																	MaxLength: ptr.To(int64(1024)),
																	MinLength: ptr.To(int64(1)),
																	Pattern:   `^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$`,
																	Type:      "string",
																},
																"status": {
																	Enum: []apiextensionsv1.JSON{
																		{Raw: []byte(`"True"`)},
																		{Raw: []byte(`"False"`)},
																		{Raw: []byte(`"Unknown"`)},
																	},
																	Type: "string",
																},
																"type": {
																	MaxLength: ptr.To(int64(316)),
																	Pattern:   `^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/)?(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$`,
																	Type:      "string",
																},
															},
															Required: []string{
																"lastTransitionTime",
																"message",
																"reason",
																"status",
																"type",
															},
															Type: "object",
														},
													},
													Type: "array",
												},
												"observedGeneration": {
													Format: "int64",
													Type:   "integer",
												},
												"resolved": {
													Properties: map[string]apiextensionsv1.JSONSchemaProps{
														"apiVersion": {
															Type: "string",
														},
														"kind": {
															Type: "string",
														},
													},
													Type: "object",
												},
											},
											Type: "object",
										},
									},
									Type: "object",
								},
							},
						},
					},
					Conversion: &apiextensionsv1.CustomResourceConversion{
						Strategy: apiextensionsv1.NoneConverter,
					},
				},
			}

			return child, nil
		},
		ChildObjectManager: &reconcilers.UpdatingObjectManager[*apiextensionsv1.CustomResourceDefinition]{
			MergeBeforeUpdate: func(current, desired *apiextensionsv1.CustomResourceDefinition) {
				current.Labels = desired.Labels
				current.Spec = desired.Spec
			},
		},
		ReflectChildStatusOnParentWithError: func(ctx context.Context, parent *duckv1.DuckType, child *apiextensionsv1.CustomResourceDefinition, err error) error {
			if err != nil {
				if apierrs.IsInvalid(err) {
					parent.GetConditionManager(ctx).MarkFalse(duckv1.DuckTypeConditionCustomResourceDefinitionEstablished, "Invalid", "%s", apierrs.ReasonForError(err))
				} else if apierrs.IsAlreadyExists(err) {
					details := err.(apierrs.APIStatus).Status().Details
					parent.GetConditionManager(ctx).MarkFalse(duckv1.DuckTypeConditionCustomResourceDefinitionEstablished, "AlreadyExists", "another %s already exists with name %s", details.Kind, details.Name)
				} else {
					parent.GetConditionManager(ctx).MarkUnknown(duckv1.DuckTypeConditionCustomResourceDefinitionEstablished, "Unknown", "")
					// retry reconcile request
					return reconcilers.ErrQuiet
				}
				return nil
			}

			if child == nil {
				parent.GetConditionManager(ctx).MarkFalse(duckv1.DuckTypeConditionCustomResourceDefinitionEstablished, "Missing", "")
				return nil
			}

			idx := slices.IndexFunc(child.Status.Conditions, func(c apiextensionsv1.CustomResourceDefinitionCondition) bool {
				return c.Type == apiextensionsv1.Established
			})
			established := apiextensionsv1.CustomResourceDefinitionCondition{Reason: "Initializing"}
			if idx >= 0 {
				established = child.Status.Conditions[idx]
			}
			if established.Status != apiextensionsv1.ConditionTrue {
				if established.Status == apiextensionsv1.ConditionFalse {
					parent.GetConditionManager(ctx).MarkFalse(duckv1.DuckTypeConditionCustomResourceDefinitionEstablished, "NotEstablished", "child CustomResourceDefinition %s is not established", child.Name)
				} else {
					parent.GetConditionManager(ctx).MarkUnknown(duckv1.DuckTypeConditionCustomResourceDefinitionEstablished, "NotEstablished", "child CustomResourceDefinition %s is not established", child.Name)
				}
				return nil
			}

			parent.GetConditionManager(ctx).MarkTrue(duckv1.DuckTypeConditionCustomResourceDefinitionEstablished, "Established", "")

			return nil
		},
	}
}

func DuckSubReconciler() reconcilers.SubReconciler[*duckv1.DuckType] {
	syncPeriod := 10 * time.Hour
	return &duckreconcilers.SubManagerReconciler[*duckv1.DuckType]{
		AssertFinalizer: fmt.Sprintf("%s/reconciler", duckv1.GroupVersion.Group),
		SyncPeriod:      &syncPeriod,
		LocalTypes: func(ctx context.Context, resource *duckv1.DuckType) ([]schema.GroupKind, error) {
			return []schema.GroupKind{
				{Group: resource.Spec.Group, Kind: resource.Spec.Kind},
				{Group: resource.Spec.Group, Kind: resource.Spec.ListKind},
			}, nil
		},
		SetupWithSubManager: func(ctx context.Context, mgr ctrl.Manager, resource *duckv1.DuckType) error {
			config := reconcilers.NewConfig(mgr, nil, syncPeriod)
			typeMeta := metav1.TypeMeta{
				APIVersion: schema.GroupVersion{Group: resource.Spec.Group, Version: "v1"}.String(),
				Kind:       resource.Spec.Kind,
			}
			if err := DuckReconciler(config, typeMeta).SetupWithManager(ctx, mgr); err != nil {
				return err
			}

			return nil
		},
	}
}

// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch
// +kubebuilder:rbac:groups=apiregistration.k8s.io,resources=apiservices,verbs=get;list;watch

func DuckReconciler(c reconcilers.Config, typeMeta metav1.TypeMeta) *reconcilers.ResourceReconciler[*duckv1.Duck] {
	return &reconcilers.ResourceReconciler[*duckv1.Duck]{
		Type: &duckv1.Duck{
			TypeMeta: typeMeta,
		},
		Setup: func(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
			bldr.Named(strings.ToLower(schema.FromAPIVersionAndKind(typeMeta.APIVersion, typeMeta.Kind).GroupKind().String()))

			return nil
		},

		Reconciler: reconcilers.Sequence[*duckv1.Duck]{
			DuckReconcilerClusterRoleChildSetReconciler(),
			DuckReconcilerReadyCheck(),
		},

		Config: c,
	}
}

// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete

func DuckReconcilerClusterRoleChildSetReconciler() reconcilers.SubReconciler[*duckv1.Duck] {
	return &reconcilers.ChildSetReconciler[*duckv1.Duck, *rbacv1.ClusterRole, *rbacv1.ClusterRoleList]{
		DesiredChildren: func(ctx context.Context, resource *duckv1.Duck) ([]*rbacv1.ClusterRole, error) {
			c := reconcilers.RetrieveConfigOrDie(ctx)

			gvk, err := c.GroupVersionKindFor(resource)
			if err != nil {
				return nil, err
			}
			mapping, err := c.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
			if err != nil {
				return nil, err
			}

			gr := schema.ParseGroupResource(resource.Name)

			children := []*rbacv1.ClusterRole{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("reconcilerio-ducks-%s-%s-view", mapping.Resource.GroupResource().String(), resource.Name),
						Labels: map[string]string{
							"ducks.reconciler.io/type": mapping.Resource.GroupResource().String(),
							"ducks.reconciler.io/role": "view",
						},
						OwnerReferences: []metav1.OwnerReference{
							*metav1.NewControllerRef(resource, gvk),
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{gr.Group},
							Resources: []string{gr.Resource},
							Verbs:     []string{"get", "list", "watch"},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("reconcilerio-ducks-%s-%s-edit", mapping.Resource.GroupResource().String(), resource.Name),
						Labels: map[string]string{
							"ducks.reconciler.io/type": mapping.Resource.GroupResource().String(),
							"ducks.reconciler.io/role": "edit",
						},
						OwnerReferences: []metav1.OwnerReference{
							*metav1.NewControllerRef(resource, gvk),
						},
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{gr.Group},
							Resources: []string{gr.Resource},
							Verbs:     []string{"get", "list", "watch", "patch"},
						},
					},
				},
			}

			return children, nil
		},
		// use SkipOwnerReference/OurChild/ListOptions because defining an owner reference to a duck is broken
		// SkipOwnerReference: true,
		// OurChild: func(resource *duckv1.Duck, child *rbacv1.ClusterRole) bool {
		// 	return metav1.IsControlledBy(child, resource)
		// },
		// ListOptions: func(ctx context.Context, resource *duckv1.Duck) []client.ListOption {
		// 	return []client.ListOption{}
		// },
		IdentifyChild: func(child *rbacv1.ClusterRole) string {
			return child.Name
		},
		ChildObjectManager: &reconcilers.UpdatingObjectManager[*rbacv1.ClusterRole]{
			HarmonizeImmutableFields: func(current, desired *rbacv1.ClusterRole) {
				if desired.AggregationRule != nil {
					// rules for aggregate roles are managed upstream
					desired.Rules = current.Rules
				}
			},
			MergeBeforeUpdate: func(current, desired *rbacv1.ClusterRole) {
				current.Labels = desired.Labels
				current.Rules = desired.Rules
				current.AggregationRule = desired.AggregationRule
			},
		},
		ReflectChildrenStatusOnParentWithError: func(ctx context.Context, parent *duckv1.Duck, result reconcilers.ChildSetResult[*rbacv1.ClusterRole]) error {
			if err := result.AggregateError(); err != nil {
				if apierrs.IsInvalid(err) {
					parent.GetConditionManager(ctx).MarkFalse(duckv1.DuckConditionRBAC, "Invalid", "%s", apierrs.ReasonForError(err))
				} else if apierrs.IsAlreadyExists(err) {
					parent.GetConditionManager(ctx).MarkFalse(duckv1.DuckConditionRBAC, "AlreadyExists", "%s", apierrs.ReasonForError(err))
				} else {
					parent.GetConditionManager(ctx).MarkUnknown(duckv1.DuckConditionRBAC, "Unknown", "")
					return err
				}
				return nil
			}

			parent.GetConditionManager(ctx).MarkTrue(duckv1.DuckConditionRBAC, "Defined", "")

			return nil
		},
	}
}

// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch
// +kubebuilder:rbac:groups=apiregistration.k8s.io,resources=apiservices,verbs=get;list;watch

func DuckReconcilerReadyCheck() reconcilers.SubReconciler[*duckv1.Duck] {
	return &reconcilers.SyncReconciler[*duckv1.Duck]{
		Setup: func(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
			bldr.Watches(&apiextensionsv1.CustomResourceDefinition{}, reconcilers.EnqueueTracked(ctx))
			bldr.Watches(&apiregistrationv1.APIService{}, reconcilers.EnqueueTracked(ctx))

			return nil
		},
		Sync: func(ctx context.Context, resource *duckv1.Duck) error {
			c := reconcilers.RetrieveConfigOrDie(ctx)

			gvr := resource.GroupVersionResource()

			// track CRD or APIService that may back this duck, we don't actually care about the content, just to be notified on changes
			c.Tracker.TrackObject(&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: gvr.GroupResource().String()}}, resource)
			c.Tracker.TrackObject(&apiregistrationv1.APIService{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s.%s", gvr.Version, gvr.Group)}}, resource)

			resources, err := c.Discovery.ServerResourcesForGroupVersion(gvr.GroupVersion().String())
			if err != nil {
				if apierrs.IsNotFound(err) {
					resource.GetConditionManager(ctx).MarkFalse(duckv1.DuckConditionAvailable, "NotFound", "")
					return nil
				}
				return err
			}
			for _, apiResource := range resources.APIResources {
				if resource.Name == (schema.GroupResource{Resource: apiResource.Name, Group: gvr.Group}).String() {
					if name := fmt.Sprintf("%s.%s", apiResource.Name, resource.Spec.Group); name != resource.Name {
						resource.GetConditionManager(ctx).MarkFalse(duckv1.DuckConditionAvailable, "Invalid", ".spec.group does not match resolved resource")
						return nil
					}
					if kind := apiResource.Kind; kind != resource.Spec.Kind {
						resource.GetConditionManager(ctx).MarkFalse(duckv1.DuckConditionAvailable, "Invalid", ".spec.kind does not match resolved kind %q", kind)
						return nil
					}

					resource.GetConditionManager(ctx).MarkTrue(duckv1.DuckConditionAvailable, "Available", "")
					return nil
				}
			}

			resource.GetConditionManager(ctx).MarkFalse(duckv1.DuckConditionAvailable, "NotFound", "")
			return nil
		},
	}
}
