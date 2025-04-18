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

package client_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	dieappsv1 "reconciler.io/dies/apis/apps/v1"
	diebatchv1 "reconciler.io/dies/apis/batch/v1"
	diemetav1 "reconciler.io/dies/apis/meta/v1"
	"reconciler.io/runtime/reconcilers"
	rtesting "reconciler.io/runtime/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	duckv1 "reconciler.io/ducks/api/v1"
	duckclient "reconciler.io/ducks/client"
	"reconciler.io/ducks/internal/testresources"
)

func TestClient(t *testing.T) {
	name := "test-name"
	namespace := "test-namespace"
	request := reconcilers.Request{NamespacedName: types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}}
	tracking := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	now := time.Now()

	duckType := duckv1.DuckTypeBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Name("conditionducks.example.com")
			d.ResourceVersion("999")
			d.CreationTimestamp(metav1.NewTime(now))
		}).
		SpecDie(func(d *duckv1.DuckTypeSpecDie) {
			d.Group("example.com")
			d.Plural("conditionducks")
			d.Kind("ConditionDuck")
		}).
		StatusDie(func(d *duckv1.DuckTypeStatusDie) {
			d.InitializeConditions(now)
			d.ConditionDie(duckv1.DuckTypeConditionReady, func(d *diemetav1.ConditionDie) {
				d.Status(metav1.ConditionTrue)
			})
		})

	duckDeployment := duckType.AsDuck("deployments.apps").
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.ResourceVersion("999")
			d.CreationTimestamp(metav1.NewTime(now))
		}).
		SpecDie(func(d *duckv1.DuckSpecDie) {
			d.Group("apps")
			d.Version("v1")
			d.Kind("Deployment")
		}).
		StatusDie(func(d *duckv1.DuckStatusDie) {
			d.InitializeConditions(now)
			d.ConditionDie(duckv1.DuckConditionReady, func(d *diemetav1.ConditionDie) {
				d.Status(metav1.ConditionTrue)
			})
		})
	duckJob := duckType.AsDuck("jobs.batch").
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.ResourceVersion("999")
			d.CreationTimestamp(metav1.NewTime(now))
		}).
		SpecDie(func(d *duckv1.DuckSpecDie) {
			d.Group("batch")
			d.Version("v1")
			d.Kind("Job")
		}).
		StatusDie(func(d *duckv1.DuckStatusDie) {
			d.InitializeConditions(now)
			d.ConditionDie(duckv1.DuckConditionReady, func(d *diemetav1.ConditionDie) {
				d.Status(metav1.ConditionTrue)
			})
		})

	deploymentBlue := dieappsv1.DeploymentBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name("blue")
			d.AddLabel("color", "blue")
			d.ResourceVersion("999")
			d.CreationTimestamp(metav1.NewTime(now))
		}).
		StatusDie(func(d *dieappsv1.DeploymentStatusDie) {
			d.Conditions(
				appsv1.DeploymentCondition{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionTrue,
				},
				appsv1.DeploymentCondition{
					Type:   appsv1.DeploymentProgressing,
					Status: corev1.ConditionTrue,
				},
			)
		})
	deploymentGreen := dieappsv1.DeploymentBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name("green")
			d.AddLabel("color", "green")
			d.ResourceVersion("999")
			d.CreationTimestamp(metav1.NewTime(now))
		}).
		StatusDie(func(d *dieappsv1.DeploymentStatusDie) {
			d.Conditions(
				appsv1.DeploymentCondition{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionTrue,
				},
				appsv1.DeploymentCondition{
					Type:   appsv1.DeploymentProgressing,
					Status: corev1.ConditionFalse,
				},
			)
		})

	jobBlue := diebatchv1.JobBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name("blue")
			d.AddLabel("color", "blue")
			d.ResourceVersion("999")
			d.CreationTimestamp(metav1.NewTime(now))
		}).
		StatusDie(func(d *diebatchv1.JobStatusDie) {
			d.Conditions(
				batchv1.JobCondition{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionTrue,
				},
			)
		})
	jobGreen := diebatchv1.JobBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name("green")
			d.AddLabel("color", "green")
			d.ResourceVersion("999")
			d.CreationTimestamp(metav1.NewTime(now))
		}).
		StatusDie(func(d *diebatchv1.JobStatusDie) {
			d.Conditions(
				batchv1.JobCondition{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionFalse,
				},
			)
		})

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(duckv1.AddToScheme(scheme))

	tests := map[string]struct {
		config    *rtesting.ExpectConfig
		duckType  string
		op        func(t *testing.T, ctx context.Context, client duckclient.Client) (any, error)
		expected  any
		shouldErr bool
	}{
		"no duck type defined": {
			config:   &rtesting.ExpectConfig{},
			duckType: duckType.GetName(),
			op: func(t *testing.T, ctx context.Context, c duckclient.Client) (any, error) {
				obj := &testresources.ConditionDuck{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "blue",
					},
				}

				err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj)

				if !errors.Is(err, duckclient.ErrUnknownDuckType) {
					t.Errorf("expected err to be ErrUnknownDuckType, got: %s", err)
				}

				return nil, err
			},
			shouldErr: true,
		},
		"duck type not ready": {
			config: &rtesting.ExpectConfig{
				GivenObjects: []client.Object{
					duckType.
						StatusDie(func(d *duckv1.DuckTypeStatusDie) {
							d.ConditionDie(duckv1.DuckTypeConditionReady, func(d *diemetav1.ConditionDie) {
								d.Status(metav1.ConditionFalse)
							})
						}),
				},
			},
			duckType: duckType.GetName(),
			op: func(t *testing.T, ctx context.Context, c duckclient.Client) (any, error) {
				obj := &testresources.ConditionDuck{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "blue",
					},
				}

				err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj)

				if !errors.Is(err, duckclient.ErrDuckTypeNotReady) {
					t.Errorf("expected err to be ErrDuckTypeNotReady, got: %s", err)
				}

				return nil, err
			},
			shouldErr: true,
		},
		"no ducks defined for type": {
			config: &rtesting.ExpectConfig{
				GivenObjects: []client.Object{
					duckType,
					deploymentBlue,
				},
			},
			duckType: duckType.GetName(),
			op: func(t *testing.T, ctx context.Context, c duckclient.Client) (any, error) {
				obj := &testresources.ConditionDuck{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "blue",
					},
				}

				err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj)

				if !errors.Is(err, duckclient.ErrUnknownDuck) {
					t.Errorf("expected err to be ErrUnknownDuck, got: %s", err)
				}

				return nil, err
			},
			shouldErr: true,
		},
		"get": {
			config: &rtesting.ExpectConfig{
				GivenObjects: []client.Object{
					duckType,
					duckDeployment,
					duckJob,

					deploymentBlue,
					deploymentGreen,
					jobBlue,
					jobGreen,
				},
			},
			duckType: duckType.GetName(),
			op: func(t *testing.T, ctx context.Context, c duckclient.Client) (any, error) {
				obj := &testresources.ConditionDuck{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "blue",
					},
				}

				err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj)

				return obj, err
			},
			expected: deploymentBlue.
				DieDefaultTypeMetadata().
				DieReleaseDuck(&testresources.ConditionDuck{}),
		},
		"get normalized version": {
			config: &rtesting.ExpectConfig{
				GivenObjects: []client.Object{
					duckType,
					duckDeployment,
					duckJob,

					deploymentBlue,
					deploymentGreen,
					jobBlue,
					jobGreen,
				},
			},
			duckType: duckType.GetName(),
			op: func(t *testing.T, ctx context.Context, c duckclient.Client) (any, error) {
				obj := &testresources.ConditionDuck{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "batch/v1beta1",
						Kind:       "Job",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "blue",
					},
				}

				err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj)

				if expected, actual := "batch/v1", obj.APIVersion; expected != actual {
					t.Errorf("expected apiVersion to be %q, got %q", expected, actual)
				}

				return obj, err
			},
			expected: jobBlue.
				DieDefaultTypeMetadata().
				DieReleaseDuck(&testresources.ConditionDuck{}),
		},
		"get and track": {
			config: &rtesting.ExpectConfig{
				GivenObjects: []client.Object{
					duckType,
					duckDeployment,
					duckJob,

					deploymentBlue,
					deploymentGreen,
					jobBlue,
					jobGreen,
				},
				ExpectTracks: []rtesting.TrackRequest{
					rtesting.NewTrackRequest(duckType.AsDuck("").DieReleaseUnstructured(), tracking, scheme),
					rtesting.NewTrackRequest(deploymentBlue, tracking, scheme),
				},
			},
			duckType: duckType.GetName(),
			op: func(t *testing.T, ctx context.Context, c duckclient.Client) (any, error) {
				obj := &testresources.ConditionDuck{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "blue",
					},
				}

				err := c.TrackAndGet(ctx, client.ObjectKeyFromObject(obj), obj)

				return obj, err
			},
			expected: deploymentBlue.
				DieDefaultTypeMetadata().
				DieReleaseDuck(&testresources.ConditionDuck{}),
		},
		"list": {
			config: &rtesting.ExpectConfig{
				GivenObjects: []client.Object{
					duckType,
					duckDeployment,
					duckJob,

					deploymentBlue,
					deploymentGreen,
					jobBlue,
					jobGreen,
				},
			},
			duckType: duckType.GetName(),
			op: func(t *testing.T, ctx context.Context, c duckclient.Client) (any, error) {
				list := &testresources.ConditionDuckList{}

				err := c.List(ctx, list)

				return list, err
			},
			expected: &testresources.ConditionDuckList{
				Items: []testresources.ConditionDuck{
					testresources.ConditionDuckBlank.DieFeedDuck(deploymentBlue.DieDefaultTypeMetadata()).DieRelease(),
					testresources.ConditionDuckBlank.DieFeedDuck(deploymentGreen.DieDefaultTypeMetadata()).DieRelease(),
					testresources.ConditionDuckBlank.DieFeedDuck(jobBlue.DieDefaultTypeMetadata()).DieRelease(),
					testresources.ConditionDuckBlank.DieFeedDuck(jobGreen.DieDefaultTypeMetadata()).DieRelease(),
				},
			},
		},
		"list normalized version": {
			config: &rtesting.ExpectConfig{
				GivenObjects: []client.Object{
					duckType,
					duckDeployment,
					duckJob,

					deploymentBlue,
					deploymentGreen,
					jobBlue,
					jobGreen,
				},
			},
			duckType: duckType.GetName(),
			op: func(t *testing.T, ctx context.Context, c duckclient.Client) (any, error) {
				list := &testresources.ConditionDuckList{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "batch/v1beta1",
						Kind:       "JobList",
					},
				}

				err := c.List(ctx, list)

				return list, err
			},
			expected: &testresources.ConditionDuckList{
				Items: []testresources.ConditionDuck{
					testresources.ConditionDuckBlank.DieFeedDuck(jobBlue.DieDefaultTypeMetadata()).DieRelease(),
					testresources.ConditionDuckBlank.DieFeedDuck(jobGreen.DieDefaultTypeMetadata()).DieRelease(),
				},
			},
		},
		"list and track": {
			config: &rtesting.ExpectConfig{
				GivenObjects: []client.Object{
					duckType,
					duckDeployment,
					duckJob,

					deploymentBlue,
					deploymentGreen,
					jobBlue,
					jobGreen,
				},
				ExpectTracks: []rtesting.TrackRequest{
					rtesting.NewTrackRequest(duckType.AsDuck("").DieReleaseUnstructured(), tracking, scheme),
					rtesting.NewTrackRequest(&appsv1.Deployment{}, tracking, scheme),
					rtesting.NewTrackRequest(&batchv1.Job{}, tracking, scheme),
				},
			},
			duckType: duckType.GetName(),
			op: func(t *testing.T, ctx context.Context, c duckclient.Client) (any, error) {
				list := &testresources.ConditionDuckList{}

				err := c.TrackAndList(ctx, list)

				return list, err
			},
			expected: &testresources.ConditionDuckList{
				Items: []testresources.ConditionDuck{
					testresources.ConditionDuckBlank.DieFeedDuck(deploymentBlue.DieDefaultTypeMetadata()).DieRelease(),
					testresources.ConditionDuckBlank.DieFeedDuck(deploymentGreen.DieDefaultTypeMetadata()).DieRelease(),
					testresources.ConditionDuckBlank.DieFeedDuck(jobBlue.DieDefaultTypeMetadata()).DieRelease(),
					testresources.ConditionDuckBlank.DieFeedDuck(jobGreen.DieDefaultTypeMetadata()).DieRelease(),
				},
			},
		},
		"list with selector": {
			config: &rtesting.ExpectConfig{
				GivenObjects: []client.Object{
					duckType,
					duckDeployment,
					duckJob,

					deploymentBlue,
					deploymentGreen,
					jobBlue,
					jobGreen,
				},
			},
			duckType: duckType.GetName(),
			op: func(t *testing.T, ctx context.Context, c duckclient.Client) (any, error) {
				list := &testresources.ConditionDuckList{}

				err := c.List(ctx, list, client.MatchingLabels{"color": "green"})

				return list, err
			},
			expected: &testresources.ConditionDuckList{
				Items: []testresources.ConditionDuck{
					testresources.ConditionDuckBlank.DieFeedDuck(deploymentGreen.DieDefaultTypeMetadata()).DieRelease(),
					testresources.ConditionDuckBlank.DieFeedDuck(jobGreen.DieDefaultTypeMetadata()).DieRelease(),
				},
			},
		},
		"list while duck not ready": {
			config: &rtesting.ExpectConfig{
				GivenObjects: []client.Object{
					duckType,
					duckDeployment,
					duckJob.
						StatusDie(func(d *duckv1.DuckStatusDie) {
							d.ConditionDie(duckv1.DuckConditionReady, func(d *diemetav1.ConditionDie) {
								d.Status(metav1.ConditionFalse)
							})
						}),

					deploymentBlue,
					deploymentGreen,
					jobBlue,
					jobGreen,
				},
			},
			duckType: duckType.GetName(),
			op: func(t *testing.T, ctx context.Context, c duckclient.Client) (any, error) {
				list := &testresources.ConditionDuckList{}

				err := c.List(ctx, list)

				return list, err
			},
			expected: &testresources.ConditionDuckList{
				Items: []testresources.ConditionDuck{
					testresources.ConditionDuckBlank.DieFeedDuck(deploymentBlue.DieDefaultTypeMetadata()).DieRelease(),
					testresources.ConditionDuckBlank.DieFeedDuck(deploymentGreen.DieDefaultTypeMetadata()).DieRelease(),
				},
			},
		},
		"list api not marked as a duck": {
			config: &rtesting.ExpectConfig{
				GivenObjects: []client.Object{
					duckType,
					duckDeployment,
					// duckJob,

					deploymentBlue,
					deploymentGreen,
					jobBlue,
					jobGreen,
				},
			},
			duckType: duckType.GetName(),
			op: func(t *testing.T, ctx context.Context, c duckclient.Client) (any, error) {
				list := &testresources.ConditionDuckList{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "batch/v1",
						Kind:       "Job",
					},
				}

				err := c.List(ctx, list)

				if !errors.Is(err, duckclient.ErrUnknownDuck) {
					t.Errorf("expected err to be ErrUnknownDuck, got: %s", err)
				}

				return nil, err
			},
			shouldErr: true,
		},
		"delete all": {
			config: &rtesting.ExpectConfig{
				GivenObjects: []client.Object{
					duckType,
					duckDeployment,
					duckJob,

					deploymentBlue,
					deploymentGreen,
					jobBlue,
					jobGreen,
				},
				ExpectDeleteCollections: []rtesting.DeleteCollectionRef{
					{Group: "apps", Kind: "Deployment"},
					{Group: "batch", Kind: "Job"},
				},
			},
			duckType: duckType.GetName(),
			op: func(t *testing.T, ctx context.Context, c duckclient.Client) (any, error) {
				obj := &testresources.ConditionDuck{}

				err := c.DeleteAllOf(ctx, obj)

				return nil, err
			},
		},
		"delete all in namespace": {
			config: &rtesting.ExpectConfig{
				GivenObjects: []client.Object{
					duckType,
					duckDeployment,
					duckJob,

					deploymentBlue,
					deploymentGreen,
					jobBlue,
					jobGreen,
				},
				ExpectDeleteCollections: []rtesting.DeleteCollectionRef{
					{Group: "apps", Kind: "Deployment", Namespace: namespace},
					{Group: "batch", Kind: "Job", Namespace: namespace},
				},
			},
			duckType: duckType.GetName(),
			op: func(t *testing.T, ctx context.Context, c duckclient.Client) (any, error) {
				obj := &testresources.ConditionDuck{}

				err := c.DeleteAllOf(ctx, obj, client.InNamespace(namespace))

				return nil, err
			},
		},
		"delete all with selector": {
			config: &rtesting.ExpectConfig{
				GivenObjects: []client.Object{
					duckType,
					duckDeployment,
					duckJob,

					deploymentBlue,
					deploymentGreen,
					jobBlue,
					jobGreen,
				},
				ExpectDeleteCollections: []rtesting.DeleteCollectionRef{
					{Group: "apps", Kind: "Deployment", Labels: labels.SelectorFromSet(labels.Set{"color": "blue"})},
					{Group: "batch", Kind: "Job", Labels: labels.SelectorFromSet(labels.Set{"color": "blue"})},
				},
			},
			duckType: duckType.GetName(),
			op: func(t *testing.T, ctx context.Context, c duckclient.Client) (any, error) {
				obj := &testresources.ConditionDuck{}

				err := c.DeleteAllOf(ctx, obj, client.MatchingLabels{"color": "blue"})

				return nil, err
			},
		},
		"delete all of kind": {
			config: &rtesting.ExpectConfig{
				GivenObjects: []client.Object{
					duckType,
					duckDeployment,
					duckJob,

					deploymentBlue,
					deploymentGreen,
					jobBlue,
					jobGreen,
				},
				ExpectDeleteCollections: []rtesting.DeleteCollectionRef{
					{Group: "batch", Kind: "Job"},
				},
			},
			duckType: duckType.GetName(),
			op: func(t *testing.T, ctx context.Context, c duckclient.Client) (any, error) {
				obj := &testresources.ConditionDuck{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "batch/v1beta1",
						Kind:       "Job",
					},
				}

				err := c.DeleteAllOf(ctx, obj)

				return nil, err
			},
		},
		"delete all unknown kind": {
			config: &rtesting.ExpectConfig{
				GivenObjects: []client.Object{
					duckType,
					duckDeployment,
					// duckJob,

					deploymentBlue,
					deploymentGreen,
					jobBlue,
					jobGreen,
				},
			},
			duckType: duckType.GetName(),
			op: func(t *testing.T, ctx context.Context, c duckclient.Client) (any, error) {
				obj := &testresources.ConditionDuck{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "batch/v1",
						Kind:       "Job",
					},
				}

				err := c.DeleteAllOf(ctx, obj)

				if !errors.Is(err, duckclient.ErrUnknownDuck) {
					t.Errorf("expected err to be ErrUnknownDuck, got: %s", err)
				}

				return nil, err
			},
			shouldErr: true,
		},
		"normalize object's group version kind": {
			config: &rtesting.ExpectConfig{
				GivenObjects: []client.Object{
					duckType,
					duckDeployment,
					duckJob,

					deploymentBlue,
					deploymentGreen,
					jobBlue,
					jobGreen,
				},
			},
			duckType: duckType.GetName(),
			op: func(t *testing.T, ctx context.Context, c duckclient.Client) (any, error) {
				obj := &testresources.ConditionDuck{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "batch/v1beta1",
						Kind:       "Job",
					},
				}

				gvk, err := c.GroupVersionKindFor(obj)

				return gvk, err
			},
			expected: schema.GroupVersionKind{
				Group:   "batch",
				Version: "v1",
				Kind:    "Job",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			test.config.Name = name
			test.config.Scheme = scheme

			ctx := context.TODO()
			ctx = reconcilers.StashRequest(ctx, request)
			ctx = reconcilers.StashResourceType(ctx, &corev1.Pod{})

			config := test.config.Config()
			client := duckclient.New(test.duckType, config)

			actual, err := test.op(t, ctx, client)
			if err != nil && !test.shouldErr {
				t.Errorf("unexpected err: %s", err)
			}
			if err == nil && test.shouldErr {
				t.Errorf("expected to err")
			}

			if diff := cmp.Diff(test.expected, actual); diff != "" {
				t.Errorf("unexpected diff (-expected, +actual): %s", diff)
			}

			test.config.AssertExpectations(t)
		})
	}
}

func Into[T any](u *unstructured.Unstructured, obj T) T {
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		panic(err)
	}
	return obj
}
