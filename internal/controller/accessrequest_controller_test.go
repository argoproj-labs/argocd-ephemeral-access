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
	"errors"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	argocd "github.com/argoproj-labs/argocd-ephemeral-access/api/argoproj/v1alpha1"
	api "github.com/argoproj-labs/argocd-ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/argocd-ephemeral-access/internal/controller/testdata"
	"github.com/argoproj-labs/argocd-ephemeral-access/pkg/plugin"
	"github.com/argoproj-labs/argocd-ephemeral-access/test/utils"
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

var _ = Describe("AccessRequest Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	type fixture struct {
		namespace      *corev1.Namespace
		rtNamespace    *corev1.Namespace
		accessrequests []*api.AccessRequest
		roletemplate   *api.RoleTemplate
		appproj        *unstructured.Unstructured
	}

	type resources struct {
		arName, appName, namespace, appProjName,
		roleTemplateName, roleTemplateNamespace,
		subject, roleName string
		policies []string
	}

	setup := func(r resources) *fixture {
		By("Creating the namespace")
		ns := utils.NewNamespace(r.namespace)
		err := k8sClient.Create(ctx, ns)
		if err != nil {
			statusErr := &apierrors.StatusError{}
			ok := errors.As(err, &statusErr)
			Expect(ok).To(BeTrue())
			Expect(statusErr.ErrStatus.Code).NotTo(Equal(409))
		}

		By("Creating the roleTemplate namespace")
		rtNs := utils.NewNamespace(r.roleTemplateNamespace)
		err = k8sClient.Create(ctx, rtNs)
		if err != nil {
			statusErr := &apierrors.StatusError{}
			ok := errors.As(err, &statusErr)
			Expect(ok).To(BeTrue())
			Expect(statusErr.ErrStatus.Code).NotTo(Equal(409))
		}

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

		By("Instantiate the RoleTemplate initial state")
		rt := utils.NewRoleTemplate(r.roleTemplateName, r.roleTemplateNamespace, r.roleName, r.policies)
		By("Instantiate the AccessRequest initial state")
		ar := utils.NewAccessRequest(r.arName, r.namespace, r.appName, r.namespace, r.roleTemplateName, r.roleTemplateNamespace, r.subject)

		return &fixture{
			namespace:      ns,
			rtNamespace:    rtNs,
			accessrequests: []*api.AccessRequest{ar},
			roletemplate:   rt,
			appproj:        appproj,
		}
	}

	deleteNamespace := func(f *fixture) {
		By("Delete the test namespace")
		Expect(k8sClient.Delete(ctx, f.namespace)).To(Succeed())

		By("Delete the roletemplate namespace")
		Expect(k8sClient.Delete(ctx, f.rtNamespace)).To(Succeed())
	}

	Context("Reconciling an AccessRequest", Ordered, func() {
		When("The subject has the necessary access", func() {
			const (
				namespace             = "test-01"
				arName                = "test-ar-01"
				appprojectName        = "sample-test-project"
				appName               = "some-application"
				roleTemplateName      = "some-role-template"
				roleTemplateNamespace = "test-01-rt"
				roleName              = "super-user"
				subject               = "some-user"
			)

			var f *fixture
			var r resources
			policies := []string{
				"p, {{.role}}, applications, sync, {{.project}}/{{.application}}, allow",
				"p, {{.role}}, applications, action/*, {{.project}}/{{.application}}, allow",
				"p, {{.role}}, applications, delete/*/Pod/*, {{.project}}/{{.application}}, allow",
				"p, {{.role}}, logs, get, {{.project}}/{{.namespace}}/{{.application}}, allow",
			}

			AfterAll(func() {
				deleteNamespace(f)
			})
			BeforeAll(func() {
				r = resources{
					arName:                arName,
					appName:               appName,
					namespace:             namespace,
					appProjName:           appprojectName,
					roleTemplateName:      roleTemplateName,
					roleTemplateNamespace: roleTemplateNamespace,
					roleName:              roleName,
					subject:               subject,
					policies:              policies,
				}
				f = setup(r)
			})
			It("will apply the roletemplate resource in k8s", func() {
				err := k8sClient.Create(ctx, f.roletemplate)
				Expect(err).NotTo(HaveOccurred())
			})
			It("will apply the access request resource in k8s", func() {
				f.accessrequests[0].Spec.Duration = metav1.Duration{Duration: time.Second * 5}
				err := k8sClient.Create(ctx, f.accessrequests[0])
				Expect(err).NotTo(HaveOccurred())
			})
			It("will verify if the access request is created", func() {
				ar := &api.AccessRequest{}
				Eventually(func() api.Status {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(f.accessrequests[0]), ar)
					Expect(err).NotTo(HaveOccurred())
					return ar.Status.RequestState
				}, timeout, interval).ShouldNot(BeEmpty())
				Expect(ar.Status.History).NotTo(BeEmpty())
				Expect(ar.Status.History[0].RequestState).To(Equal(api.InitiatedStatus))
			})
			It("will validate if the access is eventually granted", func() {
				ar := &api.AccessRequest{}
				Eventually(func() api.Status {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(f.accessrequests[0]), ar)
					Expect(err).NotTo(HaveOccurred())
					return ar.Status.RequestState
				}, timeout, interval).Should(Equal(api.GrantedStatus))
				Expect(ar.Status.ExpiresAt).NotTo(BeNil())
				Expect(ar.Status.History).Should(HaveLen(2))
				Expect(ar.Status.History[0].RequestState).To(Equal(api.InitiatedStatus))
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
				expectedPolicy1 := fmt.Sprintf("p, proj:sample-test-project:ephemeral-super-user-%s-some-application, applications, sync, sample-test-project/some-application, allow", r.namespace)
				Expect(appProj.Spec.Roles[2].Policies[0]).To(Equal(expectedPolicy1))
				expectedPolicy2 := fmt.Sprintf("p, proj:sample-test-project:ephemeral-super-user-%s-some-application, applications, action/*, sample-test-project/some-application, allow", r.namespace)
				Expect(appProj.Spec.Roles[2].Policies[1]).To(Equal(expectedPolicy2))
				expectedPolicy3 := fmt.Sprintf("p, proj:sample-test-project:ephemeral-super-user-%s-some-application, applications, delete/*/Pod/*, sample-test-project/some-application, allow", r.namespace)
				Expect(appProj.Spec.Roles[2].Policies[2]).To(Equal(expectedPolicy3))
				expectedPolicy4 := fmt.Sprintf("p, proj:sample-test-project:ephemeral-super-user-%s-some-application, logs, get, sample-test-project/%s/some-application, allow", r.namespace, r.namespace)
				Expect(appProj.Spec.Roles[2].Policies[3]).To(Equal(expectedPolicy4))
			})
			It("will validate if the final status is eventually Expired", func() {
				ar := &api.AccessRequest{}
				Eventually(func() api.Status {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(f.accessrequests[0]), ar)
					Expect(err).NotTo(HaveOccurred())
					return ar.Status.RequestState
				}, timeout, interval).Should(Equal(api.ExpiredStatus))
				Expect(ar.Status.History).Should(HaveLen(3))
				Expect(ar.Status.History[0].RequestState).To(Equal(api.InitiatedStatus))
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
			It("will validate if the AccessRequest is deleted after TTL expiration", func() {
				ar := &api.AccessRequest{}
				Eventually(func() bool {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(f.accessrequests[0]), ar)
					return apierrors.IsNotFound(err) || ar.GetDeletionTimestamp() != nil
				}).WithTimeout(timeout + 10*time.Second).WithPolling(interval).Should(BeTrue())
			})
		})
		When("protected fields values change after applied", func() {
			const (
				namespace             = "test-02"
				arName                = "test-ar-02"
				appprojectName        = "sample-test-project-02"
				appName               = "some-application"
				roleTemplateName      = "some-role-template"
				roleTemplateNamespace = "test-02-rt"
				roleName              = "super-user"
				subject               = "some-user"
			)

			var f *fixture
			var r resources
			policies := []string{
				"p, {{.role}}, applications, sync, {{.project}}/{{.application}}, allow",
				"p, {{.role}}, applications, action/*, {{.project}}/{{.application}}, allow",
				"p, {{.role}}, applications, delete/*/Pod/*, {{.project}}/{{.application}}, allow",
				"p, {{.role}}, logs, get, {{.project}}/{{.namespace}}/{{.application}}, allow",
			}

			AfterAll(func() {
				deleteNamespace(f)
			})
			BeforeAll(func() {
				r = resources{
					arName:                arName,
					appName:               appName,
					namespace:             namespace,
					appProjName:           appprojectName,
					roleTemplateName:      roleTemplateName,
					roleTemplateNamespace: roleTemplateNamespace,
					roleName:              roleName,
					subject:               subject,
					policies:              policies,
				}
				f = setup(r)
			})
			It("will apply the roletemplate resource in k8s", func() {
				err := k8sClient.Create(ctx, f.roletemplate)
				Expect(err).NotTo(HaveOccurred())
			})
			It("will apply the access request resource in k8s", func() {
				f.accessrequests[0].Spec.Duration = metav1.Duration{Duration: time.Second * 5}
				err := k8sClient.Create(ctx, f.accessrequests[0])
				Expect(err).NotTo(HaveOccurred())
			})
			It("will verify if the access request is created", func() {
				ar := &api.AccessRequest{}
				Eventually(func() api.Status {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(f.accessrequests[0]), ar)
					Expect(err).NotTo(HaveOccurred())
					return ar.Status.RequestState
				}, timeout, interval).ShouldNot(BeEmpty())
				Expect(ar.Status.History).NotTo(BeEmpty())
				Expect(ar.Status.History[0].RequestState).To(Equal(api.InitiatedStatus))
			})
			It("will return immutable error on attempt to change the target role", func() {
				ar := &api.AccessRequest{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(f.accessrequests[0]), ar)
				Expect(err).NotTo(HaveOccurred())
				ar.Spec.Role.TemplateRef.Name = "NOT-ALLOWED"

				err = k8sClient.Update(ctx, ar)

				Expect(err).ToNot(BeNil())
				e := &apierrors.StatusError{}
				ok := errors.As(err, &e)
				Expect(ok).To(BeTrue(), "returned error type is not errors.StatusError")
				Expect(string(e.ErrStatus.Reason)).To(Equal("Invalid"))
				Expect(e.ErrStatus.Message).To(ContainSubstring("Value is immutable"))
			})
			It("will return immutable error on attempt to change the Application", func() {
				ar := &api.AccessRequest{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(f.accessrequests[0]), ar)
				Expect(err).NotTo(HaveOccurred())
				ar.Spec.Application.Name = "NOT-ALLOWED"
				ar.Spec.Application.Namespace = "NOT-ALLOWED"

				err = k8sClient.Update(ctx, ar)

				Expect(err).ToNot(BeNil())
				e := &apierrors.StatusError{}
				ok := errors.As(err, &e)
				Expect(ok).To(BeTrue(), "returned error type is not errors.StatusError")
				Expect(string(e.ErrStatus.Reason)).To(Equal("Invalid"))
				Expect(e.ErrStatus.Message).To(ContainSubstring("Value is immutable"))
			})
			It("will return immutable error on attempt to change the Subject", func() {
				ar := &api.AccessRequest{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(f.accessrequests[0]), ar)
				Expect(err).NotTo(HaveOccurred())
				ar.Spec.Subject.Username = "NOT-ALLOWED"

				err = k8sClient.Update(ctx, ar)

				Expect(err).ToNot(BeNil())
				e := &apierrors.StatusError{}
				ok := errors.As(err, &e)
				Expect(ok).To(BeTrue(), "returned error type is not errors.StatusError")
				Expect(string(e.ErrStatus.Reason)).To(Equal("Invalid"))
				Expect(e.ErrStatus.Message).To(ContainSubstring("Value is immutable"))
			})
		})
		When("when timeout is configured and the plugin takes too long to approve", func() {
			const (
				namespace             = "test-03-timeout"
				arName                = "test-ar-03-timeout"
				appprojectName        = "sample-test-project-02"
				appName               = "some-application"
				roleTemplateName      = "some-role-template"
				roleTemplateNamespace = "test-03-rt-timeout"
				roleName              = "super-user"
				subject               = "some-user"
			)

			var f *fixture
			var r resources
			policies := []string{
				"p, {{.role}}, applications, sync, {{.project}}/{{.application}}, allow",
				"p, {{.role}}, applications, action/*, {{.project}}/{{.application}}, allow",
				"p, {{.role}}, applications, delete/*/Pod/*, {{.project}}/{{.application}}, allow",
				"p, {{.role}}, logs, get, {{.project}}/{{.namespace}}/{{.application}}, allow",
			}

			accessRequesterMock.EXPECT().GrantAccess(mock.Anything, mock.Anything).
				RunAndReturn(func(ar *api.AccessRequest, a *argocd.Application) (*plugin.GrantResponse, error) {
					pluginResponse := &plugin.GrantResponse{
						Status: plugin.GrantStatusGranted,
					}
					if ar.GetName() == arName {
						pluginResponse.Status = plugin.GrantStatusPending
					}
					return pluginResponse, nil
				})

			AfterAll(func() {
				deleteNamespace(f)
			})
			BeforeAll(func() {
				r = resources{
					arName:                arName,
					appName:               appName,
					namespace:             namespace,
					appProjName:           appprojectName,
					roleTemplateName:      roleTemplateName,
					roleTemplateNamespace: roleTemplateNamespace,
					roleName:              roleName,
					subject:               subject,
					policies:              policies,
				}
				f = setup(r)
			})
			It("will apply the roletemplate resource in k8s", func() {
				err := k8sClient.Create(ctx, f.roletemplate)
				Expect(err).NotTo(HaveOccurred())
			})
			It("will apply the access request resource in k8s", func() {
				f.accessrequests[0].Spec.Duration = metav1.Duration{Duration: time.Second * 5}
				err := k8sClient.Create(ctx, f.accessrequests[0])
				Expect(err).NotTo(HaveOccurred())
			})
			It("will verify if the access request is created", func() {
				ar := &api.AccessRequest{}
				Eventually(func() api.Status {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(f.accessrequests[0]), ar)
					Expect(err).NotTo(HaveOccurred())
					return ar.Status.RequestState
				}, timeout, interval).ShouldNot(BeEmpty())
				Expect(ar.Status.History).NotTo(BeEmpty())
				Expect(ar.Status.History[0].RequestState).To(Equal(api.InitiatedStatus))
			})
			It("will validate if the access will eventually timeout", func() {
				ar := &api.AccessRequest{}
				Eventually(func() api.Status {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(f.accessrequests[0]), ar)
					Expect(err).NotTo(HaveOccurred())
					return ar.Status.RequestState
				}, timeout, interval).Should(Equal(api.TimeoutStatus))
				Expect(ar.Status.ExpiresAt).To(BeNil())
				Expect(ar.Status.History).Should(HaveLen(3))
				Expect(ar.Status.History[0].RequestState).To(Equal(api.InitiatedStatus))
				Expect(ar.Status.History[1].RequestState).To(Equal(api.RequestedStatus))
				Expect(ar.Status.History[2].RequestState).To(Equal(api.TimeoutStatus))
			})
		})
	})
	Context("Changing a RoleTemplate", Ordered, func() {
		const (
			namespace             = "test-03-rt-watch"
			arName01              = "test-ar-03"
			arName02              = "test-ar-04"
			appprojectName        = "sample-test-project"
			appName               = "some-application"
			roleTemplateName      = "role-template-watch-test"
			roleTemplateNamespace = "test-03-rt-watch-rt"
			roleName              = "super-user"
			subject01             = "some-user"
			subject02             = "another-user"
		)

		var f *fixture
		var r resources
		policies := []string{
			"p, {{.role}}, applications, sync, {{.project}}/{{.application}}, allow",
			"p, {{.role}}, applications, action/*, {{.project}}/{{.application}}, allow",
			"p, {{.role}}, applications, delete/*/Pod/*, {{.project}}/{{.application}}, allow",
			"p, {{.role}}, logs, get, {{.project}}/{{.namespace}}/{{.application}}, allow",
		}

		When("used by multiple AccessRequests", func() {
			AfterAll(func() {
				deleteNamespace(f)
			})
			BeforeAll(func() {
				r = resources{
					arName:                arName01,
					appName:               appName,
					namespace:             namespace,
					appProjName:           appprojectName,
					roleTemplateName:      roleTemplateName,
					roleTemplateNamespace: roleTemplateNamespace,
					roleName:              roleName,
					subject:               subject01,
					policies:              policies,
				}
				f = setup(r)
				f.accessrequests[0].Spec.Duration = metav1.Duration{Duration: time.Second * 5}
				ar2 := f.accessrequests[0].DeepCopy()
				ar2.SetName(arName02)
				ar2.Spec.Subject.Username = subject02
				f.accessrequests = append(f.accessrequests, ar2)
			})
			It("will apply the roletemplate resource in k8s", func() {
				err := k8sClient.Create(ctx, f.roletemplate)
				Expect(err).NotTo(HaveOccurred())
			})
			It("will apply the AccessRequests resources in k8s", func() {
				for _, ar := range f.accessrequests {
					err := k8sClient.Create(ctx, ar)
					Expect(err).NotTo(HaveOccurred())
				}
			})
			It("will verify if the AccessRequests are created", func() {
				returnedAR := &api.AccessRequest{}
				for _, ar := range f.accessrequests {
					Eventually(func() api.Status {
						err := k8sClient.Get(ctx, client.ObjectKeyFromObject(ar), returnedAR)
						Expect(err).NotTo(HaveOccurred())
						return returnedAR.Status.RequestState
					}, timeout, interval).ShouldNot(BeEmpty())
					Expect(returnedAR.Status.History).NotTo(BeEmpty())
					Expect(returnedAR.Status.History[0].RequestState).To(Equal(api.InitiatedStatus))
				}
			})
			It("will validate Argo CD AppProject", func() {
				key := client.ObjectKey{
					Namespace: namespace,
					Name:      appprojectName,
				}
				appProj := &argocd.AppProject{}
				Eventually(func() []string {
					err := k8sClient.Get(ctx, key, appProj)
					By("checking if role was created in AppProject")
					Expect(err).NotTo(HaveOccurred())
					Expect(appProj).NotTo(BeNil())
					Expect(appProj.Spec.Roles).To(HaveLen(3))
					return appProj.Spec.Roles[2].Groups
				}, timeout, interval).Should(HaveLen(2))
				By("checking if subjects are added in Argo CD role")
				Expect(appProj.Spec.Roles[2].Groups[0]).To(Equal(subject01))
				Expect(appProj.Spec.Roles[2].Groups[1]).To(Equal(subject02))
			})
			It("will reflect role template changes in AppProject", func() {
				newPolicy := "update-policy-test"
				f.roletemplate.Spec.Policies = []string{newPolicy}
				err := k8sClient.Update(ctx, f.roletemplate)
				Expect(err).NotTo(HaveOccurred())
				key := client.ObjectKey{
					Namespace: namespace,
					Name:      appprojectName,
				}
				appProj := &argocd.AppProject{}
				Eventually(func() []string {
					err := k8sClient.Get(ctx, key, appProj)
					By("checking if AppProject policy")
					Expect(err).NotTo(HaveOccurred())
					Expect(appProj).NotTo(BeNil())
					Expect(appProj.Spec.Roles).To(HaveLen(3))
					return appProj.Spec.Roles[2].Policies
				}, timeout, interval).Should(HaveLen(1))
				Expect(appProj.Spec.Roles[2].Policies[0]).To(Equal(newPolicy))
			})
		})
	})
	Context("Changing a Project", Ordered, func() {
		const (
			namespace             = "test-04-project-watch"
			arName01              = "test-ar-05"
			appprojectName        = "sample-test-project"
			appName               = "some-application"
			roleTemplateName      = "role-template-watch-test"
			roleTemplateNamespace = "test-04-project-watch-rt"
			roleName              = "super-user"
			subject01             = "some-user"
			expectedPolicy        = "original-policy"
		)

		var f *fixture
		var r resources
		policies := []string{expectedPolicy}

		When("used by an active AccessRequests", func() {
			AfterAll(func() {
				deleteNamespace(f)
			})
			BeforeAll(func() {
				r = resources{
					arName:                arName01,
					appName:               appName,
					namespace:             namespace,
					appProjName:           appprojectName,
					roleTemplateName:      roleTemplateName,
					roleTemplateNamespace: roleTemplateNamespace,
					roleName:              roleName,
					subject:               subject01,
					policies:              policies,
				}
				f = setup(r)
			})
			It("will apply the roletemplate resource in k8s", func() {
				err := k8sClient.Create(ctx, f.roletemplate)
				Expect(err).NotTo(HaveOccurred())
			})
			It("will apply the AccessRequests resources in k8s", func() {
				f.accessrequests[0].Spec.Duration = metav1.Duration{Duration: time.Second * 5}
				for _, ar := range f.accessrequests {
					err := k8sClient.Create(ctx, ar)
					Expect(err).NotTo(HaveOccurred())
				}
			})
			It("will verify if the AccessRequest is created", func() {
				returnedAR := &api.AccessRequest{}
				Eventually(func() api.Status {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(f.accessrequests[0]), returnedAR)
					Expect(err).NotTo(HaveOccurred())
					return returnedAR.Status.RequestState
				}, timeout, interval).ShouldNot(BeEmpty())
				Expect(returnedAR.Status.History).NotTo(BeEmpty())
				Expect(returnedAR.Status.History[0].RequestState).To(Equal(api.InitiatedStatus))
			})
			It("will revert changes made in the AppProject managed role", func() {
				key := client.ObjectKey{
					Namespace: namespace,
					Name:      appprojectName,
				}
				appProj := &argocd.AppProject{}
				Eventually(func() string {
					err := k8sClient.Get(ctx, key, appProj)
					Expect(err).NotTo(HaveOccurred())
					By("checking if role was created in AppProject")
					Expect(appProj).NotTo(BeNil())
					Expect(appProj.Spec.Roles).To(HaveLen(3))
					Expect(appProj.Spec.Roles[2].Policies).To(HaveLen(1))
					return appProj.Spec.Roles[2].Policies[0]
				}, timeout, interval).Should(Equal(expectedPolicy))

				By("checking if subject is added in Argo CD role")
				Expect(appProj.Spec.Roles[2].Groups[0]).To(Equal(subject01))

				By("modifying the AppProject managed role")
				newPolicy := "update-policy-test"
				appProj.Spec.Roles[2].Policies = []string{newPolicy}
				err := k8sClient.Update(ctx, appProj)
				Expect(err).NotTo(HaveOccurred())

				By("checking if policy change is reverted")
				Eventually(func() string {
					err := k8sClient.Get(ctx, key, appProj)
					Expect(err).NotTo(HaveOccurred())
					Expect(appProj.Spec.Roles[2].Policies).To(HaveLen(1))
					return appProj.Spec.Roles[2].Policies[0]
				}, timeout, interval).Should(Equal(expectedPolicy))
			})
		})
	})

	Context("Changing an Application", Ordered, func() {
		const (
			namespace             = "test-application-watch"
			arName01              = "test-ar-app-watch"
			appprojectName        = "sample-test-project"
			appName               = "some-application"
			roleTemplateName      = "role-template-watch-test"
			roleTemplateNamespace = "test-application-watch-rt"
			roleName              = "super-user"
			subject01             = "some-user"
			expectedPolicy        = "original-policy"
		)

		var f *fixture
		var r resources
		policies := []string{expectedPolicy}

		When("used by an active AccessRequests", func() {
			AfterAll(func() {
				deleteNamespace(f)
			})
			BeforeAll(func() {
				r = resources{
					arName:                arName01,
					appName:               appName,
					namespace:             namespace,
					appProjName:           appprojectName,
					roleTemplateName:      roleTemplateName,
					roleTemplateNamespace: roleTemplateNamespace,
					roleName:              roleName,
					subject:               subject01,
					policies:              policies,
				}
				f = setup(r)
			})
			It("will apply the roletemplate resource in k8s", func() {
				err := k8sClient.Create(ctx, f.roletemplate)
				Expect(err).NotTo(HaveOccurred())
			})
			It("will apply the AccessRequests resources in k8s", func() {
				f.accessrequests[0].Spec.Duration = metav1.Duration{Duration: time.Second * 5}
				for _, ar := range f.accessrequests {
					err := k8sClient.Create(ctx, ar)
					Expect(err).NotTo(HaveOccurred())
				}
			})
			It("will verify if the AccessRequest is created", func() {
				returnedAR := &api.AccessRequest{}
				Eventually(func() api.Status {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(f.accessrequests[0]), returnedAR)
					Expect(err).NotTo(HaveOccurred())
					return returnedAR.Status.RequestState
				}, timeout, interval).ShouldNot(BeEmpty())
				Expect(returnedAR.Status.History).NotTo(BeEmpty())
				Expect(returnedAR.Status.History[0].RequestState).To(Equal(api.InitiatedStatus))
			})
			It("will invalidate the AccessRequest when the target project is changed in the app", func() {

				By("modifying the Application project")
				appYaml := testdata.ApplicationYaml
				app, err := utils.YamlToUnstructured(appYaml)
				Expect(err).NotTo(HaveOccurred())
				app.SetName(appName)
				app.SetNamespace(namespace)
				unstructured.SetNestedField(app.Object, "another-project", "spec", "project")
				// The dynamic client is used to create the resource with all fields
				// defined in the official Argo CD CRD
				_, err = dynClient.Resource(appResource).
					Namespace(r.namespace).
					Apply(ctx, r.appName, app, metav1.ApplyOptions{
						FieldManager: "argocd-controller",
					})
				Expect(err).NotTo(HaveOccurred())

				By("checking if AccessRequest is invalidated")
				returnedAR := &api.AccessRequest{}
				Eventually(func() api.Status {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(f.accessrequests[0]), returnedAR)
					Expect(err).NotTo(HaveOccurred())
					return returnedAR.Status.RequestState
				}, timeout, interval).Should(Equal(api.InvalidStatus))
				Expect(returnedAR.Status.RequestState).NotTo(BeEmpty())
				lastHistory := returnedAR.Status.History[len(returnedAR.Status.History)-1]
				Expect(lastHistory.RequestState).To(Equal(api.InvalidStatus))
				Expect(lastHistory.Details).NotTo(BeNil())
				Expect(*lastHistory.Details).To(ContainSubstring("project changed"))
			})
		})
	})

	Context("Validating AccessRequests", Ordered, func() {
		const (
			namespace             = "test-ar-validation"
			arName01              = "test-ar-06"
			appprojectName        = "sample-test-project"
			appName               = "some-application"
			roleTemplateName      = "role-template-watch-test"
			roleTemplateNamespace = "test-ar-validation-rt"
			roleName              = "super-user"
			subject01             = "some-user"
			expectedPolicy        = "original-policy"
		)

		var f *fixture
		var r resources
		policies := []string{expectedPolicy}

		AfterAll(func() {
			deleteNamespace(f)
		})
		BeforeAll(func() {
			r = resources{
				arName:                arName01,
				appName:               appName,
				namespace:             namespace,
				appProjName:           appprojectName,
				roleTemplateName:      roleTemplateName,
				roleTemplateNamespace: roleTemplateNamespace,
				roleName:              roleName,
				subject:               subject01,
				policies:              policies,
			}
			f = setup(r)
		})
		When("creating initial resources state in k8s", func() {
			It("will apply the roletemplate resource in k8s", func() {
				err := k8sClient.Create(ctx, f.roletemplate)
				Expect(err).NotTo(HaveOccurred())
				rtValidate := utils.NewRoleTemplate("anotherrole", roleTemplateNamespace, roleName, policies)
				err = k8sClient.Create(ctx, rtValidate)
				rtRace := utils.NewRoleTemplate("racerole", roleTemplateNamespace, roleName, policies)
				err = k8sClient.Create(ctx, rtRace)
				Expect(err).NotTo(HaveOccurred())
			})
			It("will apply the AccessRequests resources in k8s", func() {
				f.accessrequests[0].Spec.Duration = metav1.Duration{Duration: time.Minute}
				for _, ar := range f.accessrequests {
					err := k8sClient.Create(ctx, ar)
					Expect(err).NotTo(HaveOccurred())
				}
			})
			It("will verify if the AccessRequest is created", func() {
				returnedAR := &api.AccessRequest{}
				Eventually(func() api.Status {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(f.accessrequests[0]), returnedAR)
					Expect(err).NotTo(HaveOccurred())
					return returnedAR.Status.RequestState
				}, timeout, interval).ShouldNot(BeEmpty())
				Expect(returnedAR.Status.History).NotTo(BeEmpty())
				Expect(returnedAR.Status.History[0].RequestState).To(Equal(api.InitiatedStatus))
			})
		})
		When("creating conflicting AccessRequest", func() {
			It("will create conflicting AccessRequest", func() {
				conflictAR := utils.NewAccessRequest("conflict", namespace, appName, namespace, roleTemplateName, roleTemplateNamespace, subject01)
				err := k8sClient.Create(ctx, conflictAR)
				Expect(err).NotTo(HaveOccurred())
			})
			It("will verify if the conflicting AccessRequest has 'invalid' status", func() {
				returnedAR := &api.AccessRequest{}
				key := client.ObjectKey{
					Namespace: namespace,
					Name:      "conflict",
				}
				Eventually(func() api.Status {
					err := k8sClient.Get(ctx, key, returnedAR)
					Expect(err).NotTo(HaveOccurred())
					return returnedAR.Status.RequestState
				}, timeout, interval).ShouldNot(BeEmpty())
				Expect(returnedAR.Status.RequestState).To(Equal(api.InvalidStatus))
				Expect(returnedAR.Status.History).NotTo(BeEmpty())
				Expect(returnedAR.Status.History[0].RequestState).To(Equal(api.InvalidStatus))
			})
		})
		When("creating an AccessRequest for the same user/app but different role", func() {
			It("will create the AccessRequest successfully", func() {
				anotherroleAR := utils.NewAccessRequest("anotherrole", namespace, appName, namespace, "anotherrole", roleTemplateNamespace, subject01)
				anotherroleAR.Spec.Duration = metav1.Duration{Duration: time.Minute}
				err := k8sClient.Create(ctx, anotherroleAR)
				Expect(err).NotTo(HaveOccurred())
			})
			It("will verify that AccessRequest is granted", func() {
				returnedAR := &api.AccessRequest{}
				key := client.ObjectKey{
					Namespace: namespace,
					Name:      "anotherrole",
				}
				Eventually(func() api.Status {
					err := k8sClient.Get(ctx, key, returnedAR)
					Expect(err).NotTo(HaveOccurred())
					return returnedAR.Status.RequestState
				}, timeout, interval).Should(Equal(api.GrantedStatus))
				Expect(returnedAR.Status.History).NotTo(BeEmpty())
				Expect(returnedAR.Status.History).Should(HaveLen(2))
				Expect(returnedAR.Status.History[0].RequestState).To(Equal(api.InitiatedStatus))
				Expect(returnedAR.Status.History[1].RequestState).To(Equal(api.GrantedStatus))
			})
		})
		When("creating two AccessRequests at the same time", func() {
			It("will create them successfully", func() {
				race1AR := utils.NewAccessRequest("race1", namespace, appName, namespace, "racerole", roleTemplateNamespace, subject01)
				race1AR.Spec.Duration = metav1.Duration{Duration: time.Minute}
				race2AR := utils.NewAccessRequest("race2", namespace, appName, namespace, "racerole", roleTemplateNamespace, subject01)
				race2AR.Spec.Duration = metav1.Duration{Duration: time.Minute}
				go func() {
					err := k8sClient.Create(ctx, race1AR)
					Expect(err).NotTo(HaveOccurred())
				}()
				go func() {
					err := k8sClient.Create(ctx, race2AR)
					Expect(err).NotTo(HaveOccurred())
				}()
			})
			It("will verify that one AccessRequest is valid and one is invalid", func() {
				race1AR := &api.AccessRequest{}
				key1 := client.ObjectKey{
					Namespace: namespace,
					Name:      "race1",
				}
				Eventually(func() api.Status {
					err := k8sClient.Get(ctx, key1, race1AR)
					Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
					return race1AR.Status.RequestState
				}, timeout, interval).ShouldNot(BeEmpty())

				race2AR := &api.AccessRequest{}
				key2 := client.ObjectKey{
					Namespace: namespace,
					Name:      "race2",
				}
				Eventually(func() api.Status {
					err := k8sClient.Get(ctx, key2, race2AR)
					Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
					return race2AR.Status.RequestState
				}, timeout, interval).ShouldNot(BeEmpty())

				totalInvalid := 0
				totalValid := 0

				switch race1AR.Status.RequestState {
				case api.InvalidStatus:
					totalInvalid++
				case api.GrantedStatus, api.RequestedStatus:
					totalValid++
				}

				switch race2AR.Status.RequestState {
				case api.InvalidStatus:
					totalInvalid++
				case api.GrantedStatus, api.RequestedStatus:
					totalValid++
				}

				Expect(totalInvalid).To(Equal(1), "totalInvalid mismatch")
				Expect(totalValid).To(Equal(1), "totalValid mismatch")
			})
		})
	})
})

