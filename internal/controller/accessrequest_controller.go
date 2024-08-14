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
	"crypto/sha1"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields" // Required for Watching
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types" // Required for Watching
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder" // Required for Watching
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"   // Required for Watching
	"sigs.k8s.io/controller-runtime/pkg/predicate" // Required for Watching
	"sigs.k8s.io/controller-runtime/pkg/reconcile" // Required for Watching

	// "sigs.k8s.io/controller-runtime/pkg/source"    // Required for Watching

	argocd "github.com/argoproj-labs/ephemeral-access/api/argoproj/v1alpha1"
	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/internal/log"
	"github.com/cnf/structhash"
)

// AccessRequestReconciler reconciles a AccessRequest object
type AccessRequestReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	// AccessRequestFinalizerName defines the name of the AccessRequest finalizer
	// managed by this controller
	AccessRequestFinalizerName = "accessrequest.ephemeral-access.argoproj-labs.io/finalizer"
	FieldOwnerEphemeralAccess  = "ephemeral-access-controller"
	roleTemplateField          = ".spec.roleTemplateName"
)

// +kubebuilder:rbac:groups=ephemeral-access.argoproj-labs.io,resources=accessrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ephemeral-access.argoproj-labs.io,resources=accessrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ephemeral-access.argoproj-labs.io,resources=accessrequests/finalizers,verbs=update
// +kubebuilder:rbac:groups=ephemeral-access.argoproj-labs.io,resources=roletemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ephemeral-access.argoproj-labs.io,resources=roletemplates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ephemeral-access.argoproj-labs.io,resources=roletemplates/finalizers,verbs=update
// +kubebuilder:rbac:groups=argoproj.io,resources=appproject,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=argoproj.io,resources=application,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// 1. handle finalizer
// 2. validate AccessRequest
// 3. verify if accessrequest is expired and status is "granted"
// 3.1 if so, remove the user from the elevated role
// 3.2 update the accessrequest status to "expired"
// 3.3 return
// 4. verify if user has the necessary access to be promoted
// 4.1 if they don't, update the accessrequest status to "denied"
// 4.2 return
// 5. verify if CR is approved
// 6. retrieve the Application
// 7. retrieve the AppProject
// 8. assign user in the desired role in the AppProject
// 9. update the accessrequest status to "granted"
// 10. set the RequeueAfter in Result
func (r *AccessRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciliation started")

	ar := &api.AccessRequest{}
	if err := r.Get(ctx, req.NamespacedName, ar); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("Object deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Error retrieving AccessRequest from k8s")
		return ctrl.Result{}, err
	}

	// check if the object is being deleted and properly handle it
	logger.Debug("Handling finalizer")
	deleted, err := r.handleFinalizer(ctx, ar)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error handling finalizer: %w", err)
	}
	// stop the reconciliation as the object was deleted
	if deleted {
		logger.Debug("Object deleted")
		return ctrl.Result{}, nil
	}

	// if isConcluded(ar) {
	// 	result := ctrl.Result{}
	// 	logger.Info("Reconciliation not required", "status", ar.Status.RequestState, "result", result)
	// 	return result, nil
	// }

	logger.Debug("Validating AccessRequest")
	err = ar.Validate()
	if err != nil {
		logger.Info("Validation error: %s", err)
		return ctrl.Result{}, fmt.Errorf("error validating the AccessRequest: %w", err)
	}

	application, err := r.getApplication(ctx, ar)
	if err != nil {
		// TODO send an event to explain why the access request is failing
		return ctrl.Result{}, fmt.Errorf("error getting Argo CD Application: %w", err)
	}

	roleTemplate, err := r.getRoleTemplate(ctx, ar)
	if err != nil {
		// TODO send an event to explain why the access request is failing
		return ctrl.Result{}, fmt.Errorf("error getting RoleTemplate %s: %w", ar.Spec.RoleTemplateName, err)
	}

	renderedRt, err := roleTemplate.Render(application.Spec.Project, application.GetName(), application.GetNamespace(), application.Spec.Destination.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("roleTemplate error: %w", err)
	}

	// initialize the status if not done yet
	if ar.Status.RequestState == "" {
		logger.Debug("Initializing status")
		ar.UpdateStatusHistory(api.RequestedStatus, "")
		ar.Status.TargetProject = application.Spec.Project
		ar.Status.RoleTemplateHash = roleTemplateHash(renderedRt)
		r.Status().Update(ctx, ar)
	}

	logger.Debug("Handling permission")
	status, err := r.handlePermission(ctx, ar, application, renderedRt)
	if err != nil {
		logger.Error(err, "HandlePermission error")
		return ctrl.Result{}, fmt.Errorf("error handling permission: %w", err)
	}

	result := buildResult(status, ar)
	logger.Info("Reconciliation concluded", "status", status, "result", result)
	return result, nil
}

