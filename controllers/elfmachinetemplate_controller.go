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
	goctx "context"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	infrav1 "github.com/smartxworks/cluster-api-provider-elf/api/v1beta1"
	"github.com/smartxworks/cluster-api-provider-elf/pkg/context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlmgr "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/smartxworks/cluster-api-provider-elf/pkg/util"
)

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elfmachinetemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elfmachinetemplates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=elfmachinetemplates/finalizers,verbs=update

// ElfMachineTemplateReconciler reconciles a ElfMachineTemplate object.
type ElfMachineTemplateReconciler struct {
	*context.ControllerContext
}

// AddMachineTemplateControllerToManager adds the machine template controller to the provided
// manager.
func AddMachineTemplateControllerToManager(ctx *context.ControllerManagerContext, mgr ctrlmgr.Manager) error {
	var (
		controlledType      = &infrav1.ElfMachineTemplate{}
		controlledTypeName  = reflect.TypeOf(controlledType).Elem().Name()
		controllerNameShort = fmt.Sprintf("%s-controller", strings.ToLower(controlledTypeName))
	)

	// Build the controller context.
	controllerContext := &context.ControllerContext{
		ControllerManagerContext: ctx,
		Name:                     controllerNameShort,
		Logger:                   ctx.Logger.WithName(controllerNameShort),
	}

	reconciler := &ElfMachineTemplateReconciler{
		ControllerContext: controllerContext,
	}

	return ctrl.NewControllerManagedBy(mgr).
		// Watch the controlled, infrastructure resource.
		For(controlledType).
		Complete(reconciler)
}

// Reconcile ensures the back-end state reflects the Kubernetes resource state intent.
func (r *ElfMachineTemplateReconciler) Reconcile(ctx goctx.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	// Get the ElfMachineTemplate resource for this request.
	var elfMachineTemplate infrav1.ElfMachineTemplate
	if err := r.Client.Get(r, req.NamespacedName, &elfMachineTemplate); err != nil {
		if apierrors.IsNotFound(err) {
			r.Logger.Info("ElfMachineTemplate not found, won't reconcile", "key", req.NamespacedName)

			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	// Create the patch helper.
	patchHelper, err := patch.NewHelper(&elfMachineTemplate, r.Client)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to init patch helper for %s %s/%s", elfMachineTemplate.GroupVersionKind(), elfMachineTemplate.Namespace, elfMachineTemplate.Name)
	}

	logger := r.Logger.WithValues("namespace", elfMachineTemplate.Namespace, "elfMachineTemplate", elfMachineTemplate.Name)

	// Create the machine template context for this request.
	machineTemplateContext := &context.MachineTemplateContext{
		ControllerContext:  r.ControllerContext,
		ElfMachineTemplate: &elfMachineTemplate,
		Logger:             logger,
		PatchHelper:        patchHelper,
	}

	// Always issue a patch when exiting this function so changes to the
	// resource are patched back to the API server.
	defer func() {
		// Patch the ElfMachine resource.
		if err := machineTemplateContext.Patch(); err != nil {
			if reterr == nil {
				reterr = err
			}

			machineTemplateContext.Logger.Error(err, "patch failed", "elfMachine", machineTemplateContext.String())
		}
	}()

	// Handle deleted machine templates
	if !elfMachineTemplate.ObjectMeta.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, nil
	}

	// Handle non-deleted machine templates
	return r.reconcileNormal(machineTemplateContext)
}

func (r *ElfMachineTemplateReconciler) reconcileNormal(ctx *context.MachineTemplateContext) (reconcile.Result, error) {
	machines, err := util.GetElfMachinesInMachineTemplate(ctx, ctx.Client, ctx.ElfMachineTemplate.Namespace, ctx.ElfMachineTemplate.Name)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Sync virtual machine resources of ElfMachineTemplate to ElfMachines
	template := ctx.ElfMachineTemplate.Spec.Template.Spec
	for i := 0; i < len(machines); i++ {
		machine := machines[i]
		patchHelper, err := patch.NewHelper(machine, r.Client)
		if err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "failed to init patch helper for %s %s/%s", machine.GroupVersionKind(), machine.Namespace, machine.Name)
		}

		if !ShouldELFMachineNeedUpdate(template, *machine) {
			continue
		}

		machine.Spec.NumCPUs = template.NumCPUs
		machine.Spec.MemoryMiB = template.MemoryMiB
		machine.Spec.Network = template.Network

		if err := patchHelper.Patch(ctx, machine); err != nil {
			return reconcile.Result{}, errors.Wrapf(err, "failed to patch ElfMachine %s to sync VM resources from ElfMachineTemplate %s", machine.Name, ctx.ElfMachineTemplate.Name)
		}

		ctx.Logger.Info(fmt.Sprintf("Sync ElfMachine %s VM resources from ElfMachineTemplate %s successfully.", machine.Name, ctx.ElfMachineTemplate.Name))
	}

	return reconcile.Result{}, nil
}

func ShouldELFMachineNeedUpdate(template infrav1.ElfMachineSpec, machine infrav1.ElfMachine) bool {
	numCPUs := machine.Spec.NumCPUs
	memoryMiB := machine.Spec.MemoryMiB
	if numCPUs != template.NumCPUs || memoryMiB != template.MemoryMiB {
		return true
	}

	if !reflect.DeepEqual(template.Network.Devices, machine.Spec.Network.Devices) {
		return true
	}

	return false
}