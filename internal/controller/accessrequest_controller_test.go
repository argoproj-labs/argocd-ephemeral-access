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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/internal/controller/testdata"
	"github.com/argoproj-labs/ephemeral-access/test/utils"
)

var appprojectResource = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "appprojects",
}

var appResource = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "applications",
}

func newAccessRequest(name, namespace, appName, roleName, subject string) *api.AccessRequest {
	return &api.AccessRequest{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AccessRequest",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: api.AccessRequestSpec{
			Duration:         metav1.Duration{},
			RoleTemplateName: roleName,
			Application: api.TargetApplication{
				Name:      appName,
				Namespace: namespace,
			},
			Subjects: []api.Subject{
				{
					Username: subject,
				},
			},
		},
	}
}

var _ = Describe("AccessRequest Controller", func() {
	const (
		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	type fixture struct {
		accessrequest *api.AccessRequest
		appproj       *unstructured.Unstructured
	}

	type resources struct {
		arName, appName, namespace, appProjName, roleName, subject string
	}

	setup := func(r resources) *fixture {
		By("Create the Application initial state")
		appYaml := testdata.ApplicationYaml
		app, err := utils.YamlToUnstructured(appYaml)
		Expect(err).NotTo(HaveOccurred())
		app.SetName(r.appName)
		app.SetNamespace(r.namespace)
		unstructured.SetNestedField(app.Object, r.appProjName, "spec", "project")
		_, err = dynClient.Resource(appResource).
			Namespace(r.namespace).
			Apply(ctx, r.appName, app, metav1.ApplyOptions{
				FieldManager: "argocd-controller",
			})
		Expect(err).NotTo(HaveOccurred())

		By("Create the AppProject initial state")
		appprojYaml := testdata.AppProjectYaml
		appproj, err := utils.YamlToUnstructured(appprojYaml)
		appproj.SetName(r.appProjName)
		appproj.SetNamespace(r.namespace)
		Expect(err).NotTo(HaveOccurred())
		_, err = dynClient.Resource(appprojectResource).
			Namespace(r.namespace).
			Apply(ctx, r.appProjName, appproj, metav1.ApplyOptions{
				FieldManager: "argocd-controller",
			})
		Expect(err).NotTo(HaveOccurred())

		ar := newAccessRequest(r.arName, r.namespace, r.appName, r.roleName, r.subject)

		return &fixture{
			accessrequest: ar,
			appproj:       appproj,
		}
	}

	tearDown := func(r resources, f *fixture) {
		By("Delete the AccessRequest in k8s")
		Expect(k8sClient.Delete(ctx, f.accessrequest)).To(Succeed())

		By("Delete the AppProject in k8s")
		Expect(dynClient.Resource(appprojectResource).
			Namespace(r.namespace).
			Delete(ctx, r.appProjName, metav1.DeleteOptions{})).
			To(Succeed())
	}

	Context("Reconciling an AccessRequest", Ordered, func() {
		const (
			namespace      = "default"
			arName         = "test-ar-01"
			appprojectName = "sample-test-project"
			appName        = "some-application"
			roleName       = "super-user"
			subject        = "some-user"
		)

		var f *fixture
		var r resources

		When("The subject has the necessary access", func() {
			AfterAll(func() {
				tearDown(r, f)
			})
			BeforeAll(func() {
				r = resources{
					arName:      arName,
					appName:     appName,
					namespace:   namespace,
					appProjName: appprojectName,
					roleName:    roleName,
					subject:     subject,
				}
				f = setup(r)
			})
			It("will applies the access request resource in k8s", func() {
				f.accessrequest.Spec.Duration = metav1.Duration{Duration: time.Second * 5}
				err := k8sClient.Create(ctx, f.accessrequest)
				Expect(err).NotTo(HaveOccurred())
			})
			It("will verify if the access request is created", func() {
				ar := &api.AccessRequest{}
				Eventually(func() api.Status {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(f.accessrequest), ar)
					Expect(err).NotTo(HaveOccurred())
					return ar.Status.RequestState
				}, timeout, interval).ShouldNot(BeEmpty())
				Expect(ar.Status.History).NotTo(BeEmpty())
				Expect(ar.Status.History[0].RequestState).To(Equal(api.RequestedStatus))
			})
			It("will validate if the access is eventually granted", func() {
				ar := &api.AccessRequest{}
				Eventually(func() api.Status {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(f.accessrequest), ar)
					Expect(err).NotTo(HaveOccurred())
					return ar.Status.RequestState
				}, timeout, interval).Should(Equal(api.GrantedStatus))
				Expect(ar.Status.ExpiresAt).NotTo(BeNil())
				Expect(ar.Status.History).Should(HaveLen(2))
				Expect(ar.Status.History[0].RequestState).To(Equal(api.RequestedStatus))
				Expect(ar.Status.History[1].RequestState).To(Equal(api.GrantedStatus))
			})
			It("will validate if subject is added in Argo CD role", func() {
				appProj, err := dynClient.Resource(appprojectResource).
					Namespace(namespace).Get(ctx, appprojectName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(appProj).NotTo(BeNil())
				roles, found, err := unstructured.NestedSlice(appProj.Object, "spec", "roles")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(roles).To(HaveLen(3))
				role := roles[2]
				roleObj := role.(map[string]interface{})
				subjects, found, err := unstructured.NestedSlice(roleObj, "groups")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(subjects).To(HaveLen(1))
				Expect(subjects[0]).To(Equal(subject))
			})
			It("will validate if the final status is Expired", func() {
				ar := &api.AccessRequest{}
				Eventually(func() api.Status {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(f.accessrequest), ar)
					Expect(err).NotTo(HaveOccurred())
					return ar.Status.RequestState
				}, timeout, interval).Should(Equal(api.ExpiredStatus))
				Expect(ar.Status.History).Should(HaveLen(3))
				Expect(ar.Status.History[0].RequestState).To(Equal(api.RequestedStatus))
				Expect(ar.Status.History[1].RequestState).To(Equal(api.GrantedStatus))
				Expect(ar.Status.History[2].RequestState).To(Equal(api.ExpiredStatus))
			})
			It("will validate if subject is removed from Argo CD role", func() {
				appProj, err := dynClient.Resource(appprojectResource).
					Namespace(namespace).Get(ctx, appprojectName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(appProj).NotTo(BeNil())
				roles, found, err := unstructured.NestedSlice(appProj.Object, "spec", "roles")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(roles).To(HaveLen(3))
				role := roles[2]
				roleObj := role.(map[string]interface{})
				subjects, found, err := unstructured.NestedSlice(roleObj, "groups")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(subjects).To(HaveLen(0))
			})
		})
	})

	Context("Reconciling an AccessRequest", Ordered, func() {
		const (
			namespace      = "default"
			arName         = "test-ar-02"
			appprojectName = "sample-test-project-02"
			appName        = "some-application"
			roleName       = "super-user"
			subject        = "some-user"
		)

		var f *fixture
		var r resources

		When("protected fields values change after applied", func() {
			AfterAll(func() {
				tearDown(r, f)
			})
			BeforeAll(func() {
				r = resources{
					arName:      arName,
					appName:     appName,
					namespace:   namespace,
					appProjName: appprojectName,
					roleName:    roleName,
					subject:     subject,
				}
				f = setup(r)
			})
			It("will apply the access request resource in k8s", func() {
				f.accessrequest.Spec.Duration = metav1.Duration{Duration: time.Second * 5}
				err := k8sClient.Create(ctx, f.accessrequest)
				Expect(err).NotTo(HaveOccurred())
			})
			It("will verify if the access request is created", func() {
				ar := &api.AccessRequest{}
				Eventually(func() api.Status {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(f.accessrequest), ar)
					Expect(err).NotTo(HaveOccurred())
					return ar.Status.RequestState
				}, timeout, interval).ShouldNot(BeEmpty())
				Expect(ar.Status.History).NotTo(BeEmpty())
				Expect(ar.Status.History[0].RequestState).To(Equal(api.RequestedStatus))
			})
			It("will return immutable error on attempt to change the target role", func() {
				ar := &api.AccessRequest{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(f.accessrequest), ar)
				Expect(err).NotTo(HaveOccurred())
				ar.Spec.RoleTemplateName = "NOT-ALLOWED"

				err = k8sClient.Update(ctx, ar)

				Expect(err).ToNot(BeNil())
				e, ok := err.(*errors.StatusError)
				Expect(ok).To(BeTrue(), "returned error type is not errors.StatusError")
				Expect(string(e.ErrStatus.Reason)).To(Equal("Invalid"))
				Expect(e.ErrStatus.Message).To(ContainSubstring("Value is immutable"))
			})
			It("will return immutable error on attempt to change the Application", func() {
				ar := &api.AccessRequest{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(f.accessrequest), ar)
				Expect(err).NotTo(HaveOccurred())
				ar.Spec.Application.Name = "NOT-ALLOWED"
				ar.Spec.Application.Namespace = "NOT-ALLOWED"

				err = k8sClient.Update(ctx, ar)

				Expect(err).ToNot(BeNil())
				e, ok := err.(*errors.StatusError)
				Expect(ok).To(BeTrue(), "returned error type is not errors.StatusError")
				Expect(string(e.ErrStatus.Reason)).To(Equal("Invalid"))
				Expect(e.ErrStatus.Message).To(ContainSubstring("Value is immutable"))
			})
			It("will return immutable error on attempt to change the Subject", func() {
				ar := &api.AccessRequest{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(f.accessrequest), ar)
				Expect(err).NotTo(HaveOccurred())
				ar.Spec.Subjects[0].Username = "NOT-ALLOWED"

				err = k8sClient.Update(ctx, ar)

				Expect(err).ToNot(BeNil())
				e, ok := err.(*errors.StatusError)
				Expect(ok).To(BeTrue(), "returned error type is not errors.StatusError")
				Expect(string(e.ErrStatus.Reason)).To(Equal("Invalid"))
				Expect(e.ErrStatus.Message).To(ContainSubstring("Value is immutable"))
			})
		})
	})
})