func roleTemplateHash(rt *api.RoleTemplate) string {
	return fmt.Sprintf("%x", sha1.Sum(structhash.Dump(rt, 1)))
}

// isConcluded will check the status of the given AccessRequest
// to determine if it is concluded. Concluded AccessRequest means
// it is in Denied or Expired status.
func isConcluded(ar *api.AccessRequest) bool {
	switch ar.Status.RequestState {
	case api.DeniedStatus, api.ExpiredStatus:
		return true
	default:
		return false
	}
}

// buildResult will verify the given status and determine when this access
// request should be requeued.
func buildResult(status api.Status, ar *api.AccessRequest) ctrl.Result {
	result := ctrl.Result{}
	switch status {
	case api.RequestedStatus:
		result.Requeue = true
		// TODO add a controller requeue configuration
		result.RequeueAfter = time.Minute * 3
	case api.GrantedStatus:
		result.Requeue = true
		result.RequeueAfter = ar.Status.ExpiresAt.Sub(time.Now())
	}
	return result
}

// handlePermission will analyse the given ar and proceed with granting
// or removing Argo CD access for the subjects listed in the AccessRequest.
// The following validations will be executed:
//  1. Check if the given ar is expired. If so, subjects will be removed from
//     the Argo CD role.
//  2. Check if the subjects are allowed to be assigned in the given AccessRequest
//     target role. If so, it will proceed with grating Argo CD access. Otherwise
//     it will return DeniedStatus.
//
// It will update the AccessRequest status accordingly with the situation.
func (r *AccessRequestReconciler) handlePermission(ctx context.Context, ar *api.AccessRequest, app *argocd.Application, rt *api.RoleTemplate) (api.Status, error) {
	logger := log.FromContext(ctx)

	if ar.IsExpiring() {
		logger.Info("AccessRequest is expired")
		err := r.handleAccessExpired(ctx, ar, rt)
		if err != nil {
			return "", fmt.Errorf("error handling access expired: %w", err)
		}
		return api.ExpiredStatus, nil
	}

	resp, err := r.Allowed(ctx, ar, app)
	if err != nil {
		return "", fmt.Errorf("error verifying if subject is allowed: %w", err)
	}
	if !resp.Allowed {
		rtHash := roleTemplateHash(rt)
		err = r.updateStatus(ctx, ar, api.DeniedStatus, resp.Message, rtHash)
		if err != nil {
			return "", fmt.Errorf("error updating access request status to denied: %w", err)
		}
		return api.DeniedStatus, nil
	}

	details := ""
	status, err := r.grantArgoCDAccess(ctx, ar, rt)
	if err != nil {
		details = fmt.Sprintf("Error granting Argo CD Access: %s", err)
	}
	// only update status if the current state is different
	if ar.Status.RequestState != status {
		rtHash := roleTemplateHash(rt)
		err = r.updateStatus(ctx, ar, status, details, rtHash)
		if err != nil {
			return "", fmt.Errorf("error updating access request status to granted: %w", err)
		}
	}
	return status, nil
}

// roleTemplateUpdated will return true if the RoleTemplate previously associated with
// the given AccessRequest is different than the given what is defined in the given rt.
// Will return false otherwise.
func roleTemplateUpdated(ar *api.AccessRequest, rt *api.RoleTemplate) bool {
	hash := roleTemplateHash(rt)
	if ar.Status.RoleTemplateHash != hash {
		return true
	}
	return false
}

