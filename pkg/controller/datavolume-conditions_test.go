/*
Copyright 2020 The CDI Authors.

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

package controller

import (
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
)

var _ = Describe("findConditionByType", func() {
	It("should locate the right condition by type", func() {
		conditions := make([]cdiv1.DataVolumeCondition, 0)
		readyCondition := cdiv1.DataVolumeCondition{
			Type: cdiv1.DataVolumeReady,
		}
		conditions = append(conditions, readyCondition)
		runningCondition := cdiv1.DataVolumeCondition{
			Type: cdiv1.DataVolumeRunning,
		}
		conditions = append(conditions, runningCondition)
		boundCondition := cdiv1.DataVolumeCondition{
			Type: cdiv1.DataVolumeBound,
		}
		conditions = append(conditions, boundCondition)

		Expect(*findConditionByType(cdiv1.DataVolumeReady, conditions)).To(Equal(readyCondition))
		Expect(*findConditionByType(cdiv1.DataVolumeRunning, conditions)).To(Equal(runningCondition))
		Expect(*findConditionByType(cdiv1.DataVolumeBound, conditions)).To(Equal(boundCondition))
	})
})

var _ = Describe("updateRunningCondition", func() {
	It("should create condition if it doesn't exist", func() {
		conditions := make([]cdiv1.DataVolumeCondition, 0)
		conditions = updateRunningCondition(conditions, make(map[string]string))
		Expect(len(conditions)).To(Equal(1))
		Expect(conditions[0].Type).To(Equal(cdiv1.DataVolumeRunning))
		Expect(conditions[0].Message).To(BeEmpty())
		Expect(conditions[0].Status).To(Equal(corev1.ConditionFalse))
		Expect(conditions[0].Reason).To(Equal(""))
	})

	It("should have empty message if annotation is empty", func() {
		conditions := make([]cdiv1.DataVolumeCondition, 0)
		conditions = updateRunningCondition(conditions, map[string]string{AnnRunningConditionMessage: ""})
		Expect(len(conditions)).To(Equal(1))
		Expect(conditions[0].Type).To(Equal(cdiv1.DataVolumeRunning))
		Expect(conditions[0].Message).To(BeEmpty())
		Expect(conditions[0].Status).To(Equal(corev1.ConditionFalse))
		Expect(conditions[0].Reason).To(Equal(""))
	})

	It("should properly escape message from annotation", func() {
		conditions := make([]cdiv1.DataVolumeCondition, 0)
		conditions = updateRunningCondition(conditions, map[string]string{AnnRunningConditionMessage: "this is a message with quotes \"", AnnRunningConditionReason: "this is a \" reason with \" quotes"})
		Expect(len(conditions)).To(Equal(1))
		Expect(conditions[0].Type).To(Equal(cdiv1.DataVolumeRunning))
		Expect(conditions[0].Message).To(Equal("this is a message with quotes \""))
		Expect(conditions[0].Status).To(Equal(corev1.ConditionFalse))
		Expect(conditions[0].Reason).To(Equal("this is a \" reason with \" quotes"))
	})

	table.DescribeTable("runningCondition", func(conditionString string, status corev1.ConditionStatus, noAnnotation bool) {
		conditions := make([]cdiv1.DataVolumeCondition, 0)
		if noAnnotation {
			conditions = updateRunningCondition(conditions, map[string]string{})
		} else {
			conditions = updateRunningCondition(conditions, map[string]string{AnnRunningCondition: conditionString})
		}
		condition := findConditionByType(cdiv1.DataVolumeRunning, conditions)
		Expect(condition.Type).To(Equal(cdiv1.DataVolumeRunning))
		Expect(condition.Message).To(BeEmpty())
		Expect(condition.Status).To(Equal(status))
	},
		table.Entry("condition true", "true", corev1.ConditionTrue, false),
		table.Entry("condition false", "false", corev1.ConditionFalse, false),
		table.Entry("condition invalid", "invalid", corev1.ConditionUnknown, false),
		table.Entry("no condition", "", corev1.ConditionFalse, true),
	)

	table.DescribeTable("runningConditionAndsource", func(conditionString, message, reason, sourceConditionString, sourceConditionMessage, sourceConditionReason string, status corev1.ConditionStatus, expectedMessage, expectedReason string) {
		conditions := make([]cdiv1.DataVolumeCondition, 0)
		if sourceConditionString != "" {
			conditions = updateRunningCondition(conditions, map[string]string{AnnRunningCondition: conditionString, AnnRunningConditionMessage: message, AnnRunningConditionReason: reason, AnnSourceRunningCondition: sourceConditionString, AnnSourceRunningConditionMessage: sourceConditionMessage, AnnSourceRunningConditionReason: sourceConditionReason})
		} else {
			conditions = updateRunningCondition(conditions, map[string]string{AnnRunningCondition: conditionString, AnnRunningConditionMessage: message, AnnRunningConditionReason: reason})
		}
		condition := findConditionByType(cdiv1.DataVolumeRunning, conditions)
		Expect(condition.Type).To(Equal(cdiv1.DataVolumeRunning))
		Expect(condition.Message).To(Equal(expectedMessage))
		Expect(condition.Reason).To(Equal(expectedReason))
		Expect(condition.Status).To(Equal(status))
	},
		table.Entry("condition true, source true", "true", "", "", "true", "", "", corev1.ConditionTrue, "", ""),
		table.Entry("condition true, source false", "true", "", "", "false", "scratch creating", "Creating Scratch", corev1.ConditionFalse, "scratch creating", "Creating Scratch"),
		table.Entry("condition true, source unknown", "true", "", "", "invalid", "unknown message", "unknown reason", corev1.ConditionUnknown, "unknown message", "unknown reason"),
		table.Entry("condition true, no source", "true", "", "", "", "", "", corev1.ConditionTrue, "", ""),
		table.Entry("condition false, source true", "false", "Pod Pending", "Pending", "true", "", "", corev1.ConditionFalse, "Pod Pending", "Pending"),
		table.Entry("condition false, source false", "false", "Pod Pending", "Pending", "false", "Pod Pending", "Pending", corev1.ConditionFalse, "Pod Pending and Pod Pending", "Pending and Pending"),
		table.Entry("condition false, source unknown", "false", "Pod Pending", "Pending", "unknown", "unknown", "unknown", corev1.ConditionUnknown, "Pod Pending and unknown", "Pending and unknown"),
	)
})

var _ = Describe("updateReadyCondition", func() {
	It("should create condition if it doesn't exist", func() {
		conditions := make([]cdiv1.DataVolumeCondition, 0)
		conditions = updateReadyCondition(conditions, corev1.ConditionTrue, "message", "reason")
		Expect(len(conditions)).To(Equal(1))
		Expect(conditions[0].Type).To(Equal(cdiv1.DataVolumeReady))
		Expect(conditions[0].Message).To(Equal("message"))
		Expect(conditions[0].Reason).To(Equal("reason"))
		Expect(conditions[0].Status).To(Equal(corev1.ConditionTrue))
	})
})

var _ = Describe("updateBoundCondition", func() {
	It("should create condition if it doesn't exist", func() {
		conditions := make([]cdiv1.DataVolumeCondition, 0)
		conditions = updateBoundCondition(conditions, nil)
		Expect(len(conditions)).To(Equal(2))
		condition := findConditionByType(cdiv1.DataVolumeBound, conditions)
		Expect(condition.Type).To(Equal(cdiv1.DataVolumeBound))
		Expect(condition.Message).To(Equal("No PVC found"))
		Expect(condition.Reason).To(Equal(notFound))
		Expect(condition.Status).To(Equal(corev1.ConditionUnknown))
	})

	It("should be bound if PVC bound", func() {
		conditions := make([]cdiv1.DataVolumeCondition, 0)
		pvc := createPvc("test", corev1.NamespaceDefault, nil, nil)
		pvc.Status.Phase = corev1.ClaimBound
		conditions = updateBoundCondition(conditions, pvc)
		Expect(len(conditions)).To(Equal(1))
		condition := findConditionByType(cdiv1.DataVolumeBound, conditions)
		Expect(condition.Type).To(Equal(cdiv1.DataVolumeBound))
		Expect(condition.Message).To(Equal("PVC test Bound"))
		Expect(condition.Reason).To(Equal(pvcBound))
		Expect(condition.Status).To(Equal(corev1.ConditionTrue))
	})

	It("should be bound if PVC bound and other PVC is bound", func() {
		conditions := make([]cdiv1.DataVolumeCondition, 0)
		pvc := createPvc("test", corev1.NamespaceDefault, map[string]string{AnnBoundCondition: "true"}, nil)
		pvc.Status.Phase = corev1.ClaimBound
		conditions = updateBoundCondition(conditions, pvc)
		Expect(len(conditions)).To(Equal(1))
		condition := findConditionByType(cdiv1.DataVolumeBound, conditions)
		Expect(condition.Type).To(Equal(cdiv1.DataVolumeBound))
		Expect(condition.Message).To(Equal("PVC test Bound"))
		Expect(condition.Reason).To(Equal(pvcBound))
		Expect(condition.Status).To(Equal(corev1.ConditionTrue))
	})

	It("should be pending if PVC bound and other PVC is not bound", func() {
		conditions := make([]cdiv1.DataVolumeCondition, 0)
		pvc := createPvc("test", corev1.NamespaceDefault, map[string]string{AnnBoundCondition: "false", AnnBoundConditionReason: "not bound", AnnBoundConditionMessage: "scratch PVC not bound"}, nil)
		pvc.Status.Phase = corev1.ClaimBound
		conditions = updateBoundCondition(conditions, pvc)
		Expect(len(conditions)).To(Equal(2))
		condition := findConditionByType(cdiv1.DataVolumeBound, conditions)
		Expect(condition.Type).To(Equal(cdiv1.DataVolumeBound))
		Expect(condition.Message).To(Equal("scratch PVC not bound"))
		Expect(condition.Reason).To(Equal("not bound"))
		Expect(condition.Status).To(Equal(corev1.ConditionFalse))
		condition = findConditionByType(cdiv1.DataVolumeReady, conditions)
		Expect(condition.Type).To(Equal(cdiv1.DataVolumeReady))
		Expect(condition.Message).To(BeEmpty())
		Expect(condition.Reason).To(BeEmpty())
		Expect(condition.Status).To(Equal(corev1.ConditionFalse))
	})

	It("should be pending if PVC pending", func() {
		conditions := make([]cdiv1.DataVolumeCondition, 0)
		pvc := createPvc("test", corev1.NamespaceDefault, nil, nil)
		pvc.Status.Phase = corev1.ClaimPending
		conditions = updateBoundCondition(conditions, pvc)
		Expect(len(conditions)).To(Equal(2))
		condition := findConditionByType(cdiv1.DataVolumeBound, conditions)
		Expect(condition.Type).To(Equal(cdiv1.DataVolumeBound))
		Expect(condition.Message).To(Equal("PVC test Pending"))
		Expect(condition.Reason).To(Equal(pvcPending))
		Expect(condition.Status).To(Equal(corev1.ConditionFalse))
		condition = findConditionByType(cdiv1.DataVolumeReady, conditions)
		Expect(condition.Type).To(Equal(cdiv1.DataVolumeReady))
		Expect(condition.Message).To(BeEmpty())
		Expect(condition.Reason).To(BeEmpty())
		Expect(condition.Status).To(Equal(corev1.ConditionFalse))
	})

	It("should be pending if PVC pending, even if scratch PVC is bound", func() {
		conditions := make([]cdiv1.DataVolumeCondition, 0)
		pvc := createPvc("test", corev1.NamespaceDefault, map[string]string{AnnBoundCondition: "true"}, nil)
		pvc.Status.Phase = corev1.ClaimPending
		conditions = updateBoundCondition(conditions, pvc)
		Expect(len(conditions)).To(Equal(2))
		condition := findConditionByType(cdiv1.DataVolumeBound, conditions)
		Expect(condition.Type).To(Equal(cdiv1.DataVolumeBound))
		Expect(condition.Message).To(Equal("PVC test Pending"))
		Expect(condition.Reason).To(Equal(pvcPending))
		Expect(condition.Status).To(Equal(corev1.ConditionFalse))
		condition = findConditionByType(cdiv1.DataVolumeReady, conditions)
		Expect(condition.Type).To(Equal(cdiv1.DataVolumeReady))
		Expect(condition.Message).To(BeEmpty())
		Expect(condition.Reason).To(BeEmpty())
		Expect(condition.Status).To(Equal(corev1.ConditionFalse))
	})

	It("should be pending if PVC pending, if scratch PVC is not bound, message should be combined", func() {
		reason := "not bound"
		conditions := make([]cdiv1.DataVolumeCondition, 0)
		pvc := createPvc("test", corev1.NamespaceDefault, map[string]string{AnnBoundCondition: "false", AnnBoundConditionReason: reason, AnnBoundConditionMessage: "scratch PVC not bound"}, nil)
		pvc.Status.Phase = corev1.ClaimPending
		conditions = updateBoundCondition(conditions, pvc)
		Expect(len(conditions)).To(Equal(2))
		condition := findConditionByType(cdiv1.DataVolumeBound, conditions)
		Expect(condition.Type).To(Equal(cdiv1.DataVolumeBound))
		Expect(condition.Message).To(Equal("target PVC test Pending and scratch PVC not bound"))
		Expect(condition.Reason).To(Equal(reason))
		Expect(condition.Status).To(Equal(corev1.ConditionFalse))
		condition = findConditionByType(cdiv1.DataVolumeReady, conditions)
		Expect(condition.Type).To(Equal(cdiv1.DataVolumeReady))
		Expect(condition.Message).To(BeEmpty())
		Expect(condition.Reason).To(BeEmpty())
		Expect(condition.Status).To(Equal(corev1.ConditionFalse))
	})

	It("should be lost if PVC lost", func() {
		conditions := make([]cdiv1.DataVolumeCondition, 0)
		pvc := createPvc("test", corev1.NamespaceDefault, nil, nil)
		pvc.Status.Phase = corev1.ClaimLost
		conditions = updateBoundCondition(conditions, pvc)
		Expect(len(conditions)).To(Equal(2))
		condition := findConditionByType(cdiv1.DataVolumeBound, conditions)
		Expect(condition.Type).To(Equal(cdiv1.DataVolumeBound))
		Expect(condition.Message).To(Equal("Claim Lost"))
		Expect(condition.Reason).To(Equal(claimLost))
		Expect(condition.Status).To(Equal(corev1.ConditionFalse))
	})
})
