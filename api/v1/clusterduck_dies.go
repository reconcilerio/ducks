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
	ClusterDuckConditionReadyBlank = diemetav1.ConditionBlank.Type(ClusterDuckConditionReady).Status(metav1.ConditionUnknown).Reason("Initializing")
)

func (d *ClusterDuckStatusDie) InitializeConditionsDie(now time.Time) *ClusterDuckStatusDie {
	ctx := rtime.StashNow(context.TODO(), now)
	return d.DieStamp(func(r *ClusterDuckStatus) {
		r.InitializeConditions(ctx)
	})
}

func (d *ClusterDuckStatusDie) ObservedGeneration(v int64) *ClusterDuckStatusDie {
	return d.DieStamp(func(r *ClusterDuckStatus) {
		r.ObservedGeneration = v
	})
}

func (d *ClusterDuckStatusDie) Conditions(v ...metav1.Condition) *ClusterDuckStatusDie {
	return d.DieStamp(func(r *ClusterDuckStatus) {
		r.Conditions = v
	})
}

func (d *ClusterDuckStatusDie) ConditionsDie(v ...*diemetav1.ConditionDie) *ClusterDuckStatusDie {
	return d.DieStamp(func(r *ClusterDuckStatus) {
		r.Conditions = make([]metav1.Condition, len(v))
		for i := range v {
			r.Conditions[i] = v[i].DieRelease()
		}
	})
}