// updateStatusWithRetry will retrieve the latest AccessRequest state before
// attempting to update its status. In case of conflict error, it will retry
// using the DefaultRetry backoff which has the following configs:
//
//	Steps: 5, Duration: 10 milliseconds, Factor: 1.0, Jitter: 0.1
func (r *AccessRequestReconciler) updateStatusWithRetry(ctx context.Context, ar *api.AccessRequest, status api.Status, details string, rtHash string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := r.Get(ctx, client.ObjectKeyFromObject(ar), ar)
		if err != nil {
			return err
		}
		return r.updateStatus(ctx, ar, status, details, rtHash)
	})
}

// updateStatus will update the given AccessRequest status field with the
// given status and details.
func (r *AccessRequestReconciler) updateStatus(ctx context.Context, ar *api.AccessRequest, status api.Status, details string, rtHash string) error {
	// if it is already updated skip
	if ar.Status.RequestState == status && ar.Status.RoleTemplateHash == rtHash {
		return nil
	}
	ar.UpdateStatusHistory(status, details)
	ar.Status.RoleTemplateHash = rtHash
	return r.Status().Update(ctx, ar)
}

func (r *AccessRequestReconciler) getApplication(ctx context.Context, ar *api.AccessRequest) (*argocd.Application, error) {
	application := &argocd.Application{}
	objKey := client.ObjectKey{
		Namespace: ar.Spec.Application.Namespace,
		Name:      ar.Spec.Application.Name,
	}
	err := r.Get(ctx, objKey, application)
	if err != nil {
		return nil, err
	}
	return application, nil
}

func (r *AccessRequestReconciler) getRoleTemplate(ctx context.Context, ar *api.AccessRequest) (*api.RoleTemplate, error) {
	roleTemplate := &api.RoleTemplate{}
	objKey := client.ObjectKey{
		Namespace: ar.Spec.Application.Namespace,
		Name:      ar.Spec.RoleTemplateName,
	}
	err := r.Get(ctx, objKey, roleTemplate)
	if err != nil {
		return nil, err
	}
	return roleTemplate, nil
}

// removeArgoCDAccess will remove the subjects in the given AccessRequest from
// the given ar.TargetRoleName from the Argo CD project referenced in the
// ar.Spec.AppProject. The AppProject update will be executed via a patch with
// optimistic lock enabled. It will retry in case of AppProject conflict is
// identied.
func (r *AccessRequestReconciler) removeArgoCDAccess(ctx context.Context, ar *api.AccessRequest, rt *api.RoleTemplate) error {
	logger := log.FromContext(ctx)
	logger.Info("Removing Argo CD Access")

	project := &argocd.AppProject{}
	objKey := client.ObjectKey{
		Namespace: ar.Spec.Application.Namespace,
		Name:      ar.Status.TargetProject,
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := r.Get(ctx, objKey, project)
		if err != nil {
			e := fmt.Errorf("error getting Argo CD Project %s/%s: %w", objKey.Namespace, objKey.Name, err)
			return client.IgnoreNotFound(e)
		}
		patch := client.MergeFromWithOptions(project.DeepCopy(), client.MergeFromWithOptimisticLock{})

		logger.Debug("Removing subjects from role")
		removeSubjectsFromRole(project, ar, rt)
		// this is necessary to make sure that the AppProject role managed by
		// this controller is always in sync with what is defined in the
		// RoleTemplate
		updateProjectPolicies(project, ar, rt)

		logger.Debug("Patching AppProject")
		opts := []client.PatchOption{client.FieldOwner(FieldOwnerEphemeralAccess)}
		err = r.Patch(ctx, project, patch, opts...)
		if err != nil {
			return fmt.Errorf("error patching Argo CD Project %s/%s: %w", objKey.Namespace, objKey.Name, err)
		}
		return nil
	})
}

