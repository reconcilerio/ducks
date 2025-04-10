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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	diemetav1 "reconciler.io/dies/apis/meta/v1"
	rtime "reconciler.io/runtime/time"
)

var (
	DuckTypeConditionReadyBlank                               = diemetav1.ConditionBlank.Type(DuckTypeConditionReady).Status(metav1.ConditionUnknown).Reason("Initializing")
	DuckTypeConditionRBACBlanks                               = diemetav1.ConditionBlank.Type(DuckTypeConditionRBAC).Status(metav1.ConditionUnknown).Reason("Initializing")
	DuckTypeConditionCustomResourceDefinitionEstablishedBlank = diemetav1.ConditionBlank.Type(DuckTypeConditionCustomResourceDefinitionEstablished).Status(metav1.ConditionUnknown).Reason("Initializing")
)

func (d *DuckTypeStatusDie) InitializeConditionsDie(now time.Time) *DuckTypeStatusDie {
	ctx := rtime.StashNow(context.TODO(), now)
	return d.DieStamp(func(r *DuckTypeStatus) {
		r.InitializeConditions(ctx)
	})
}

func (d *DuckTypeStatusDie) ObservedGeneration(v int64) *DuckTypeStatusDie {
	return d.DieStamp(func(r *DuckTypeStatus) {
		r.ObservedGeneration = v
	})
}

func (d *DuckTypeStatusDie) Conditions(v ...metav1.Condition) *DuckTypeStatusDie {
	return d.DieStamp(func(r *DuckTypeStatus) {
		r.Conditions = v
	})
}

// ConditionDie mutates a single item in Conditions matched by the nested field Type, appending a new item if no match is found.
func (d *DuckTypeStatusDie) ConditionDie(v string, fn func(d *diemetav1.ConditionDie)) *DuckTypeStatusDie {
	return d.DieStamp(func(r *DuckTypeStatus) {
		for i := range r.Conditions {
			if v == r.Conditions[i].Type {
				d := diemetav1.ConditionBlank.DieImmutable(false).DieFeed(r.Conditions[i])
				fn(d)
				r.Conditions[i] = d.DieRelease()
				return
			}
		}

		d := diemetav1.ConditionBlank.DieImmutable(false).DieFeed(metav1.Condition{Type: v})
		fn(d)
		r.Conditions = append(r.Conditions, d.DieRelease())
	})
}
