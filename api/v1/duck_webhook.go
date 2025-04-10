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
	"strings"

	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (r *Duck) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithDefaulter(r).
		WithValidator(r).
		Complete()
}

var _ webhook.CustomDefaulter = &Duck{}

func (r *Duck) Default(ctx context.Context, obj runtime.Object) error {
	r = obj.(*Duck)

	if err := r.Spec.Default(ctx); err != nil {
		return err
	}

	return nil
}

func (r *DuckSpec) Default(ctx context.Context) error {
	return nil
}

var _ webhook.CustomValidator = &Duck{}

func (r *Duck) ValidateCreate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	if err := r.Default(ctx, obj); err != nil {
		return nil, err
	}
	r = obj.(*Duck)

	return nil, r.Validate(ctx, field.NewPath("")).ToAggregate()
}

func (r *Duck) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	if err := r.Default(ctx, newObj); err != nil {
		return nil, err
	}
	r = newObj.(*Duck)

	return nil, r.Validate(ctx, field.NewPath("")).ToAggregate()
}

func (r *Duck) ValidateDelete(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	return
}

func (r *Duck) Validate(ctx context.Context, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	errs = append(errs, r.Spec.Validate(ctx, fldPath.Child("spec"))...)
	if !strings.HasSuffix(r.Name, "."+r.Spec.Group) {
		errs = append(errs, field.Invalid(fldPath.Child("spec", "group"), r.Spec.Group, "resource name must end with group"))
	}

	return errs
}

func (r *DuckSpec) Validate(ctx context.Context, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if r.Group == "" {
		errs = append(errs, field.Required(fldPath.Child("group"), ""))
	}
	if r.Version == "" {
		errs = append(errs, field.Required(fldPath.Child("version"), ""))
	}
	if r.Kind == "" {
		errs = append(errs, field.Required(fldPath.Child("kind"), ""))
	}

	return errs
}
