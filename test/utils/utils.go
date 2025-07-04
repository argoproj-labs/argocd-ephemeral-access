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

package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	api "github.com/argoproj-labs/argocd-ephemeral-access/api/ephemeral-access/v1alpha1"
	. "github.com/onsi/ginkgo/v2" //nolint:golint,revive
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"
)

const (
	prometheusOperatorVersion = "v0.72.0"
	prometheusOperatorURL     = "https://github.com/prometheus-operator/prometheus-operator/" +
		"releases/download/%s/bundle.yaml"

	certmanagerVersion = "v1.14.4"
	certmanagerURLTmpl = "https://github.com/jetstack/cert-manager/releases/download/%s/cert-manager.yaml"
)

func warnError(err error) {
	fmt.Fprintf(GinkgoWriter, "warning: %v\n", err)
}

// InstallPrometheusOperator installs the prometheus Operator to be used to export the enabled metrics.
func InstallPrometheusOperator() error {
	url := fmt.Sprintf(prometheusOperatorURL, prometheusOperatorVersion)
	cmd := exec.Command("kubectl", "create", "-f", url)
	_, err := Run(cmd)
	return err
}

// Run executes the provided command within this context
func Run(cmd *exec.Cmd) ([]byte, error) {
	dir, _ := GetProjectDir()
	cmd.Dir = dir

	if err := os.Chdir(cmd.Dir); err != nil {
		fmt.Fprintf(GinkgoWriter, "chdir dir: %s\n", err)
	}

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	fmt.Fprintf(GinkgoWriter, "running: %s\n", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%s failed with error: (%v) %s", command, err, string(output))
	}

	return output, nil
}

// UninstallPrometheusOperator uninstalls the prometheus
func UninstallPrometheusOperator() {
	url := fmt.Sprintf(prometheusOperatorURL, prometheusOperatorVersion)
	cmd := exec.Command("kubectl", "delete", "-f", url)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// UninstallCertManager uninstalls the cert manager
func UninstallCertManager() {
	url := fmt.Sprintf(certmanagerURLTmpl, certmanagerVersion)
	cmd := exec.Command("kubectl", "delete", "-f", url)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// InstallCertManager installs the cert manager bundle.
func InstallCertManager() error {
	url := fmt.Sprintf(certmanagerURLTmpl, certmanagerVersion)
	cmd := exec.Command("kubectl", "apply", "-f", url)
	if _, err := Run(cmd); err != nil {
		return err
	}
	// Wait for cert-manager-webhook to be ready, which can take time if cert-manager
	// was re-installed after uninstalling on a cluster.
	cmd = exec.Command("kubectl", "wait", "deployment.apps/cert-manager-webhook",
		"--for", "condition=Available",
		"--namespace", "cert-manager",
		"--timeout", "5m",
	)

	_, err := Run(cmd)
	return err
}

// LoadImageToKindCluster loads a local docker image to the kind cluster
func LoadImageToKindClusterWithName(name string) error {
	cluster := "kind"
	if v, ok := os.LookupEnv("KIND_CLUSTER"); ok {
		cluster = v
	}
	kindOptions := []string{"load", "docker-image", name, "--name", cluster}
	cmd := exec.Command("kind", kindOptions...)
	_, err := Run(cmd)
	return err
}

// GetNonEmptyLines converts given command output string into individual objects
// according to line breakers, and ignores the empty elements in it.
func GetNonEmptyLines(output string) []string {
	var res []string
	elements := strings.Split(output, "\n")
	for _, element := range elements {
		if element != "" {
			res = append(res, element)
		}
	}

	return res
}

// GetProjectDir will return the directory where the project is
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, err
	}
	wd = strings.Replace(wd, "/test/e2e", "", -1)
	return wd, nil
}

func ToUnstructured(val interface{}) (*unstructured.Unstructured, error) {
	data, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	res := make(map[string]interface{})
	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: res}, nil
}

func YamlToUnstructured(yamlStr string) (*unstructured.Unstructured, error) {
	obj := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(yamlStr), &obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: obj}, nil
}

// NewAccessRequest creates an AccessRequest
func NewAccessRequest(name, namespace, appName, appNamespace, roleName, roleNamespace, subject string) *api.AccessRequest {
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
			Duration: metav1.Duration{},
			Role: api.TargetRole{
				TemplateRef: api.TargetRoleTemplate{
					Name:      roleName,
					Namespace: roleNamespace,
				},
			},
			Application: api.TargetApplication{
				Name:      appName,
				Namespace: appNamespace,
			},
			Subject: api.Subject{
				Username: subject,
			},
		},
	}
}

// AccessRequestMutation is used to mutate an AccessRequest to simultate the actions of the controller
type AccessRequestMutation func(ar *api.AccessRequest)

// WithRole adds a role to the AccessRequest spec
func WithRole() AccessRequestMutation {
	name := "Ephemeral Role"
	return func(ar *api.AccessRequest) {
		ar.Spec.Role.Ordinal = 1
		ar.Spec.Role.FriendlyName = &name
	}
}

// WithName updartes the name of the AccessRequest
func WithName(name string) AccessRequestMutation {
	return func(ar *api.AccessRequest) {
		ar.ObjectMeta.Name = name
	}
}

// ToInvalidState transition the AccessRequest to an invalid status
func ToInvalidState() AccessRequestMutation {
	return func(ar *api.AccessRequest) {
		ar.Status = api.AccessRequestStatus{
			RequestState: api.InvalidStatus,
			History: []api.AccessRequestHistory{
				{
					TransitionTime: metav1.Now(),
					RequestState:   api.InvalidStatus,
					Details:        ptr.To("invalid for some reasons"),
				},
			},
		}
	}
}

