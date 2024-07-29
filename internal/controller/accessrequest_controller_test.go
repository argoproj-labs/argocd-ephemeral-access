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
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/argoproj-labs/ephemeral-access/api/v1alpha1"
)

var _ = Describe("AccessRequest Controller", func() {
	const (
		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)
	Context("Reconciling a resource", Ordered, func() {
		const resourceName = "test-resource"

		ctx := context.Background()
		accessrequest := &api.AccessRequest{}

		When("Creating an AccessRequest", func() {
			BeforeEach(func() {
				By("creating the custom resource for the Kind AccessRequest")
				accessrequest = &api.AccessRequest{
					TypeMeta: metav1.TypeMeta{
						Kind:       "",
						APIVersion: "",
					},
					ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: "default"},
					Spec: api.AccessRequestSpec{
						Duration:       metav1.Duration{},
						TargetRoleName: "",
						Application:    api.TargetApplication{},
						Subjects:       []api.Subject{},
					},
				}
			})
			AfterAll(func() {
				By("Delete the AccessRequest in k8s")
				Expect(k8sClient.Delete(ctx, accessrequest)).To(Succeed())
			})
			It("Applies the resource in k8s", func() {
				accessrequest.Spec.Duration = metav1.Duration{Duration: time.Second * 5}
				err := k8sClient.Create(ctx, accessrequest)
				Expect(err).NotTo(HaveOccurred())
			})
			It("Verify if it is created", func() {
				Eventually(func() api.Status {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(accessrequest), accessrequest)
					Expect(err).NotTo(HaveOccurred())
					return accessrequest.Status.RequestState
				}, timeout, interval).ShouldNot(BeEmpty())
				Expect(accessrequest.Status.History).NotTo(BeEmpty())
				Expect(accessrequest.Status.History[0].RequestState).To(Equal(api.RequestedStatus))
			})
			It("Checks if the intermediate status is Granted", func() {
				Eventually(func() api.Status {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(accessrequest), accessrequest)
					Expect(err).NotTo(HaveOccurred())
					return accessrequest.Status.RequestState
				}, timeout, interval).Should(Equal(api.GrantedStatus))
				Expect(accessrequest.Status.ExpiresAt).NotTo(BeNil())
				Expect(accessrequest.Status.History).Should(HaveLen(2))
				Expect(accessrequest.Status.History[0].RequestState).To(Equal(api.RequestedStatus))
				Expect(accessrequest.Status.History[1].RequestState).To(Equal(api.GrantedStatus))
			})
			It("Checks if the final status is Expired", func() {
				Eventually(func() api.Status {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(accessrequest), accessrequest)
					Expect(err).NotTo(HaveOccurred())
					return accessrequest.Status.RequestState
				}, timeout, interval).Should(Equal(api.ExpiredStatus))
				Expect(accessrequest.Status.History).Should(HaveLen(3))
				Expect(accessrequest.Status.History[0].RequestState).To(Equal(api.RequestedStatus))
				Expect(accessrequest.Status.History[1].RequestState).To(Equal(api.GrantedStatus))
				Expect(accessrequest.Status.History[2].RequestState).To(Equal(api.ExpiredStatus))
			})
		})
	})
})
