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

	argocd "github.com/argoproj-labs/ephemeral-access/api/argoproj/v1alpha1"
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

func newRoleTemplate(templateName, namespace, roleName string, policies []string) *api.RoleTemplate {
	return &api.RoleTemplate{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleTemplate",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      templateName,
			Namespace: namespace,
		},
		Spec: api.RoleTemplateSpec{
			Name:        roleName,
			Description: "",
			Policies:    policies,
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
		roletemplate  *api.RoleTemplate
		appproj       *unstructured.Unstructured
	}

	type resources struct {
		arName, appName, namespace, appProjName, roleTemplateName, subject, roleName string
		policies                                                                     []string
	}

	setup := func(r resources) *fixture {
		By("Create the Application initial state")
		appYaml := testdata.ApplicationYaml
		app, err := utils.YamlToUnstructured(appYaml)
		Expect(err).NotTo(HaveOccurred())
		app.SetName(r.appName)
		app.SetNamespace(r.namespace)
		unstructured.SetNestedField(app.Object, r.appProjName, "spec", "project")
		// The dynamic client is used to create the resource with all fields
		// defined in the official Argo CD CRD
		_, err = dynClient.Resource(appResource).
			Namespace(r.namespace).
			Apply(ctx, r.appName, app, metav1.ApplyOptions{
				FieldManager: "argocd-controller",
			})
		Expect(err).NotTo(HaveOccurred())

		By("Create the AppProject initial state")
		appprojYaml := testdata.AppProjectYaml
		appproj, err := utils.YamlToUnstructured(appprojYaml)
		Expect(err).NotTo(HaveOccurred())
		appproj.SetName(r.appProjName)
		appproj.SetNamespace(r.namespace)
		_, err = dynClient.Resource(appprojectResource).
			Namespace(r.namespace).
			Apply(ctx, r.appProjName, appproj, metav1.ApplyOptions{
				FieldManager: "argocd-controller",
			})
		Expect(err).NotTo(HaveOccurred())

		By("Create the RoleTemplate initial state")
		ar := newAccessRequest(r.arName, r.namespace, r.appName, r.roleTemplateName, r.subject)
		rt := newRoleTemplate(r.roleTemplateName, r.namespace, r.roleName, r.policies)

		return &fixture{
			accessrequest: ar,
			roletemplate:  rt,
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
			namespace        = "default"
			arName           = "test-ar-01"
			appprojectName   = "sample-test-project"
			appName          = "some-application"
			roleTemplateName = "some-role-template"
			roleName         = "super-user"
			subject          = "some-user"
		)

		var f *fixture
		var r resources
		policies := []string{
			"p, {{.Role}}, applications, sync, {{.Project}}/{{.Application}}, allow",
			"p, {{.Role}}, applications, action/*, {{.Project}}/{{.Application}}, allow",
			"p, {{.Role}}, applications, delete/*/Pod/*, {{.Project}}/{{.Application}}, allow",
			"p, {{.Role}}, logs, get, {{.Project}}/{{.Namespace}}/{{.Application}}, allow",
		}

		When("The subject has the necessary access", func() {
			AfterAll(func() {
				tearDown(r, f)
			})
			BeforeAll(func() {
				r = resources{
					arName:           arName,
					appName:          appName,
					namespace:        namespace,
					appProjName:      appprojectName,
					roleTemplateName: roleTemplateName,
					roleName:         roleName,
					subject:          subject,
					policies:         policies,
				}
				f = setup(r)
			})
			It("will apply the roletemplate resource in k8s", func() {
				err := k8sClient.Create(ctx, f.roletemplate)
				Expect(err).NotTo(HaveOccurred())
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
			It("will validate Argo CD AppProject", func() {
				key := client.ObjectKey{
					Namespace: namespace,
					Name:      appprojectName,
				}
				appProj := &argocd.AppProject{}
				err := k8sClient.Get(ctx, key, appProj)

				By("checking if subject is added in Argo CD role")
				Expect(err).NotTo(HaveOccurred())
				Expect(appProj).NotTo(BeNil())
				Expect(appProj.Spec.Roles).To(HaveLen(3))
				Expect(appProj.Spec.Roles[2].Groups).To(HaveLen(1))
				Expect(appProj.Spec.Roles[2].Groups[0]).To(Equal(subject))

				By("checking if roles are properly rendered from templates")
				Expect(appProj.Spec.Roles[2].Policies).To(HaveLen(4))
				expectedPolicy1 := "p, proj:sample-test-project:ephemeral-super-user-default-some-application, applications, sync, sample-test-project/some-application, allow"
				Expect(appProj.Spec.Roles[2].Policies[0]).To(Equal(expectedPolicy1))
				expectedPolicy2 := "p, proj:sample-test-project:ephemeral-super-user-default-some-application, applications, action/*, sample-test-project/some-application, allow"
				Expect(appProj.Spec.Roles[2].Policies[1]).To(Equal(expectedPolicy2))
				expectedPolicy3 := "p, proj:sample-test-project:ephemeral-super-user-default-some-application, applications, delete/*/Pod/*, sample-test-project/some-application, allow"
				Expect(appProj.Spec.Roles[2].Policies[2]).To(Equal(expectedPolicy3))
				expectedPolicy4 := "p, proj:sample-test-project:ephemeral-super-user-default-some-application, logs, get, sample-test-project/default/some-application, allow"
				Expect(appProj.Spec.Roles[2].Policies[3]).To(Equal(expectedPolicy4))
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
				key := client.ObjectKey{
					Namespace: namespace,
					Name:      appprojectName,
				}
				appProj := &argocd.AppProject{}
				err := k8sClient.Get(ctx, key, appProj)
				Expect(err).NotTo(HaveOccurred())
				Expect(appProj).NotTo(BeNil())
				Expect(appProj.Spec.Roles).To(HaveLen(3))
				Expect(appProj.Spec.Roles[2].Groups).To(HaveLen(0))
			})
		})
	})

	Context("Reconciling an AccessRequest", Ordered, func() {
		const (
			namespace        = "default"
			arName           = "test-ar-02"
			appprojectName   = "sample-test-project-02"
			appName          = "some-application"
			roleTemplateName = "some-role-template"
			roleName         = "super-user"
			subject          = "some-user"
		)

		var f *fixture
		var r resources
		policies := []string{
			"p, {{.Role}}, applications, sync, {{.Project}}/{{.Application}}, allow",
			"p, {{.Role}}, applications, action/*, {{.Project}}/{{.Application}}, allow",
			"p, {{.Role}}, applications, delete/*/Pod/*, {{.Project}}/{{.Application}}, allow",
			"p, {{.Role}}, logs, get, {{.Project}}/{{.Namespace}}/{{.Application}}, allow",
		}

		When("protected fields values change after applied", func() {
			AfterAll(func() {
				tearDown(r, f)
			})
			BeforeAll(func() {
				r = resources{
					arName:           arName,
					appName:          appName,
					namespace:        namespace,
					appProjName:      appprojectName,
					roleTemplateName: roleTemplateName,
					roleName:         roleName,
					subject:          subject,
					policies:         policies,
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
	Context("Changing a RoleTemplate used by multiple AccessRequests", Ordered, func() {
	})
	Context("Deleting RoleTemplate used by multiple AccessRequests", Ordered, func() {
	})
})
