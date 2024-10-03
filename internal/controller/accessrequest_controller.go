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
	"github.com/argoproj-labs/ephemeral-access/internal/controller/config"
	"github.com/argoproj-labs/ephemeral-access/pkg/log"
)

// AccessRequestReconciler reconciles a AccessRequest object
type AccessRequestReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Service *Service
	Config  config.ControllerConfigurer
}

const (
	// AccessRequestFinalizerName defines the name of the AccessRequest finalizer
	// managed by this controller
	AccessRequestFinalizerName = "accessrequest.ephemeral-access.argoproj-labs.io/finalizer"
	roleTemplateField          = ".spec.roleTemplateName"
	projectField               = ".status.targetProject"
	userField                  = ".spec.subject.username"
	appField                   = ".spec.application.name"
	appNamespaceField          = ".spec.application.namespace"
)

// +kubebuilder:rbac:groups=ephemeral-access.argoproj-labs.io,resources=accessrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ephemeral-access.argoproj-labs.io,resources=accessrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ephemeral-access.argoproj-labs.io,resources=accessrequests/finalizers,verbs=update
// +kubebuilder:rbac:groups=ephemeral-access.argoproj-labs.io,resources=roletemplates,verbs=get;list;watch
// +kubebuilder:rbac:groups=ephemeral-access.argoproj-labs.io,resources=roletemplates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ephemeral-access.argoproj-labs.io,resources=roletemplates/finalizers,verbs=update
// +kubebuilder:rbac:groups=argoproj.io,resources=appproject,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=argoproj.io,resources=application,verbs=get

// Reconcile is the main function that will be invoked on every change in
// AccessRequests desired state. It will:
//  1. Handle the accessrequest finalizer
//  2. Validate the AccessRequest
//  3. Verify if AccessRequest is expired
//     3.1 If so, remove the user from the elevated role
//     3.2 Update the accessrequest status to "expired"
//  4. Verify if user has the necessary access to be promoted
//     4.1 If they don't, update the accessrequest status to "denied"
//  5. Invoke preconfigured plugin to check if access can be granted
//  8. Assign user in the desired role in the AppProject
//  9. Update the accessrequest status to "granted"
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

	logger.Debug("Validating AccessRequest")
	err = r.Validate(ctx, ar)
	if err != nil {
		if _, ok := err.(*AccessRequestConflictError); ok {
			logger.Error(err, "AccessRequest conflict error")
			ar.UpdateStatusHistory(api.InvalidStatus, err.Error())
			err = r.Status().Update(ctx, ar)
			if err != nil {
				return reconcile.Result{}, fmt.Errorf("error updating status to invalid: %s", err)
			}
			return ctrl.Result{}, nil
		}
		logger.Info(fmt.Sprintf("Validation error: %s", err))
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

	renderedRt, err := roleTemplate.Render(application.Spec.Project, application.GetName(), application.GetNamespace())
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("roleTemplate error: %w", err)
	}

	// initialize the status if not done yet
	if ar.Status.RequestState == "" {
		logger.Debug("Initializing status")
		ar.UpdateStatusHistory(api.RequestedStatus, "")
		ar.Status.TargetProject = application.Spec.Project
		ar.Status.RoleTemplateHash = RoleTemplateHash(renderedRt)
		r.Status().Update(ctx, ar)
	}

	logger.Debug("Handling permission")
	status, err := r.Service.HandlePermission(ctx, ar, application, renderedRt)
	if err != nil {
		logger.Error(err, "HandlePermission error")
		return ctrl.Result{}, fmt.Errorf("error handling permission: %w", err)
	}

	result := buildResult(status, ar, r.Config.ControllerRequeueInterval())
	logger.Info("Reconciliation concluded", "status", status, "result", result)
	return result, nil
}

type AccessRequestConflictError struct {
	message string
}

func (e *AccessRequestConflictError) Error() string {
	return e.message
}

func NewAccessRequestConflictError(msg string) *AccessRequestConflictError {
	return &AccessRequestConflictError{
		message: msg,
	}
}

