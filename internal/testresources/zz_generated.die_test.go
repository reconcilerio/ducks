//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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

// Code generated by diegen. DO NOT EDIT.

package testresources

import (
	testingx "testing"

	testing "reconciler.io/dies/testing"
)

func TestConditionDuckDie_MissingMethods(t *testingx.T) {
	die := ConditionDuckBlank
	ignore := []string{"TypeMeta", "ObjectMeta"}
	diff := testing.DieFieldDiff(die).Delete(ignore...)
	if diff.Len() != 0 {
		t.Errorf("found missing fields for ConditionDuckDie: %s", diff.List())
	}
}

func TestConditionDuckStatusDie_MissingMethods(t *testingx.T) {
	die := ConditionDuckStatusBlank
	ignore := []string{}
	diff := testing.DieFieldDiff(die).Delete(ignore...)
	if diff.Len() != 0 {
		t.Errorf("found missing fields for ConditionDuckStatusDie: %s", diff.List())
	}
}
