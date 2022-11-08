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
	"bytes"
	goctx "context"
	"flag"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capiutil "sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"

	infrav1 "github.com/smartxworks/cluster-api-provider-elf/api/v1beta1"
	"github.com/smartxworks/cluster-api-provider-elf/test/fake"
)

var _ = Describe("ElfMachineTemplateReconciler", func() {
	var (
		elfMachine         *infrav1.ElfMachine
		elfMachineTemplate *infrav1.ElfMachineTemplate
		logBuffer          *bytes.Buffer
	)

	ctx := goctx.Background()

	BeforeEach(func() {
		// set log
		if err := flag.Set("logtostderr", "false"); err != nil {
			_ = fmt.Errorf("Error setting logtostderr flag")
		}
		if err := flag.Set("v", "6"); err != nil {
			_ = fmt.Errorf("Error setting v flag")
		}
		logBuffer = new(bytes.Buffer)
		klog.SetOutput(logBuffer)

		_, _, elfMachine, _, _ = fake.NewClusterAndMachineObjects()
		elfMachineTemplate = fake.NewMachineTemplate()
		elfMachine.Annotations = map[string]string{clusterv1.TemplateClonedFromNameAnnotation: elfMachineTemplate.Name}
	})

	Context("Reconcile an ElfMachineTemplate", func() {
		It("should not reconcile when ElfMachineTemplate not found", func() {
			ctrlContext := newCtrlContexts()

			reconciler := &ElfMachineTemplateReconciler{ControllerContext: ctrlContext}
			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: capiutil.ObjectKey(elfMachineTemplate)})
			Expect(result).To(BeZero())
			Expect(err).To(BeNil())
			Expect(logBuffer.String()).To(ContainSubstring("ElfMachineTemplate not found, won't reconcile"))
		})

		It("should not sync to ElfMachine when ElfMachineTemplate virtual machine resources has not changed", func() {
			elfMachine.Spec = elfMachineTemplate.Spec.Template.Spec
			ctrlContext := newCtrlContexts(elfMachineTemplate, elfMachine)

			reconciler := &ElfMachineTemplateReconciler{ControllerContext: ctrlContext}
			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: capiutil.ObjectKey(elfMachineTemplate)})
			Expect(result).To(BeZero())
			Expect(err).To(BeNil())
			newElfMachine := &infrav1.ElfMachine{}
			Expect(reconciler.Client.Get(reconciler, capiutil.ObjectKey(elfMachine), newElfMachine)).To(Succeed())
			Expect(newElfMachine.Spec.NumCPUs).To(Equal(elfMachine.Spec.NumCPUs))
			Expect(newElfMachine.Spec.MemoryMiB).To(Equal(elfMachine.Spec.MemoryMiB))
		})

		It("should sync to ElfMachine when ElfMachineTemplate CPUs has been increased", func() {
			elfMachine.Spec = elfMachineTemplate.Spec.Template.Spec
			elfMachineTemplate.Spec.Template.Spec.NumCPUs += 1
			ctrlContext := newCtrlContexts(elfMachineTemplate, elfMachine)

			reconciler := &ElfMachineTemplateReconciler{ControllerContext: ctrlContext}
			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: capiutil.ObjectKey(elfMachineTemplate)})
			Expect(result).To(BeZero())
			Expect(err).To(BeNil())
			Expect(reconciler.Client.Get(reconciler, capiutil.ObjectKey(elfMachine), elfMachine)).To(Succeed())
			Expect(elfMachine.Spec.NumCPUs).To(Equal(elfMachineTemplate.Spec.Template.Spec.NumCPUs))
		})

		It("should sync to ElfMachine when ElfMachineTemplate memory has been increased", func() {
			elfMachine.Spec = elfMachineTemplate.Spec.Template.Spec
			elfMachineTemplate.Spec.Template.Spec.MemoryMiB += 1
			ctrlContext := newCtrlContexts(elfMachineTemplate, elfMachine)

			reconciler := &ElfMachineTemplateReconciler{ControllerContext: ctrlContext}
			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: capiutil.ObjectKey(elfMachineTemplate)})
			Expect(result).To(BeZero())
			Expect(err).To(BeNil())
			Expect(reconciler.Client.Get(reconciler, capiutil.ObjectKey(elfMachine), elfMachine)).To(Succeed())
			Expect(elfMachine.Spec.MemoryMiB).To(Equal(elfMachineTemplate.Spec.Template.Spec.MemoryMiB))
		})
	})
})