// removeSubjectsFromRole will iterate ovet the roles in the given project and
// remove the subjects from the given AccessRequest from the role specified in
// the ar.TargetRoleName.
func removeSubjectsFromRole(project *argocd.AppProject, ar *api.AccessRequest, rt *api.RoleTemplate) {
	roleName := rt.AppProjectRoleName(ar.Spec.Application.Name, ar.Spec.Application.Namespace)
	for idx, role := range project.Spec.Roles {
		if role.Name == roleName {
			groups := []string{}
			for _, group := range role.Groups {
				remove := false
				for _, subject := range ar.Spec.Subjects {
					if group == subject.Username {
						remove = true
						break
					}
				}
				if !remove {
					groups = append(groups, group)
				}
			}
			project.Spec.Roles[idx].Groups = groups
		}
	}
}

// grantArgoCDAccess will associate the given AccessRequest subjects in the
// Argo CD AppProject specified in the ar.Spec.AppProject in the role defined
// in ar.TargetRoleName. The AppProject update will be executed via a patch with
// optimistic lock enabled. It Will retry in case of AppProject conflict is
// identied.
func (r *AccessRequestReconciler) grantArgoCDAccess(ctx context.Context, ar *api.AccessRequest, rt *api.RoleTemplate) (api.Status, error) {
	logger := log.FromContext(ctx)
	logger.Info("Granting Argo CD Access")
	project := &argocd.AppProject{}
	objKey := client.ObjectKey{
		Namespace: ar.Spec.Application.Namespace,
		Name:      ar.Status.TargetProject,
	}
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := r.Get(ctx, objKey, project)
		if err != nil {
			return fmt.Errorf("error getting Argo CD Project %s/%s: %s", objKey.Namespace, objKey.Name, err)
		}
		patch := client.MergeFromWithOptions(project.DeepCopy(), client.MergeFromWithOptimisticLock{})

		logger.Debug("Adding subjects in role")
		addSubjectsInRole(project, ar, rt)
		// this is necessary to make sure that the AppProject role managed by
		// this controller is always in sync with what is defined in the
		// RoleTemplate
		updateProjectPolicies(project, ar, rt)

		logger.Debug("Patching AppProject")
		opts := []client.PatchOption{client.FieldOwner("ephemeral-access-controller")}
		err = r.Patch(ctx, project, patch, opts...)
		if err != nil {
			return fmt.Errorf("error patching Argo CD Project %s/%s: %w", objKey.Namespace, objKey.Name, err)
		}

		return nil
	})
	if err != nil {
		return api.DeniedStatus, err
	}
	return api.GrantedStatus, nil
}

