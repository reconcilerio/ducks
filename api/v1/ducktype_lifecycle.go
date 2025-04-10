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

	"reconciler.io/runtime/apis"
)

const (
	DuckTypeConditionReady                               = apis.ConditionReady
	DuckTypeConditionRBAC                                = "RBAC"
	DuckTypeConditionCustomResourceDefinitionEstablished = "CustomResourceDefinitionEstablished"
)

func (r *DuckType) GetConditionsAccessor() apis.ConditionsAccessor {
	return &r.Status
}

func (r *DuckType) GetConditionSet() apis.ConditionSet {
	return r.Status.GetConditionSet()
}

func (r *DuckTypeStatus) GetConditionSet() apis.ConditionSet {
	return apis.NewLivingConditionSetWithHappyReason(
		"Ready",
		DuckTypeConditionRBAC,
		DuckTypeConditionCustomResourceDefinitionEstablished,
	)
}

func (r *DuckType) GetConditionManager(ctx context.Context) apis.ConditionManager {
	return r.Status.GetConditionManager(ctx)
}

func (r *DuckTypeStatus) GetConditionManager(ctx context.Context) apis.ConditionManager {
	return r.GetConditionSet().ManageWithContext(ctx, r)
}

func (r *DuckTypeStatus) InitializeConditions(ctx context.Context) {
	r.GetConditionManager(ctx).InitializeConditions()
}

var _ apis.ConditionsAccessor = (*DuckTypeStatus)(nil)
