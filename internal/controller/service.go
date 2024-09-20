package controller

import (
	"context"
	"crypto/sha1"
	"fmt"

	argocd "github.com/argoproj-labs/ephemeral-access/api/argoproj/v1alpha1"
	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/internal/controller/config"
	"github.com/argoproj-labs/ephemeral-access/pkg/log"
	"github.com/cnf/structhash"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	FieldOwnerEphemeralAccess = "ephemeral-access-controller"
)

type K8sClient interface {
	// Patch patches the given obj in the Kubernetes cluster. obj must be a
	// struct pointer so that obj can be updated with the content returned by the Server.
	Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error

	// Get retrieves an obj for the given object key from the Kubernetes Cluster.
	// obj must be a struct pointer so that obj can be updated with the response
	// returned by the Server.
	Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error

	// Status knows how to create a client which can update status subresource
	// for kubernetes objects.
	Status() client.SubResourceWriter
}

type Service struct {
	k8sClient K8sClient
	Config    config.ControllerConfigurer
}

func NewService(c K8sClient, cfg config.ControllerConfigurer) *Service {
	return &Service{
		k8sClient: c,
		Config:    cfg,
	}
}

// handlePermission will analyse the given ar and proceed with granting
// or removing Argo CD access for the subject listed in the AccessRequest.
// The following validations will be executed:
//  1. Check if the given ar is expired. If so, the subject will be removed from
//     the Argo CD role.
//  2. Check if the subject is allowed to be assigned in the given AccessRequest
//     target role. If so, it will proceed with grating Argo CD access. Otherwise
//     it will return DeniedStatus.
//
// It will update the AccessRequest status accordingly with the situation.
func (s *Service) HandlePermission(ctx context.Context, ar *api.AccessRequest, app *argocd.Application, rt *api.RoleTemplate) (api.Status, error) {
	logger := log.FromContext(ctx)

	if ar.IsExpiring() {
		logger.Info("AccessRequest is expired")
		err := s.handleAccessExpired(ctx, ar, rt)
		if err != nil {
			return "", fmt.Errorf("error handling access expired: %w", err)
		}
		return api.ExpiredStatus, nil
	}

	resp, err := Allowed(ctx, ar, app)
	if err != nil {
		return "", fmt.Errorf("error verifying if subject is allowed: %w", err)
	}
	if !resp.Allowed {
		rtHash := RoleTemplateHash(rt)
		err = s.updateStatus(ctx, ar, api.DeniedStatus, resp.Message, rtHash)
		if err != nil {
			return "", fmt.Errorf("error updating access request status to denied: %w", err)
		}
		return api.DeniedStatus, nil
	}

	details := ""
	status, err := s.grantArgoCDAccess(ctx, ar, rt)
	if err != nil {
		details = fmt.Sprintf("Error granting Argo CD Access: %s", err)
	}
	// only update status if the current state is different
	if ar.Status.RequestState != status {
		rtHash := RoleTemplateHash(rt)
		err = s.updateStatus(ctx, ar, status, details, rtHash)
		if err != nil {
			return "", fmt.Errorf("error updating access request status to granted: %w", err)
		}
	}
	return status, nil
}

// handleAccessExpired will remove the Argo CD access for the subject and
// update the AccessRequest status field.
func (s *Service) handleAccessExpired(ctx context.Context, ar *api.AccessRequest, rt *api.RoleTemplate) error {
	err := s.RemoveArgoCDAccess(ctx, ar, rt)
	if err != nil {
		return fmt.Errorf("error removing access for expired request: %w", err)
	}
	hash := RoleTemplateHash(rt)
	err = s.updateStatus(ctx, ar, api.ExpiredStatus, "", hash)
	if err != nil {
		return fmt.Errorf("error updating access request status to expired: %w", err)
	}
	return nil
}