// addSubjectsInRole will associate the given AccessRequest subjects in the
// specific role in the given project.
func addSubjectsInRole(project *argocd.AppProject, ar *api.AccessRequest, rt *api.RoleTemplate) {
	roleFound := false
	roleName := rt.AppProjectRoleName(ar.Spec.Application.Name, ar.Spec.Application.Namespace)
	for idx, role := range project.Spec.Roles {
		if role.Name == roleName {
			roleFound = true
			for _, subject := range ar.Spec.Subjects {
				hasAccess := false
				for _, group := range role.Groups {
					if group == subject.Username {
						hasAccess = true
						break
					}
				}
				if !hasAccess {
					project.Spec.Roles[idx].Groups = append(project.Spec.Roles[idx].Groups, subject.Username)
				}
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
	groups := []string{}
	for _, subject := range ar.Spec.Subjects {
		groups = append(groups, subject.Username)
	}
	role := argocd.ProjectRole{
		Name:        rt.AppProjectRoleName(ar.Spec.Application.Name, ar.Spec.Application.Namespace),
		Description: rt.Spec.Description,
		// TODO replace from template
		Policies: rt.Spec.Policies,
		Groups:   groups,
	}
	project.Spec.Roles = append(project.Spec.Roles, role)
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
			project.Spec.Roles[idx].JWTTokens = nil
		}
	}
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
func (r *AccessRequestReconciler) Allowed(ctx context.Context, ar *api.AccessRequest, app *argocd.Application) (AllowedResponse, error) {
	return AllowedResponse{Allowed: true}, nil
}

// handleAccessExpired will remove the Argo CD access for the subject and update the
// AccessRequest status field.
func (r *AccessRequestReconciler) handleAccessExpired(ctx context.Context, ar *api.AccessRequest, rt *api.RoleTemplate) error {
	err := r.removeArgoCDAccess(ctx, ar, rt)
	if err != nil {
		return fmt.Errorf("error removing access for expired request: %w", err)
	}
	hash := roleTemplateHash(rt)
	err = r.updateStatus(ctx, ar, api.ExpiredStatus, "", hash)
	if err != nil {
		return fmt.Errorf("error updating access request status to expired: %w", err)
	}
	return nil
}

// handleFinalizer will check if the AccessRequest is being deleted and
// proceed with the necessary clean up logic if so. If the object is not
// being deleted, it will register the AccessRequest finalizer in the live
// state. The function will return a boolean value to determine if the object
// was deleted.
func (r *AccessRequestReconciler) handleFinalizer(ctx context.Context, ar *api.AccessRequest) (bool, error) {

	// examine DeletionTimestamp to determine if object is under deletion
	if ar.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have the
		// finalizer, then we register it.
		if !controllerutil.ContainsFinalizer(ar, AccessRequestFinalizerName) {
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				err := r.Get(ctx, client.ObjectKeyFromObject(ar), ar)
				if err != nil {
					return err
				}
				controllerutil.AddFinalizer(ar, AccessRequestFinalizerName)
				return r.Update(ctx, ar)

			})
			if err != nil {
				return false, fmt.Errorf("error adding finalizer: %w", err)
			}
		}
		return false, nil
	}

	// The object is being deleted
	if controllerutil.ContainsFinalizer(ar, AccessRequestFinalizerName) {
		// if the access request is not expired yet then
		// execute the cleanup procedure before removing the finalizer
		if ar.Status.RequestState != api.ExpiredStatus {
			// this is a best effort to update policies that eventually changed
			// in the project. Errors are ignored as it is more important to
			// remove the user from the role.
			rt, _ := r.getRoleTemplate(ctx, ar)
			if err := r.removeArgoCDAccess(ctx, ar, rt); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried.
				return false, fmt.Errorf("error cleaning up Argo CD access: %w", err)
			}
		}

		// remove our finalizer from the list and update it.
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			err := r.Get(ctx, client.ObjectKeyFromObject(ar), ar)
			if err != nil {
				return client.IgnoreNotFound(err)
			}
			controllerutil.RemoveFinalizer(ar, AccessRequestFinalizerName)
			return r.Update(ctx, ar)

		})
		if err != nil {
			return false, fmt.Errorf("error removing finalizer: %w", err)
		}
	}
	return true, nil
}

// findObjectsForRoleTemplate will retrieve all AccessRequest resources referencing
// the given roleTemplate and build a list of reconcile requests to be sent to the
// controller. Only non-concluded AccessRequests will be added to the reconciliation
// list. An AccessRequest is defined as concluded if their status is Expired or Denied.
func (r *AccessRequestReconciler) findObjectsForRoleTemplate(ctx context.Context, roleTemplate client.Object) []reconcile.Request {
	attachedAccessRequests := &api.AccessRequestList{}
	listOps := &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(roleTemplateField, roleTemplate.GetName()),
		Namespace:     roleTemplate.GetNamespace(),
	}
	err := r.List(ctx, attachedAccessRequests, listOps)
	if err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(attachedAccessRequests.Items))
	for i, item := range attachedAccessRequests.Items {
		if !isConcluded(&item) {
			requests[i] = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      item.GetName(),
					Namespace: item.GetNamespace(),
				},
			}
		}
	}
	if len(requests) == 0 {
		return nil
	}
	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *AccessRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().
		IndexField(context.Background(), &api.AccessRequest{}, roleTemplateField, func(rawObj client.Object) []string {
			// Extract the ConfigMap name from the ConfigDeployment Spec, if one is provided
			ar := rawObj.(*api.AccessRequest)
			if ar.Spec.RoleTemplateName == "" {
				return nil
			}
			return []string{ar.Spec.RoleTemplateName}
		}); err != nil {
		return fmt.Errorf("error creating index field for roleTemplateName: %w", err)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.AccessRequest{}).
		Watches(&api.RoleTemplate{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForRoleTemplate),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Complete(r)
}
