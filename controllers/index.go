/*
Copyright 2022.

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

package controllers

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/smartxworks/cluster-api-provider-elf/api/v1beta1"
)

// indexByElfMachineTemplateName adds the index by TemplateClonedFromNameAnnotation to the
// managers cache.
func indexByElfMachineTemplateName(ctx context.Context, mgr ctrl.Manager) error {
	if err := mgr.GetCache().IndexField(ctx, &infrav1.ElfMachine{},
		infrav1.TemplateClonedFromNameAnnotationField,
		elfMachineByElfMachineTemplateName,
	); err != nil {
		return errors.Wrap(err, "failed to set index field for TemplateClonedFromNameAnnotationField")
	}

	return nil
}

func elfMachineByElfMachineTemplateName(o client.Object) []string {
	elfMachine, ok := o.(*infrav1.ElfMachine)
	if !ok {
		panic(fmt.Sprintf("Expected ElfMachine but got a %T", o))
	}

	if value, ok := elfMachine.Annotations[clusterv1.TemplateClonedFromNameAnnotation]; ok {
		return []string{value}
	}

	return nil
}