func TestProjectChangeShouldTriggerReconcile(t *testing.T) {
	role1 := argocd.ProjectRole{
		Name:        fmt.Sprintf("%srole1", api.RoleNamePrefix),
		Description: "desc1",
		Policies:    []string{"policy1", "policy2"},
		JWTTokens: []argocd.JWTToken{
			{ID: "token1"},
		},
		Groups: []string{"group1"},
	}
	role2 := argocd.ProjectRole{
		Name:        fmt.Sprintf("%srole2", api.RoleNamePrefix),
		Description: "desc2",
		Policies:    []string{"policy3"},
		JWTTokens: []argocd.JWTToken{
			{ID: "token2"},
		},
		Groups: []string{"group2"},
	}

	tests := []struct {
		name    string
		oldProj func() *argocd.AppProject
		newProj func() *argocd.AppProject
		want    bool
	}{
		{
			name: "tokens length changed",
			oldProj: func() *argocd.AppProject {
				return &argocd.AppProject{Spec: argocd.AppProjectSpec{Roles: []argocd.ProjectRole{role1}}}
			},
			newProj: func() *argocd.AppProject {
				r := role1.DeepCopy()
				r.JWTTokens = []argocd.JWTToken{}
				return &argocd.AppProject{Spec: argocd.AppProjectSpec{Roles: []argocd.ProjectRole{*r}}}
			},
			want: true,
		},
		{
			name: "tokens changed",
			oldProj: func() *argocd.AppProject {
				return &argocd.AppProject{Spec: argocd.AppProjectSpec{Roles: []argocd.ProjectRole{role1}}}
			},
			newProj: func() *argocd.AppProject {
				r := role1.DeepCopy()
				r.JWTTokens = []argocd.JWTToken{{ID: "token1-changed"}}
				return &argocd.AppProject{Spec: argocd.AppProjectSpec{Roles: []argocd.ProjectRole{*r}}}
			},
			want: true,
		},
		{
			name: "groups length changed",
			oldProj: func() *argocd.AppProject {
				return &argocd.AppProject{Spec: argocd.AppProjectSpec{Roles: []argocd.ProjectRole{role1}}}
			},
			newProj: func() *argocd.AppProject {
				r := role1.DeepCopy()
				r.Groups = append(r.Groups, "group3")
				return &argocd.AppProject{Spec: argocd.AppProjectSpec{Roles: []argocd.ProjectRole{*r}}}
			},
			want: true,
		},
		{
			name: "groups changed",
			oldProj: func() *argocd.AppProject {
				return &argocd.AppProject{Spec: argocd.AppProjectSpec{Roles: []argocd.ProjectRole{role1}}}
			},
			newProj: func() *argocd.AppProject {
				r := role1.DeepCopy()
				r.Groups[0] = "group1-changed"
				return &argocd.AppProject{Spec: argocd.AppProjectSpec{Roles: []argocd.ProjectRole{*r}}}
			},
			want: true,
		},
		{
			name: "roles length changed",
			oldProj: func() *argocd.AppProject {
				return &argocd.AppProject{Spec: argocd.AppProjectSpec{Roles: []argocd.ProjectRole{role1}}}
			},
			newProj: func() *argocd.AppProject {
				return &argocd.AppProject{Spec: argocd.AppProjectSpec{Roles: []argocd.ProjectRole{role1, role2}}}
			},
			want: true,
		},
		{
			name: "role name changed",
			oldProj: func() *argocd.AppProject {
				return &argocd.AppProject{Spec: argocd.AppProjectSpec{Roles: []argocd.ProjectRole{role1}}}
			},
			newProj: func() *argocd.AppProject {
				return &argocd.AppProject{Spec: argocd.AppProjectSpec{Roles: []argocd.ProjectRole{
					{Name: "role1-changed", Description: "desc1", Policies: []string{"policy1", "policy2"}},
				},
				}}
			},
			want: true,
		},
		{
			name: "role description changed",
			oldProj: func() *argocd.AppProject {
				return &argocd.AppProject{Spec: argocd.AppProjectSpec{Roles: []argocd.ProjectRole{role1}}}
			},
			newProj: func() *argocd.AppProject {
				return &argocd.AppProject{Spec: argocd.AppProjectSpec{Roles: []argocd.ProjectRole{
					{Name: "role1", Description: "desc1-changed", Policies: []string{"policy1", "policy2"}}},
				}}
			},
			want: true,
		},
		{
			name: "role policies changed",
			oldProj: func() *argocd.AppProject {
				return &argocd.AppProject{Spec: argocd.AppProjectSpec{Roles: []argocd.ProjectRole{role1}}}
			},
			newProj: func() *argocd.AppProject {
				return &argocd.AppProject{Spec: argocd.AppProjectSpec{Roles: []argocd.ProjectRole{
					{Name: "role1", Description: "desc1", Policies: []string{"policy1"}},
				}}}
			},
			want: true,
		},
		{
			name: "no change",
			oldProj: func() *argocd.AppProject {
				return &argocd.AppProject{Spec: argocd.AppProjectSpec{Roles: []argocd.ProjectRole{role2, role1}}}
			},
			newProj: func() *argocd.AppProject {
				return &argocd.AppProject{Spec: argocd.AppProjectSpec{Roles: []argocd.ProjectRole{role1, role2}}}
			},
			want: false,
		},
		{
			name:    "old project nil",
			oldProj: func() *argocd.AppProject { return nil },
			newProj: func() *argocd.AppProject {
				return &argocd.AppProject{Spec: argocd.AppProjectSpec{Roles: []argocd.ProjectRole{role1, role2}}}
			},
			want: true,
		},
		{
			name: "new project nil",
			oldProj: func() *argocd.AppProject {
				return &argocd.AppProject{Spec: argocd.AppProjectSpec{Roles: []argocd.ProjectRole{role1, role2}}}
			},
			newProj: func() *argocd.AppProject { return nil },
			want:    true,
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			got := ProjectChangeShouldTriggerReconcile(tt.newProj(), tt.oldProj())
			if got != tt.want {
				t.Errorf("ProjectChangeShouldTriggerReconcile() = %v, want %v", got, tt.want)
			}
		})
	}
}
