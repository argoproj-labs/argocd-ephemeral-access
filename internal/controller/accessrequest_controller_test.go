/*
Copyright 2024.

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
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ephemeralaccessv1alpha1 "github.com/argoproj-labs/ephemeral-access/api/v1alpha1"
)

var _ = Describe("AccessRequest Controller", func() {
	const (
		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		// typeNamespacedName := types.NamespacedName{
		// 	Name:      resourceName,
		// 	Namespace: "default",
		// }
		accessrequest := &ephemeralaccessv1alpha1.AccessRequest{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind AccessRequest")
			accessrequest = &ephemeralaccessv1alpha1.AccessRequest{
				TypeMeta: metav1.TypeMeta{
					Kind:       "",
					APIVersion: "",
				},
				ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: "default"},
				Spec: ephemeralaccessv1alpha1.AccessRequestSpec{
					Duration:       metav1.Duration{},
					TargetRoleName: "",
					Application:    ephemeralaccessv1alpha1.TargetApplication{},
					Subjects:       []ephemeralaccessv1alpha1.Subject{},
				},
			}
		})

		AfterEach(func() {
			By("Cleanup the specific resource instance AccessRequest")
			Expect(k8sClient.Delete(ctx, accessrequest)).To(Succeed())
		})
		It("Should set the status successfully ", func() {
			By("Reconciling the created resource")
			err := k8sClient.Create(ctx, accessrequest)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
