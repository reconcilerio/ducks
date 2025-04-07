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
	ClusterDuckConditionReady = apis.ConditionReady
)

func (r *ClusterDuck) GetConditionsAccessor() apis.ConditionsAccessor {
	return &r.Status
}

func (r *ClusterDuck) GetConditionSet() apis.ConditionSet {
	return r.Status.GetConditionSet()
}

func (r *ClusterDuckStatus) GetConditionSet() apis.ConditionSet {
	return apis.NewLivingConditionSetWithHappyReason(
		"Ready",
	)
}

func (r *ClusterDuck) GetConditionManager(ctx context.Context) apis.ConditionManager {
	return r.Status.GetConditionManager(ctx)
}

func (r *ClusterDuckStatus) GetConditionManager(ctx context.Context) apis.ConditionManager {
	return r.GetConditionSet().ManageWithContext(ctx, r)
}

func (r *ClusterDuckStatus) InitializeConditions(ctx context.Context) {
	r.GetConditionManager(ctx).InitializeConditions()
}

var _ apis.ConditionsAccessor = (*ClusterDuckStatus)(nil)