// removeArgoCDAccess will remove the subject in the given AccessRequest from
// the given ar.TargetRoleName from the Argo CD project referenced in the
// ar.Spec.AppProject. The AppProject update will be executed via a patch with
// optimistic lock enabled. It will retry in case of AppProject conflict is
// identied.
func (s *Service) RemoveArgoCDAccess(ctx context.Context, ar *api.AccessRequest, rt *api.RoleTemplate) error {
	logger := log.FromContext(ctx)
	logger.Info("Removing Argo CD Access")
	projName := ar.Status.TargetProject
	projNamespace := ar.GetNamespace()

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		project, err := s.getProject(ctx, projName, projNamespace)
		if err != nil {
			e := fmt.Errorf("error getting Argo CD Project %s/%s: %w", projNamespace, projName, err)
			return client.IgnoreNotFound(e)
		}
		patch := client.MergeFromWithOptions(project.DeepCopy(), client.MergeFromWithOptimisticLock{})

		logger.Debug("Removing subject from role")
		removeSubjectFromRole(project, ar, rt)
		// this is necessary to make sure that the AppProject role managed by
		// this controller is always in sync with what is defined in the
		// RoleTemplate
		updateProjectPolicies(project, ar, rt)

		logger.Debug("Patching AppProject")
		opts := []client.PatchOption{client.FieldOwner(FieldOwnerEphemeralAccess)}
		err = s.k8sClient.Patch(ctx, project, patch, opts...)
		if err != nil {
			return fmt.Errorf("error patching Argo CD Project %s/%s: %w", projNamespace, projName, err)
		}
		return nil
	})
}

// grantArgoCDAccess will associate the given AccessRequest subject in the
// Argo CD AppProject specified in the ar.Spec.AppProject in the role defined
// in ar.TargetRoleName. The AppProject update will be executed via a patch with
// optimistic lock enabled. It Will retry in case of AppProject conflict is
// identied.
func (s *Service) grantArgoCDAccess(ctx context.Context, ar *api.AccessRequest, rt *api.RoleTemplate) (api.Status, error) {
	logger := log.FromContext(ctx)
	logger.Info("Granting Argo CD Access")

	projName := ar.Status.TargetProject
	projNamespace := ar.GetNamespace()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		project, err := s.getProject(ctx, projName, projNamespace)
		if err != nil {
			return fmt.Errorf("error getting Argo CD Project %s/%s: %w", projNamespace, projName, err)
		}
		patch := client.MergeFromWithOptions(project.DeepCopy(), client.MergeFromWithOptimisticLock{})

		logger.Debug("Adding subject in role")
		addSubjectInRole(project, ar, rt)
		// this is necessary to make sure that the AppProject role managed by
		// this controller is always in sync with what is defined in the
		// RoleTemplate
		updateProjectPolicies(project, ar, rt)

		logger.Debug("Patching AppProject")
		opts := []client.PatchOption{client.FieldOwner("ephemeral-access-controller")}
		err = s.k8sClient.Patch(ctx, project, patch, opts...)
		if err != nil {
			return fmt.Errorf("error patching Argo CD Project %s/%s: %w", projNamespace, projName, err)
		}

		return nil
	})
	if err != nil {
		return api.DeniedStatus, err
	}
	return api.GrantedStatus, nil
}

// RoleTemplateHash will generate a hash for the given role template
// based only on the necessary fields to require an update in the AppProject
// role
func RoleTemplateHash(rt *api.RoleTemplate) string {
	rtForHash := *&api.RoleTemplate{
		TypeMeta: rt.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      rt.GetName(),
			Namespace: rt.GetNamespace(),
		},
		Spec: api.RoleTemplateSpec{
			Name:        rt.Spec.Name,
			Description: rt.Spec.Description,
			Policies:    rt.Spec.Policies,
		},
	}
	return fmt.Sprintf("%x", sha1.Sum(structhash.Dump(rtForHash, 1)))
}

func (s *Service) getProject(ctx context.Context, name, ns string) (*argocd.AppProject, error) {
	project := &argocd.AppProject{}
	objKey := client.ObjectKey{
		Namespace: ns,
		Name:      name,
	}
	err := s.k8sClient.Get(ctx, objKey, project)
	if err != nil {
		return nil, err
	}
	return project, nil
}

// updateStatusWithRetry will retrieve the latest AccessRequest state before
// attempting to update its status. In case of conflict error, it will retry
// using the DefaultRetry backoff which has the following configs:
//
//	Steps: 5, Duration: 10 milliseconds, Factor: 1.0, Jitter: 0.1
func (s *Service) updateStatusWithRetry(ctx context.Context, ar *api.AccessRequest, status api.Status, details string, rtHash string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := s.k8sClient.Get(ctx, client.ObjectKeyFromObject(ar), ar)
		if err != nil {
			return err
		}
		return s.updateStatus(ctx, ar, status, details, rtHash)
	})
}