// ToRequestedState transition the AccessRequest to a requested status
func ToRequestedState() AccessRequestMutation {
	return func(ar *api.AccessRequest) {
		now := metav1.Now()
		ar.Status.RequestState = api.RequestedStatus
		ar.Status.History = append(ar.Status.History, api.AccessRequestHistory{
			TransitionTime: now,
			RequestState:   api.RequestedStatus,
		})
	}
}

// ToInitiatedState transition the AccessRequest to the initiated status
func ToInitiatedState() AccessRequestMutation {
	return func(ar *api.AccessRequest) {
		ar.Status = api.AccessRequestStatus{
			RequestState:     api.InitiatedStatus,
			TargetProject:    "my-proj",
			RoleName:         "ephemeral-some-role",
			RoleTemplateHash: "0123456789",
			History: []api.AccessRequestHistory{
				{
					TransitionTime: metav1.Now(),
					RequestState:   api.InitiatedStatus,
				},
			},
		}
	}
}

// ToGrantedState transition the AccessRequest to a granted status
func ToGrantedState() AccessRequestMutation {
	return func(ar *api.AccessRequest) {
		now := metav1.Now()
		exp := metav1.NewTime(now.Add(1 * time.Minute))
		ar.Status.RequestState = api.GrantedStatus
		ar.Status.ExpiresAt = &exp
		ar.Status.History = append(ar.Status.History, api.AccessRequestHistory{
			TransitionTime: now,
			RequestState:   api.GrantedStatus,
		})
	}
}

// ToDeniedState transition the AccessRequest to a denied status
func ToDeniedState() AccessRequestMutation {
	return func(ar *api.AccessRequest) {
		now := metav1.Now()
		ar.Status.RequestState = api.DeniedStatus
		ar.Status.History = append(ar.Status.History, api.AccessRequestHistory{
			TransitionTime: now,
			RequestState:   api.DeniedStatus,
			Details:        ptr.To("Denied because this is a test"),
		})
	}
}

// ToExpiredState transition the AccessRequest to an expired status
func ToExpiredState() AccessRequestMutation {
	return func(ar *api.AccessRequest) {
		now := metav1.Now()
		ar.Status.RequestState = api.ExpiredStatus
		ar.Status.ExpiresAt = &now
		ar.Status.History = append(ar.Status.History, api.AccessRequestHistory{
			TransitionTime: now,
			RequestState:   api.ExpiredStatus,
		})
	}
}

// newAccessRequest returns an AccessRequest object with the default required fields populated
func newAccessRequest() *api.AccessRequest {
	return &api.AccessRequest{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AccessRequest",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "access-request-name",
			Namespace: "access-request-namespace",
		},
		Spec: api.AccessRequestSpec{
			Duration: metav1.Duration{},
			Role: api.TargetRole{
				TemplateRef: api.TargetRoleTemplate{
					Name:      "role-template-name",
					Namespace: "ephemeral",
				},
			},
			Application: api.TargetApplication{
				Name:      "my-app",
				Namespace: "my-app-namespace",
			},
			Subject: api.Subject{
				Username: "my@user.com",
			},
		},
	}
}

// NewAccessRequestCreated creates an AccessRequest
func NewAccessRequestCreated(transformers ...AccessRequestMutation) *api.AccessRequest {
	ar := newAccessRequest()
	for _, t := range transformers {
		t(ar)
	}
	return ar
}

// NewAccessRequestCreated creates an AccessRequest transitioned in an invalid state
func NewAccessRequestInvalid(transformers ...AccessRequestMutation) *api.AccessRequest {
	return NewAccessRequestCreated(append([]AccessRequestMutation{ToInvalidState()}, transformers...)...)
}

// NewAccessRequestRequested creates an AccessRequest in initiated state
func NewAccessRequestInitiated(transformers ...AccessRequestMutation) *api.AccessRequest {
	return NewAccessRequestCreated(append([]AccessRequestMutation{ToInitiatedState()}, transformers...)...)
}

// NewAccessRequestRequested creates an AccessRequest transitioned in a requested state
func NewAccessRequestRequested(transformers ...AccessRequestMutation) *api.AccessRequest {
	return NewAccessRequestInitiated(append([]AccessRequestMutation{ToRequestedState()}, transformers...)...)
}

// NewAccessRequestGranted creates an AccessRequest transitioned in an invalid state
func NewAccessRequestGranted(transformers ...AccessRequestMutation) *api.AccessRequest {
	return NewAccessRequestInitiated(append([]AccessRequestMutation{ToGrantedState()}, transformers...)...)
}

// NewAccessRequestDenied creates an AccessRequest transitioned in a denied state
func NewAccessRequestDenied(transformers ...AccessRequestMutation) *api.AccessRequest {
	return NewAccessRequestInitiated(append([]AccessRequestMutation{ToDeniedState()}, transformers...)...)
}

// NewAccessRequestExpired creates an AccessRequest transitioned in an expired state
func NewAccessRequestExpired(transformers ...AccessRequestMutation) *api.AccessRequest {
	return NewAccessRequestGranted(append([]AccessRequestMutation{ToExpiredState()}, transformers...)...)
}

func NewRoleTemplate(templateName, namespace, roleName string, policies []string) *api.RoleTemplate {
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

func NewNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// Eventually runs f until it returns true, an error or the timeout expires
func Eventually(f func() (bool, error), timeout time.Duration, interval time.Duration) error {
	start := time.Now()
	for {
		if ok, err := f(); ok {
			return nil
		} else if err != nil {
			return err
		}
		if time.Since(start) > timeout {
			return fmt.Errorf("timed out waiting for eventual success")
		}
		time.Sleep(interval)
	}
}
