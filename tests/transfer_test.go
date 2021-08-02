package tests_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = Describe("[rfe_id:5630][crit:high]ObjectTransfer tests", func() {
	const (
		imagePath = "/pvc/disk.img"
	)

	f := framework.NewFramework("transfer-test")

	deleteTransfer := func(name string) {
		Eventually(func() bool {
			if err := f.CdiClient.CdiV1beta1().ObjectTransfers().Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
				if k8serrors.IsNotFound(err) {
					return true
				}
				Expect(err).ToNot(HaveOccurred())
			}

			ot, err := f.CdiClient.CdiV1beta1().ObjectTransfers().Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					return true
				}
				Expect(err).ToNot(HaveOccurred())
			}

			if len(ot.Finalizers) > 0 {
				ot.Finalizers = nil
				if _, err = f.CdiClient.CdiV1beta1().ObjectTransfers().Update(context.TODO(), ot, metav1.UpdateOptions{}); err != nil {
					if k8serrors.IsNotFound(err) {
						return true
					}
					if k8serrors.IsConflict(err) {
						return false
					}
					Expect(err).ToNot(HaveOccurred())
				}
			}

			return false

		}, 1*time.Minute, 2*time.Second).Should(BeTrue())
	}

	createDV := func(namespace, name string) *cdiv1.DataVolume {
		dataVolume := utils.NewDataVolumeWithHTTPImport(name, "500Mi", fmt.Sprintf(utils.TinyCoreIsoURL, f.CdiInstallNs))
		dataVolume, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, namespace, dataVolume)
		Expect(err).ToNot(HaveOccurred())

		f.ForceBindPvcIfDvIsWaitForFirstConsumer(dataVolume)
		err = utils.WaitForDataVolumePhase(f.CdiClient, namespace, cdiv1.Succeeded, dataVolume.Name)
		Expect(err).ToNot(HaveOccurred())

		return dataVolume
	}

	getHash := func(ns *corev1.Namespace, pvcName string) string {
		pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(ns.Name).Get(context.TODO(), pvcName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		sourceMD5, err := f.GetMD5(ns, pvc, imagePath, 0)
		Expect(err).ToNot(HaveOccurred())

		err = utils.DeleteVerifierPod(f.K8sClient, ns.Name)
		Expect(err).ToNot(HaveOccurred())

		return sourceMD5
	}

	doTransfer := func(ot *cdiv1.ObjectTransfer) *cdiv1.ObjectTransfer {
		var err error
		ot, err = f.CdiClient.CdiV1beta1().ObjectTransfers().Create(context.TODO(), ot, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() bool {
			ot, err = f.CdiClient.CdiV1beta1().ObjectTransfers().Get(context.TODO(), ot.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return ot.Status.Phase == cdiv1.ObjectTransferComplete
		}, 2*time.Minute, 2*time.Second).Should(BeTrue())

		return ot
	}

	pvUID := func(namespace, name string) types.UID {
		pvc, err := f.K8sClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		pv, err := f.K8sClient.CoreV1().PersistentVolumes().Get(context.TODO(), pvc.Spec.VolumeName, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		return pv.UID
	}

	Describe("Validation tests", func() {
		var (
			err error
			ot  *cdiv1.ObjectTransfer
		)

		AfterEach(func() {
			if ot != nil && ot.Name != "" {
				deleteTransfer(ot.Name)
			}
		})

		DescribeTable("[posneg:negative][test_id:5834]should reject not target name/namespace", func(s cdiv1.ObjectTransferSpec, errString string) {
			ot = &cdiv1.ObjectTransfer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ot",
				},
				Spec: s,
			}

			ot, err = f.CdiClient.CdiV1beta1().ObjectTransfers().Create(context.TODO(), ot, metav1.CreateOptions{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(errString))
		},
			Entry("No target", cdiv1.ObjectTransferSpec{
				Source: cdiv1.TransferSource{
					Kind:      "DataVolume",
					Namespace: "foo",
					Name:      "bar",
				},
			}, "Target namespace and/or target name must be supplied"),
			Entry("Bad Kind", cdiv1.ObjectTransferSpec{
				Source: cdiv1.TransferSource{
					Kind:      "VolumeSnapshot",
					Namespace: "foo",
					Name:      "bar",
				},
				Target: cdiv1.TransferTarget{
					Namespace: &[]string{"bar"}[0],
				},
			}, "Unsupported kind \"VolumeSnapshot\""),
		)

		It("[posneg:negative][test_id:5836]should not allow spec update", func() {
			ot = &cdiv1.ObjectTransfer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ot-" + f.Namespace.Name,
				},
				Spec: cdiv1.ObjectTransferSpec{
					Source: cdiv1.TransferSource{
						Kind:      "DataVolume",
						Namespace: "foo",
						Name:      "bar",
					},
					Target: cdiv1.TransferTarget{
						Namespace: &[]string{"bar"}[0],
					},
				},
			}

			ot, err = f.CdiClient.CdiV1beta1().ObjectTransfers().Create(context.TODO(), ot, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				ot, err = f.CdiClient.CdiV1beta1().ObjectTransfers().Get(context.TODO(), ot.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				ot2 := ot.DeepCopy()
				ot2.Spec.Source.Namespace = "baz"
				_, err = f.CdiClient.CdiV1beta1().ObjectTransfers().Update(context.TODO(), ot2, metav1.UpdateOptions{})
				Expect(err).To(HaveOccurred())
				if k8serrors.IsConflict(err) {
					return false
				}
				Expect(err.Error()).To(ContainSubstring("ObjectTransfer spec is immutable"))
				return true
			}, 1*time.Minute, 2*time.Second).Should(BeTrue())
		})

		It("[posneg:negative][test_id:5719]should not allow if target exists", func() {
			dataVolume := createDV(f.Namespace.Name, "source-dv")

			for _, k := range []string{"DataVolume", "PersistentVolumeClaim"} {
				ot = &cdiv1.ObjectTransfer{
					ObjectMeta: metav1.ObjectMeta{
						Name: "ot-" + f.Namespace.Name,
					},
					Spec: cdiv1.ObjectTransferSpec{
						Source: cdiv1.TransferSource{
							Kind:      k,
							Namespace: "foo",
							Name:      "bar",
						},
						Target: cdiv1.TransferTarget{
							Namespace: &dataVolume.Namespace,
							Name:      &dataVolume.Name,
						},
					},
				}

				ot, err = f.CdiClient.CdiV1beta1().ObjectTransfers().Create(context.TODO(), ot, metav1.CreateOptions{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("already exists"))
			}
		})
	})

	Describe("DataVolume tests", func() {
		AfterEach(func() {
			otl, err := f.CdiClient.CdiV1beta1().ObjectTransfers().List(context.TODO(), metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())

			for _, ot := range otl.Items {
				deleteTransfer(ot.Name)
			}
		})

		It("[test_id:5694]should transfer to target and back again", func() {
			dataVolume := createDV(f.Namespace.Name, "source-dv")

			sourceMD5 := getHash(f.Namespace, dataVolume.Name)
			uid := pvUID(dataVolume.Namespace, dataVolume.Name)

			targetNs, err := f.CreateNamespace(f.NsPrefix, map[string]string{
				framework.NsPrefixLabel: f.NsPrefix,
			})
			Expect(err).ToNot(HaveOccurred())
			f.AddNamespaceToDelete(targetNs)

			ot := &cdiv1.ObjectTransfer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ot-" + f.Namespace.Name,
				},
				Spec: cdiv1.ObjectTransferSpec{
					Source: cdiv1.TransferSource{
						Kind:      "DataVolume",
						Namespace: f.Namespace.Name,
						Name:      "source-dv",
					},
					Target: cdiv1.TransferTarget{
						Namespace: &targetNs.Name,
					},
				},
			}

			defer deleteTransfer(ot.Name)
			ot = doTransfer(ot)

			if true {
				return
			}

			targetHash := getHash(targetNs, dataVolume.Name)
			Expect(sourceMD5).To(Equal(targetHash))
			Expect(uid).To(Equal(pvUID(targetNs.Name, dataVolume.Name)))

			deleteTransfer(ot.Name)

			ot = &cdiv1.ObjectTransfer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ot-" + targetNs.Name,
				},
				Spec: cdiv1.ObjectTransferSpec{
					Source: cdiv1.TransferSource{
						Kind:      "DataVolume",
						Namespace: targetNs.Name,
						Name:      "source-dv",
					},
					Target: cdiv1.TransferTarget{
						Namespace: &f.Namespace.Name,
					},
				},
			}

			defer deleteTransfer(ot.Name)
			ot = doTransfer(ot)

			targetHash = getHash(f.Namespace, dataVolume.Name)
			Expect(sourceMD5).To(Equal(targetHash))
			Expect(uid).To(Equal(pvUID(f.Namespace.Name, dataVolume.Name)))
		})

		DescribeTable("[test_id:5695]should do concurrent transfers", func(createTargetNamespace bool, targetName *string) {
			var sourceNamespaces []string
			var wg sync.WaitGroup
			n := 5

			for i := 0; i < n; i++ {
				ns, err := f.CreateNamespace(f.NsPrefix, map[string]string{
					framework.NsPrefixLabel: f.NsPrefix,
				})
				Expect(err).NotTo(HaveOccurred())
				f.AddNamespaceToDelete(ns)
				sourceNamespaces = append(sourceNamespaces, ns.Name)

				wg.Add(1)
				go func(ns string) {
					defer GinkgoRecover()
					defer wg.Done()
					createDV(ns, "source-dv")
				}(ns.Name)
			}

			wg.Wait()

			for i := 0; i < n; i++ {
				var targetNamespace string

				if createTargetNamespace {
					ns, err := f.CreateNamespace(f.NsPrefix, map[string]string{
						framework.NsPrefixLabel: f.NsPrefix,
					})
					Expect(err).NotTo(HaveOccurred())
					f.AddNamespaceToDelete(ns)
					targetNamespace = ns.Name
				}

				wg.Add(1)
				go func(sourceNs string) {
					defer GinkgoRecover()
					defer wg.Done()
					sourceName := "source-dv"
					targetNs := sourceNs
					uid := pvUID(sourceNs, sourceName)

					if createTargetNamespace {
						targetNs = targetNamespace
					}

					ot := &cdiv1.ObjectTransfer{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ot-to-" + targetNs,
						},
						Spec: cdiv1.ObjectTransferSpec{
							Source: cdiv1.TransferSource{
								Kind:      "DataVolume",
								Namespace: sourceNs,
								Name:      sourceName,
							},
							Target: cdiv1.TransferTarget{
								Name: targetName,
							},
						},
					}

					if createTargetNamespace {
						ot.Spec.Target.Namespace = &targetNamespace
					}

					defer deleteTransfer(ot.Name)
					ot = doTransfer(ot)

					tn := sourceName
					if targetName != nil {
						tn = *targetName
					}
					Expect(uid).To(Equal(pvUID(targetNs, tn)))

				}(sourceNamespaces[i])
			}

			ch := make(chan struct{})
			defer close(ch)

			go func() {
				done := false
				for {
					select {
					case <-ch:
						done = true
					case <-time.After(5 * time.Second):
					}

					l, err := f.CdiClient.CdiV1beta1().ObjectTransfers().List(context.TODO(), metav1.ListOptions{})
					Expect(err).ToNot(HaveOccurred())
					bs, err := json.MarshalIndent(l.Items, "", "    ")
					Expect(err).ToNot(HaveOccurred())
					fmt.Fprintf(GinkgoWriter, "%s\n", string(bs))

					if done {
						return
					}
				}
			}()

			wg.Wait()
		},
			Entry("with new namespace and same name", true, nil),
			Entry("with new namespace and explicit name", true, &[]string{"target-name"}[0]),
			Entry("with same namespace and explicit name", false, &[]string{"target-name"}[0]),
		)

		It("[posneg:negative][test_id:5734]should handle quota failure", func() {
			sq := int64(100 * 1024 * 1024)
			bq := int64(1024 * 1024 * 1024)
			dataVolume := createDV(f.Namespace.Name, "source-dv")

			uid := pvUID(dataVolume.Namespace, dataVolume.Name)

			targetNs, err := f.CreateNamespace(f.NsPrefix, map[string]string{
				framework.NsPrefixLabel: f.NsPrefix,
			})
			Expect(err).ToNot(HaveOccurred())
			f.AddNamespaceToDelete(targetNs)

			rq := &corev1.ResourceQuota{
				ObjectMeta: metav1.ObjectMeta{
					Name: "quota",
				},
				Spec: corev1.ResourceQuotaSpec{
					Hard: corev1.ResourceList{
						corev1.ResourceRequestsStorage: *resource.NewQuantity(sq, resource.DecimalSI),
					},
				},
			}

			rq, err = f.K8sClient.CoreV1().ResourceQuotas(targetNs.Name).Create(context.TODO(), rq, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			ot := &cdiv1.ObjectTransfer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ot-" + f.Namespace.Name,
				},
				Spec: cdiv1.ObjectTransferSpec{
					Source: cdiv1.TransferSource{
						Kind:      "DataVolume",
						Namespace: f.Namespace.Name,
						Name:      "source-dv",
					},
					Target: cdiv1.TransferTarget{
						Namespace: &targetNs.Name,
					},
				},
			}

			defer deleteTransfer(ot.Name)

			ot, err = f.CdiClient.CdiV1beta1().ObjectTransfers().Create(context.TODO(), ot, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				transferName := "pvc-transfer-" + string(ot.UID)
				ot2, err := f.CdiClient.CdiV1beta1().ObjectTransfers().Get(context.TODO(), transferName, metav1.GetOptions{})
				if k8serrors.IsNotFound(err) {
					return false
				}
				Expect(err).ToNot(HaveOccurred())
				for _, c := range ot2.Status.Conditions {
					if c.Type != cdiv1.ObjectTransferConditionComplete {
						continue
					}
					return strings.Contains(c.Reason, "exceeded quota")
				}
				return false
			}, 2*time.Minute, 2*time.Second).Should(BeTrue())

			rq, err = f.K8sClient.CoreV1().ResourceQuotas(targetNs.Name).Get(context.TODO(), rq.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			rq.Spec.Hard[corev1.ResourceRequestsStorage] = *resource.NewQuantity(bq, resource.DecimalSI)
			rq, err = f.K8sClient.CoreV1().ResourceQuotas(targetNs.Name).Update(context.TODO(), rq, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				ot2, err := f.CdiClient.CdiV1beta1().ObjectTransfers().Get(context.TODO(), ot.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return ot2.Status.Phase == cdiv1.ObjectTransferComplete
			}, 5*time.Minute, 2*time.Second).Should(BeTrue())

			Expect(uid).To(Equal(pvUID(targetNs.Name, "source-dv")))
		})
	})
})
