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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/validate-duck-reconciler-io-v1-ducktype,mutating=false,failurePolicy=fail,sideEffects=None,groups=duck.reconciler.io,resources=ducktypes,verbs=create;update,versions=v1,name=v1.ducktypes.duck.reconciler.io,admissionReviewVersions={v1,v1beta1}

func (r *DuckType) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, r).
		WithDefaulter(r).
		WithValidator(r).
		Complete()
}

var _ admission.Defaulter[*DuckType] = &DuckType{}

func (r *DuckType) Default(ctx context.Context, obj *DuckType) error {
	if err := obj.Spec.Default(ctx); err != nil {
		return err
	}

	return nil
}

func (r *DuckTypeSpec) Default(ctx context.Context) error {
	if r.Singular == "" {
		r.Singular = strings.ToLower(r.Kind)
	}
	if r.ListKind == "" {
		r.ListKind = fmt.Sprintf("%sList", r.Kind)
	}

	return nil
}

var _ admission.Validator[*DuckType] = &DuckType{}

func (r *DuckType) ValidateCreate(ctx context.Context, obj *DuckType) (warnings admission.Warnings, err error) {
	if err := r.Default(ctx, obj); err != nil {
		return nil, err
	}

	return nil, obj.Validate(ctx, field.NewPath("")).ToAggregate()
}

func (r *DuckType) ValidateUpdate(ctx context.Context, oldObj, newObj *DuckType) (warnings admission.Warnings, err error) {
	if err := r.Default(ctx, newObj); err != nil {
		return nil, err
	}

	return nil, newObj.Validate(ctx, field.NewPath("")).ToAggregate()
}

func (r *DuckType) ValidateDelete(ctx context.Context, obj *DuckType) (warnings admission.Warnings, err error) {
	return
}

func (r *DuckType) Validate(ctx context.Context, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if r.Name != fmt.Sprintf("%s.%s", r.Spec.Plural, r.Spec.Group) {
		errs = append(errs, field.Invalid(fldPath.Child("metadata", "name"), r.Name, "name must take the form `<plural>.<group>`"))
	}
	errs = append(errs, r.Spec.Validate(ctx, fldPath.Child("spec"))...)

	return errs
}

func (r *DuckTypeSpec) Validate(ctx context.Context, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if r.Group == "" {
		errs = append(errs, field.Required(fldPath.Child("group"), ""))
	} else if strings.ToLower(r.Group) != r.Group {
		errs = append(errs, field.Invalid(fldPath.Child("group"), r.Group, "must be all lowercase"))
	}
	if r.Plural == "" {
		errs = append(errs, field.Required(fldPath.Child("plural"), ""))
	} else if strings.ToLower(r.Plural) != r.Plural {
		errs = append(errs, field.Invalid(fldPath.Child("plural"), r.Plural, "must be all lowercase"))
	}
	if r.Singular == "" {
		// defaulted
		errs = append(errs, field.Required(fldPath.Child("singular"), ""))
	} else if strings.ToLower(r.Singular) != r.Singular {
		errs = append(errs, field.Invalid(fldPath.Child("singular"), r.Singular, "must be all lowercase"))
	}
	if r.Kind == "" {
		errs = append(errs, field.Required(fldPath.Child("kind"), ""))
	}
	if r.ListKind == "" {
		// defaulted
		errs = append(errs, field.Required(fldPath.Child("listKind"), ""))
	}

	return errs
}
