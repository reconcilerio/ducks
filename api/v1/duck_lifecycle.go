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

package v1

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reconciler.io/runtime/apis"
)

const (
	DuckConditionReady     = apis.ConditionReady
	DuckConditionRBAC      = "RBAC"
	DuckConditionAvailable = "Available"
)

func (r *Duck) GetConditionsAccessor() apis.ConditionsAccessor {
	return &r.Status
}

func (r *Duck) GetConditionSet() apis.ConditionSet {
	return r.Status.GetConditionSet()
}

func (r *DuckStatus) GetConditionSet() apis.ConditionSet {
	return apis.NewLivingConditionSetWithHappyReason(
		"Ready",
		DuckConditionRBAC,
		DuckConditionAvailable,
	)
}

func (r *Duck) GetConditionManager(ctx context.Context) apis.ConditionManager {
	return r.Status.GetConditionManager(ctx)
}

func (r *DuckStatus) GetConditionManager(ctx context.Context) apis.ConditionManager {
	return r.GetConditionSet().ManageWithContext(ctx, r)
}

func (r *DuckStatus) InitializeConditions(ctx context.Context) {
	r.GetConditionManager(ctx).InitializeConditions()
}

var _ apis.ConditionsAccessor = (*DuckStatus)(nil)

func (r *Duck) GroupVersionResource() schema.GroupVersionResource {
	return schema.ParseGroupResource(r.Name).WithVersion(r.Spec.Version)
}

func (r *DuckSpec) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   r.Group,
		Version: r.Version,
		Kind:    r.Kind,
	}
}

func (r *DuckSpec) TypeMeta() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: schema.GroupVersion{
			Group:   r.Group,
			Version: r.Version,
		}.String(),
		Kind: r.Kind,
	}
}