// updateStatus will update the given AccessRequest status field with the
// given status and details.
func (s *Service) updateStatus(ctx context.Context, ar *api.AccessRequest, status api.Status, details string, rtHash string) error {
	// if it is already updated skip
	if ar.Status.RequestState == status && ar.Status.RoleTemplateHash == rtHash {
		return nil
	}
	ar.UpdateStatusHistory(status, details)
	ar.Status.RoleTemplateHash = rtHash
	return s.k8sClient.Status().Update(ctx, ar)
}

// removeSubjectFromRole will iterate over the roles in the given project and
// remove the subject from the given AccessRequest from the role specified in
// the ar.TargetRoleName.
func removeSubjectFromRole(project *argocd.AppProject, ar *api.AccessRequest, rt *api.RoleTemplate) {
	roleName := rt.AppProjectRoleName(ar.Spec.Application.Name, ar.Spec.Application.Namespace)
	for idx, role := range project.Spec.Roles {
		if role.Name == roleName {
			groups := []string{}
			for _, group := range role.Groups {
				remove := false
				if group == ar.Spec.Subject.Username {
					remove = true
					break
				}
				if !remove {
					groups = append(groups, group)
				}
			}
			project.Spec.Roles[idx].Groups = groups
		}
	}
}

// updateProjectPolicies will update the given project to match all Policies
// defined by the given RoleTemplate for the role name specified in the rt.
// It will also update the description and revoke any JWT tokens that were
// associated with this specific role. Noop if the given rt is nil.
func updateProjectPolicies(project *argocd.AppProject, ar *api.AccessRequest, rt *api.RoleTemplate) {
	if rt == nil {
		return
	}
	roleName := rt.AppProjectRoleName(ar.Spec.Application.Name, ar.Spec.Application.Namespace)
	for idx, role := range project.Spec.Roles {
		if role.Name == roleName {
			project.Spec.Roles[idx].Description = rt.Spec.Description
			project.Spec.Roles[idx].Policies = rt.Spec.Policies
			project.Spec.Roles[idx].JWTTokens = []argocd.JWTToken{}
		}
	}
}

// addSubjectInRole will associate the given AccessRequest subject in the
// specific role in the given project.
func addSubjectInRole(project *argocd.AppProject, ar *api.AccessRequest, rt *api.RoleTemplate) {
	roleFound := false
	roleName := rt.AppProjectRoleName(ar.Spec.Application.Name, ar.Spec.Application.Namespace)
	for idx, role := range project.Spec.Roles {
		if role.Name == roleName {
			roleFound = true
			hasAccess := false
			for _, group := range role.Groups {
				if group == ar.Spec.Subject.Username {
					hasAccess = true
					break
				}
			}
			if !hasAccess {
				project.Spec.Roles[idx].Groups = append(project.Spec.Roles[idx].Groups, ar.Spec.Subject.Username)
			}
		}
	}
	if !roleFound {
		addRoleInProject(project, ar, rt)
	}
}

// addRoleInProject will initialize the role owned by the ephemeral-access
// controller and associate it in the given project.
func addRoleInProject(project *argocd.AppProject, ar *api.AccessRequest, rt *api.RoleTemplate) {
	groups := []string{ar.Spec.Subject.Username}
	role := argocd.ProjectRole{
		Name:        rt.AppProjectRoleName(ar.Spec.Application.Name, ar.Spec.Application.Namespace),
		Description: rt.Spec.Description,
		Policies:    rt.Spec.Policies,
		Groups:      groups,
	}
	project.Spec.Roles = append(project.Spec.Roles, role)
}

// AllowedResponse defines the response that will be returned by permission
// verifier plugins.
type AllowedResponse struct {
	Allowed bool
	Message string
}

// TODO
// 0. implement the plugin system
// 1. verify if user is sudoer
// 2. verify if CR is approved
func Allowed(ctx context.Context, ar *api.AccessRequest, app *argocd.Application) (AllowedResponse, error) {
	return AllowedResponse{Allowed: true}, nil
}