// Validate will verify if there are existing AccessRequests for the same
// user/app/role already in progress.
func (r *AccessRequestReconciler) Validate(ctx context.Context, ar *api.AccessRequest) error {
	arList, err := r.findAccessRequestsByUserAndApp(ctx,
		ar.GetNamespace(),
		ar.Spec.Subject.Username,
		ar.Spec.Application.Name,
		ar.Spec.Application.Namespace)
	if err != nil {
		return fmt.Errorf("error finding AccessRequests by user and app: %w", err)
	}
	for _, arResp := range arList.Items {
		// skip if it is the same AccessRequest
		if arResp.GetName() == ar.GetName() &&
			arResp.GetNamespace() == ar.GetNamespace() {
			continue
		}
		// skip if the request is for different role template
		if arResp.Spec.RoleTemplateName != ar.Spec.RoleTemplateName {
			continue
		}
		// if the existing request is pending, granted, or empty (not reconciled yet)
		// then the new request is a duplicate and must be rejected
		if arResp.Status.RequestState == api.GrantedStatus ||
			arResp.Status.RequestState == api.RequestedStatus ||
			arResp.Status.RequestState == "" {
			return NewAccessRequestConflictError(fmt.Sprintf("found existing AccessRequest (%s/%s) in %s state", arResp.GetNamespace(), arResp.GetName(), string(arResp.Status.RequestState)))
		}
	}
	return nil
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
func buildResult(status api.Status, ar *api.AccessRequest, requeueInterval time.Duration) ctrl.Result {
	result := ctrl.Result{}
	switch status {
	case api.RequestedStatus:
		result.Requeue = true
		result.RequeueAfter = requeueInterval
	case api.GrantedStatus:
		result.Requeue = true
		result.RequeueAfter = ar.Status.ExpiresAt.Sub(time.Now())
	}
	return result
}

// roleTemplateUpdated will return true if the RoleTemplate previously associated with
// the given AccessRequest is different than the given what is defined in the given rt.
// Will return false otherwise.
func roleTemplateUpdated(ar *api.AccessRequest, rt *api.RoleTemplate) bool {
	return ar.Status.RoleTemplateHash != RoleTemplateHash(rt)
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
		Name:      ar.Spec.RoleTemplateName,
		Namespace: ar.GetNamespace(),
	}
	err := r.Get(ctx, objKey, roleTemplate)
	if err != nil {
		return nil, err
	}
	return roleTemplate, nil
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
			if err := r.Service.RemoveArgoCDAccess(ctx, ar, rt); err != nil {
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

// callReconcileForRoleTemplate will retrieve all AccessRequest resources referencing
// the given roleTemplate and build a list of reconcile requests to be sent to the
// controller. Only non-concluded AccessRequests will be added to the reconciliation
// list. An AccessRequest is defined as concluded if their status is Expired or Denied.
func (r *AccessRequestReconciler) callReconcileForRoleTemplate(ctx context.Context, roleTemplate client.Object) []reconcile.Request {
	logger := log.FromContext(ctx)
	logger.Debug(fmt.Sprintf("RoleTemplate %s updated: searching for associated AccessRequests...", roleTemplate.GetName()))
	attachedAccessRequests := &api.AccessRequestList{}
	listOps := &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(roleTemplateField, roleTemplate.GetName()),
	}
	err := r.List(ctx, attachedAccessRequests, listOps)
	if err != nil {
		logger.Error(err, "findObjectsForRoleTemplate error: list k8s resources error")
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
	totalRequests := len(requests)
	if totalRequests == 0 {
		return nil
	}
	logger.Debug(fmt.Sprintf("Found %d associated AccessRequests with RoleTemplate %s. Reconciling...", totalRequests, roleTemplate.GetName()))
	return requests
}

// findAccessRequestsByUserAndApp will list all AccessRequests in the given namespace
// filtering by the given username, appName and appNamespace.
func (r *AccessRequestReconciler) findAccessRequestsByUserAndApp(ctx context.Context, namespace, username, appName, appNamespace string) (*api.AccessRequestList, error) {
	arList := &api.AccessRequestList{}
	selector := fields.SelectorFromSet(
		fields.Set{
			userField:         username,
			appField:          appName,
			appNamespaceField: appNamespace,
		})

	listOps := &client.ListOptions{
		FieldSelector: selector,
		Namespace:     namespace,
	}

	err := r.List(ctx, arList, listOps)
	if err != nil {
		return nil, fmt.Errorf("List error: %w", err)
	}
	return arList, nil
}

// callReconcileForProject will retrieve all AccessRequest resources referencing
// the given project and build a list of reconcile requests to be sent to the
// controller. Only non-concluded AccessRequests will be added to the reconciliation
// list. An AccessRequest is defined as concluded if their status is Expired or Denied.
func (r *AccessRequestReconciler) callReconcileForProject(ctx context.Context, project client.Object) []reconcile.Request {
	logger := log.FromContext(ctx)
	logger.Debug(fmt.Sprintf("Project %s updated: searching for associated AccessRequests...", project.GetName()))
	associatedAccessRequests := &api.AccessRequestList{}
	listOps := &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(projectField, project.GetName()),
		// This makes a requirement that the AccessRequest has to live in the
		// same namespace as the AppProject.
		Namespace: project.GetNamespace(),
	}
	err := r.List(ctx, associatedAccessRequests, listOps)
	if err != nil {
		logger.Error(err, "findObjectsForProject error: list k8s resources error")
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(associatedAccessRequests.Items))
	for i, item := range associatedAccessRequests.Items {
		if !isConcluded(&item) {
			requests[i] = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      item.GetName(),
					Namespace: item.GetNamespace(),
				},
			}
		}
	}
	totalRequests := len(requests)
	if totalRequests == 0 {
		return nil
	}
	logger.Debug(fmt.Sprintf("Found %d associated AccessRequests with project %s. Reconciling...", totalRequests, project.GetName()))
	return requests
}

// createProjectIndex will create an AccessRequest index by project to allow
// fetching all objects referencing a given AppProject.
func createProjectIndex(mgr ctrl.Manager) error {
	err := mgr.GetFieldIndexer().
		IndexField(context.Background(), &api.AccessRequest{}, projectField,
			func(rawObj client.Object) []string {
				ar := rawObj.(*api.AccessRequest)
				if ar.Status.TargetProject == "" {
					return nil
				}
				return []string{ar.Status.TargetProject}
			})
	if err != nil {
		return fmt.Errorf("error creating project field index: %w", err)
	}
	return nil
}

// createRoleTemplateIndex create an AccessRequest index by role template name
// to allow fetching all objects referencing a given RoleTemplate.
func createRoleTemplateIndex(mgr ctrl.Manager) error {
	err := mgr.GetFieldIndexer().
		IndexField(context.Background(), &api.AccessRequest{}, roleTemplateField, func(rawObj client.Object) []string {
			ar := rawObj.(*api.AccessRequest)
			if ar.Spec.RoleTemplateName == "" {
				return nil
			}
			return []string{ar.Spec.RoleTemplateName}
		})
	if err != nil {
		return fmt.Errorf("error creating roleTemplateName field index: %w", err)
	}
	return nil
}

// createRoleTemplateIndex will create an AccessRequest index by the following fields:
// - .spec.subject.username
// - .spec.application.name
// - .spec.application.namespace
func createUserAppIndex(mgr ctrl.Manager) error {
	err := mgr.GetFieldIndexer().
		IndexField(context.Background(), &api.AccessRequest{}, userField, func(rawObj client.Object) []string {
			ar := rawObj.(*api.AccessRequest)
			if ar.Spec.Subject.Username == "" {
				return nil
			}
			return []string{ar.Spec.Subject.Username}
		})
	if err != nil {
		return fmt.Errorf("error creating username field index: %w", err)
	}
	err = mgr.GetFieldIndexer().
		IndexField(context.Background(), &api.AccessRequest{}, appField, func(rawObj client.Object) []string {
			ar := rawObj.(*api.AccessRequest)
			if ar.Spec.Application.Name == "" {
				return nil
			}
			return []string{ar.Spec.Application.Name}
		})
	if err != nil {
		return fmt.Errorf("error creating application name field index: %w", err)
	}
	err = mgr.GetFieldIndexer().
		IndexField(context.Background(), &api.AccessRequest{}, appNamespaceField, func(rawObj client.Object) []string {
			ar := rawObj.(*api.AccessRequest)
			if ar.Spec.Application.Namespace == "" {
				return nil
			}
			return []string{ar.Spec.Application.Namespace}
		})
	if err != nil {
		return fmt.Errorf("error creating application namespace field index: %w", err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AccessRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := createProjectIndex(mgr)
	if err != nil {
		return fmt.Errorf("project index error: %w", err)
	}
	err = createRoleTemplateIndex(mgr)
	if err != nil {
		return fmt.Errorf("roleTemplate index error: %w", err)
	}
	err = createUserAppIndex(mgr)
	if err != nil {
		return fmt.Errorf("userapp index error: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&api.AccessRequest{}).
		Watches(&api.RoleTemplate{},
			handler.EnqueueRequestsFromMapFunc(r.callReconcileForRoleTemplate),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(&argocd.AppProject{},
			handler.EnqueueRequestsFromMapFunc(r.callReconcileForProject),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Complete(r)
}
