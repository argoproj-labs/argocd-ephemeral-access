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
	"errors"
	"fmt"
	"slices"
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
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"   // Required for Watching
	"sigs.k8s.io/controller-runtime/pkg/predicate" // Required for Watching
	"sigs.k8s.io/controller-runtime/pkg/reconcile" // Required for Watching

	argocd "github.com/argoproj-labs/argocd-ephemeral-access/api/argoproj/v1alpha1"
	api "github.com/argoproj-labs/argocd-ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/argocd-ephemeral-access/internal/controller/config"
	"github.com/argoproj-labs/argocd-ephemeral-access/internal/controller/metrics"
	"github.com/argoproj-labs/argocd-ephemeral-access/pkg/log"
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
	roleTemplateNameField      = ".spec.role.template.name"
	roleTemplateNamespaceField = ".spec.role.template.namespace"
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
// +kubebuilder:rbac:groups=argoproj.io,resources=appprojects,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=argoproj.io,resources=applications,verbs=get;list;watch

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

	ar := &api.AccessRequest{}
	if err := r.Get(ctx, req.NamespacedName, ar); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("Object deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Error retrieving AccessRequest from k8s")
		return ctrl.Result{}, err
	}
	values := []interface{}{
		"subject", ar.Spec.Subject.Username,
		"role", ar.Spec.Role.FriendlyName,
		"duration", ar.Spec.Duration.Duration.String(),
		"application.name", ar.Spec.Application.Name,
		"application.namespace", ar.Spec.Application.Namespace,
	}
	logger = logger.WithValues(values...)
	ctx = log.IntoContext(ctx, logger)
	logger.Info("Reconciliation started")

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

	ttlExceeded, err := r.handleTTL(ctx, ar)
	if err != nil {
		logger.Error(err, "Error handling TTL")
		return ctrl.Result{}, fmt.Errorf("error handling TTL: %w", err)
	}
	// stop the reconciliation if the TTL was exceeded
	if ttlExceeded {
		logger.Debug("AccessRequest TTL exceeded: cleaning up...")
		return ctrl.Result{}, nil
	}

	// stop if the reconciliation was previously concluded
	if ar.IsConcluded() {
		logger.Debug(fmt.Sprintf("Reconciliation concluded as the AccessRequest is %s: skipping...", string(ar.Status.RequestState)))
		return ctrl.Result{}, nil
	}

	// check if the AccessRequest is in conflict with existing requests
	err = r.HandleConflict(ctx, ar)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error handling conflict: %w", err)
	}

	logger.Debug("Handling permission")
	status, err := r.Service.HandlePermission(ctx, ar)
	if err != nil {
		logger.Error(err, "HandlePermission error")
		return ctrl.Result{}, fmt.Errorf("error handling permission: %w", err)
	}

	timeout, err := r.handleRequestTimeout(ctx, ar)
	if err != nil {
		logger.Error(err, "Error handling timeout")
		return ctrl.Result{}, fmt.Errorf("error handling timeout: %w", err)
	}
	if timeout {
		logger.Info(fmt.Sprintf("AccessRequest timeout (%s) exceeded. Stopping reconciliation...", r.Config.ControllerRequestTimeout().String()))
		status = api.TimeoutStatus
	}

	result := buildResult(status, ar, r.Config)
	metrics.IncrementAccessRequestCounter(status)
	logger.Info("Reconciliation concluded", "status", status, "result", result)
	return result, nil
}

// HandleConflict processes AccessRequest objects to handle validation conflicts.
// If a conflict is detected, it updates the AccessRequest status to invalid.
// Returns an AccessRequestConflictError if a conflict scenario is identified.
func (r *AccessRequestReconciler) HandleConflict(ctx context.Context, ar *api.AccessRequest) error {
	logger := log.FromContext(ctx)
	logger.Debug("Validating AccessRequest")
	err := r.ValidateConflict(ctx, ar)
	if err != nil {
		accessRequestConflictError := &AccessRequestConflictError{}
		if errors.As(err, &accessRequestConflictError) {
			logger.Error(err, "AccessRequest conflict error")
			ar.UpdateStatusHistory(api.InvalidStatus, err.Error())
			err = r.Status().Update(ctx, ar)
			if err != nil {
				return fmt.Errorf("error updating status to invalid: %w", err)
			}
			metrics.IncrementAccessRequestCounter(api.InvalidStatus)
			return nil
		}
		logger.Info(fmt.Sprintf("ValidateConflict error: %s", err))
		return fmt.Errorf("error validating AccessRequest conflicts: %w", err)
	}
	return nil
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

// ValidateConflict will verify if there are existing AccessRequests for the same
// user/app/role already in progress.
func (r *AccessRequestReconciler) ValidateConflict(ctx context.Context, ar *api.AccessRequest) error {
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
		if arResp.Spec.Role.TemplateRef.Name != ar.Spec.Role.TemplateRef.Name ||
			arResp.Spec.Role.TemplateRef.Namespace != ar.Spec.Role.TemplateRef.Namespace {
			continue
		}
		// if the existing request is pending or granted, then the new request is
		// a duplicate and must be rejected
		if arResp.Status.RequestState == api.GrantedStatus ||
			arResp.Status.RequestState == api.RequestedStatus {
			return NewAccessRequestConflictError(fmt.Sprintf("found existing AccessRequest (%s/%s) in %s state", arResp.GetNamespace(), arResp.GetName(), string(arResp.Status.RequestState)))
		}
		// if the existing request reconciliation isn't initialized yet, then we
		// compare the creation timestamp and just allow the older one to proceed
		if arResp.Status.RequestState == "" &&
			arResp.GetCreationTimestamp().After(ar.GetCreationTimestamp().Time) {
			return NewAccessRequestConflictError(fmt.Sprintf("found older AccessRequest (%s/%s) in progress", arResp.GetNamespace(), arResp.GetName()))
		}
	}
	return nil
}

// buildResult will verify the given status and determine when this access
// request should be requeued.
func buildResult(status api.Status, ar *api.AccessRequest, config config.ControllerConfigurer) ctrl.Result {
	result := ctrl.Result{}
	switch status {
	case api.RequestedStatus:
		result.Requeue = true
		result.RequeueAfter = config.ControllerRequeueInterval()
	case api.GrantedStatus:
		result.Requeue = true
		result.RequeueAfter = time.Until(ar.Status.ExpiresAt.Time)
	default:
		if ar.IsConcluded() && hasTTLConfig(config) {
			if ttl := getTTLTime(ar, config); ttl != nil {
				result.Requeue = true
				result.RequeueAfter = time.Until(*ttl)
			}
		}
	}
	return result
}

// hasTTLConfig checks if a TTL (Time-To-Live) configuration is set for the controller.
// It determines this by verifying if the TTL value is not equal to zero.
//
// Parameters:
// - c: An implementation of the ControllerConfigurer interface.
//
// Returns:
// - A boolean indicating whether a TTL configuration is set.
func hasTTLConfig(c config.ControllerConfigurer) bool {
	return c.ControllerAccessRequestTTL() != time.Nanosecond*0
}

// hasTimeoutConfig checks if the controller configuration has a non-zero timeout value
// for handling requests.
//
// Parameters:
// - c: The controller configuration implementing the ControllerConfigurer interface.
//
// Returns:
// - A boolean indicating whether the timeout is configured (true) or not (false).
func hasTimeoutConfig(c config.ControllerConfigurer) bool {
	return c.ControllerRequestTimeout() != time.Nanosecond*0
}

// handleTTL checks whether the AccessRequest has exceeded its configured TTL (Time-To-Live) duration.
// If the TTL is not configured, it returns false and does not mark the resource for deletion.
// If the TTL is exceeded, it marks the resource for deletion by adding a deletion timestamp.
// TTL is only applied for concluded AccessRequests (i.e., those in Denied, Expired, etc. See isConcluded()).
//
// Parameters:
// - ctx: The context for the operation, used for cancellation and deadlines.
// - ar: The AccessRequest object to check and potentially mark for deletion.
//
// Returns:
// - A boolean indicating whether the TTL was exceeded.
// - An error if adding the deletion timestamp fails, or nil if successful.
func (r *AccessRequestReconciler) handleTTL(ctx context.Context, ar *api.AccessRequest) (bool, error) {
	// Skip if the TTL is not configured.
	if !hasTTLConfig(r.Config) {
		return false, nil
	}
	// Skip if the AccessRequest is not concluded.
	if !ar.IsConcluded() {
		return false, nil
	}

	// Check if the AccessRequest has exceeded its configured TTL (Time-To-Live) duration.
	ttl := getTTLTime(ar, r.Config)
	if ttl == nil {
		return false, nil
	}
	ttlExceeded := time.Now().After(*ttl)

	if ttlExceeded {
		// If TTL is exceeded, set the resource to be deleted.
		err := r.Delete(ctx, ar)
		if err != nil {
			return ttlExceeded, fmt.Errorf("error adding deletion timestamp: %w", err)
		}
	}
	return ttlExceeded, nil
}

// getTTLTime calculates the TTL (Time-To-Live) for an AccessRequest object.
// If the AccessRequest has concluded, it determines the TTL based on the last
// status history transition time and the configured TTL duration.
// Parameters:
// - ar: Pointer to the AccessRequest object.
// - config: ControllerConfigurer providing the TTL configuration.
// Returns:
// - A pointer to the calculated TTL time if the AccessRequest is concluded.
// - nil if the AccessRequest is not concluded.
func getTTLTime(ar *api.AccessRequest, config config.ControllerConfigurer) *time.Time {
	if !ar.IsConcluded() {
		return nil
	}
	lastStatusHistory := ar.Status.History[len(ar.Status.History)-1]
	ttl := lastStatusHistory.TransitionTime.Add(config.ControllerAccessRequestTTL())
	return &ttl
}

// getTimeoutTime calculates the timeout time for an AccessRequest object.
// The timeout is determined based on the last status history transition time
// and the configured request timeout duration. If the AccessRequest is not in
// InitiatedStatus or RequestedStatus, it does not have a timeout.
//
// Parameters:
// - ar: Pointer to the AccessRequest object.
// - config: ControllerConfigurer providing the timeout configuration.
//
// Returns:
// - A pointer to the calculated timeout time if applicable.
// - nil if the AccessRequest does not have a timeout.
func getTimeoutTime(ar *api.AccessRequest, config config.ControllerConfigurer) *time.Time {
	if ar.Status.RequestState != api.InitiatedStatus && ar.Status.RequestState != api.RequestedStatus {
		return nil
	}
	lastStatusHistory := ar.Status.History[len(ar.Status.History)-1]
	timeout := lastStatusHistory.TransitionTime.Add(config.ControllerRequestTimeout())
	return &timeout
}

// doWithRetry attempts to execute the provided function `fn` with retries on conflict
// errors. It ensures that the AccessRequest object is re-fetched on subsequent attempts
// after the first failure.
//
// Parameters:
// - ctx: The context for managing request deadlines and cancellations.
// - ar: The AccessRequest object to operate on.
// - fn: A function that performs the desired operation on the AccessRequest object.
//
// Returns:
// - An error if the operation fails after exhausting all retries.
func (r *AccessRequestReconciler) doWithRetry(ctx context.Context, ar *api.AccessRequest, fn func(ctx context.Context, ar *api.AccessRequest) error) error {
	firstAttempt := true
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if !firstAttempt {
			// Re-fetch the AccessRequest object to ensure the latest state is used.
			err := r.Get(ctx, client.ObjectKeyFromObject(ar), ar)
			if err != nil {
				return fmt.Errorf("error getting the AccessRequest: %w", err)
			}
		}
		firstAttempt = false
		// Execute the provided function with the AccessRequest object.
		return fn(ctx, ar)
	})
}

// handleRequestTimeout checks if the AccessRequest resource has exceeded its configured timeout duration.
// If the timeout is not configured, it returns false and does not update the status.
// Should only process timeouts for AccessRequests in InitiatedStatus or in RequestedStatus.
// If the timeout is exceeded, it updates the status to indicate a timeout.
//
// Parameters:
// - ctx: The context for managing request deadlines and cancellations.
// - ar: The AccessRequest object to evaluate.
//
// Returns:
// - A boolean indicating whether the AccessRequest has timed out.
// - An error if there is an issue updating the status.
func (r *AccessRequestReconciler) handleRequestTimeout(ctx context.Context, ar *api.AccessRequest) (bool, error) {
	// If the timeout is not configured, return false and do not update the status.
	if !hasTimeoutConfig(r.Config) {
		return false, nil
	}
	timeoutAt := getTimeoutTime(ar, r.Config)
	if timeoutAt == nil {
		return false, nil
	}
	// Check if the AccessRequest has exceeded its configured timeout duration.
	timedout := time.Now().After(*timeoutAt)
	if timedout {
		updateStatusFn := func(ctx context.Context, ar *api.AccessRequest) error {
			// Update the status history to indicate a timeout.
			ar.UpdateStatusHistory(api.TimeoutStatus, "AccessRequest timed out")
			return r.Status().Update(ctx, ar)
		}
		// Attempt to update the status with retries on conflict errors.
		err := r.doWithRetry(ctx, ar, updateStatusFn)
		if err != nil {
			return timedout, fmt.Errorf("error updating status to timeout: %w", err)
		}
	}
	return timedout, nil
}

// handleFinalizer will check if the AccessRequest is being deleted and
// proceed with the necessary clean up logic if so. If the object is not
// being deleted, it will register the AccessRequest finalizer in the live
// state. The function will return a boolean value to determine if the object
// was deleted.
func (r *AccessRequestReconciler) handleFinalizer(ctx context.Context, ar *api.AccessRequest) (bool, error) {
	// examine DeletionTimestamp to determine if object is under deletion
	if ar.DeletionTimestamp.IsZero() {
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
			rt, _ := r.Service.getRenderedRole(ctx, ar, ar.Status.TargetProject)
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
	selector := fields.SelectorFromSet(
		fields.Set{
			roleTemplateNameField:      roleTemplate.GetName(),
			roleTemplateNamespaceField: roleTemplate.GetNamespace(),
		})
	listOps := &client.ListOptions{
		FieldSelector: selector,
	}
	err := r.List(ctx, attachedAccessRequests, listOps)
	if err != nil {
		logger.Error(err, "findObjectsForRoleTemplate error: list k8s resources error")
		return []reconcile.Request{}
	}

	requests := []reconcile.Request{}
	for _, ar := range attachedAccessRequests.Items {
		if !ar.IsConcluded() {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      ar.GetName(),
					Namespace: ar.GetNamespace(),
				},
			})
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

	requests := []reconcile.Request{}
	for _, ar := range associatedAccessRequests.Items {
		if !ar.IsConcluded() {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      ar.GetName(),
					Namespace: ar.GetNamespace(),
				},
			})
		}
	}
	totalRequests := len(requests)
	if totalRequests == 0 {
		return nil
	}
	logger.Debug(fmt.Sprintf("Found %d associated AccessRequests with project %s. Reconciling...", totalRequests, project.GetName()))
	return requests
}

// callReconcileForApplication finds all AccessRequest resources associated with the given Application object
// and returns a list of reconcile.Requests for those that are not yet concluded. This is typically used to
// trigger reconciliation of AccessRequests when their associated Application is updated.
//
// Parameters:
//
//	ctx - The context for the request, used for logging and cancellation.
//	app - The Application object for which to find associated AccessRequests.
//
// Returns:
//
//	A slice of reconcile.Requests for each associated AccessRequest that is not concluded.
//	Returns nil if no such AccessRequests are found or if an error occurs during listing.
func (r *AccessRequestReconciler) callReconcileForApplication(ctx context.Context, app client.Object) []reconcile.Request {
	logger := log.FromContext(ctx)
	logger.Debug(fmt.Sprintf("Application %s/%s updated: searching for associated AccessRequests...", app.GetNamespace(), app.GetName()))
	associatedAccessRequests := &api.AccessRequestList{}
	selector := fields.SelectorFromSet(
		fields.Set{
			appField:          app.GetName(),
			appNamespaceField: app.GetNamespace(),
		})
	listOps := &client.ListOptions{
		FieldSelector: selector,
	}
	err := r.List(ctx, associatedAccessRequests, listOps)
	if err != nil {
		logger.Error(err, "findObjectsForProject error: list k8s resources error")
		return []reconcile.Request{}
	}

	requests := []reconcile.Request{}
	for _, ar := range associatedAccessRequests.Items {
		if !ar.IsConcluded() {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      ar.GetName(),
					Namespace: ar.GetNamespace(),
				},
			})
		}
	}
	totalRequests := len(requests)
	if totalRequests == 0 {
		return nil
	}
	logger.Debug(fmt.Sprintf("Found %d associated AccessRequests with Application %s/%s. Reconciling...", totalRequests, app.GetNamespace(), app.GetName()))
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
		IndexField(context.Background(), &api.AccessRequest{}, roleTemplateNameField, func(rawObj client.Object) []string {
			ar := rawObj.(*api.AccessRequest)
			if ar.Spec.Role.TemplateRef.Name == "" {
				return nil
			}
			return []string{ar.Spec.Role.TemplateRef.Name}
		})
	if err != nil {
		return fmt.Errorf("error creating Role.Template.Name field index: %w", err)
	}

	err = mgr.GetFieldIndexer().
		IndexField(context.Background(), &api.AccessRequest{}, roleTemplateNamespaceField, func(rawObj client.Object) []string {
			ar := rawObj.(*api.AccessRequest)
			if ar.Spec.Role.TemplateRef.Namespace == "" {
				return nil
			}
			return []string{ar.Spec.Role.TemplateRef.Namespace}
		})
	if err != nil {
		return fmt.Errorf("error creating Role.Template.Namespace field index: %w", err)
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

// ProjectChangedPredicate returns a predicate that triggers reconciliation
// only when an ArgoCD AppProject has changed in a way that should trigger
// a reconcile, as determined by ProjectChangeShouldTriggerReconcile.
// It ignores create, delete, and generic events.
func ProjectChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			newProj := e.ObjectNew.(*argocd.AppProject)
			oldProj := e.ObjectOld.(*argocd.AppProject)

			return ProjectChangeShouldTriggerReconcile(newProj, oldProj)
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

// ProjectChangeShouldTriggerReconcile determines whether a change between two AppProject
// objects should trigger a reconcile. It compares the roles, policies, JWT tokens, and groups
// between the new and old project specifications. If there are any differences in the number
// of roles, policies, JWT tokens, groups, or in the role names or descriptions, it returns true.
// If either project is nil, it also returns true.
func ProjectChangeShouldTriggerReconcile(newProj, oldProj *argocd.AppProject) bool {
	if newProj == nil || oldProj == nil {
		return true
	}
	if len(newProj.Spec.Roles) != len(oldProj.Spec.Roles) {
		return true
	}

	for _, oldRole := range oldProj.Spec.Roles {
		if !hasRole(newProj.Spec.Roles, oldRole) {
			return true
		}
	}
	return false
}

// hasRole checks if the given role exists in the list of roles.
// It compares the role's Name and Description, and ensures that the Policies,
// JWTTokens, and Groups slices are of equal length and contain the same elements.
// Returns true if an equivalent role is found, otherwise returns false.
func hasRole(roles []argocd.ProjectRole, role argocd.ProjectRole) bool {
	for _, r := range roles {
		if r.Name == role.Name && r.Description == role.Description {
			if !MatchRolePoliciesAndTokens(r, role.Policies, role.JWTTokens) {
				return false
			}
			if len(r.Groups) != len(role.Groups) {
				return false
			}
			for _, group := range r.Groups {
				if !slices.Contains(role.Groups, group) {
					return false
				}
			}

			return true
		}
	}
	return false
}

// MatchRolePoliciesAndTokens compares two ProjectRole objects and returns true
// if both have identical Policies and JWTTokens slices (regardless of order).
// Returns false if the lengths differ or any element is missing in either slice.
func MatchRolePoliciesAndTokens(role argocd.ProjectRole, policies []string, tokens []argocd.JWTToken) bool {
	if len(role.Policies) != len(policies) {
		return false
	}
	if len(role.JWTTokens) != len(tokens) {
		return false
	}

	for _, policy := range role.Policies {
		if !slices.Contains(policies, policy) {
			return false
		}
	}
	for _, jwtToken := range role.JWTTokens {
		if !slices.Contains(tokens, jwtToken) {
			return false
		}
	}
	return true
}

// ApplicationChangedPredicate returns a predicate that triggers reconciliation
// when an ArgoCD Application's project changes, or when the Application is deleted.
// It ignores create and generic events.
func ApplicationChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			newApp := e.ObjectNew.(*argocd.Application)
			oldApp := e.ObjectOld.(*argocd.Application)

			if newApp == nil || oldApp == nil {
				return true
			}

			if newApp.Spec.Project != oldApp.Spec.Project {
				return true
			}
			return false
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
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
		For(&api.AccessRequest{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&api.RoleTemplate{},
			handler.EnqueueRequestsFromMapFunc(r.callReconcileForRoleTemplate),
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&argocd.AppProject{},
			handler.EnqueueRequestsFromMapFunc(r.callReconcileForProject),
			builder.WithPredicates(ProjectChangedPredicate())).
		Watches(&argocd.Application{},
			handler.EnqueueRequestsFromMapFunc(r.callReconcileForApplication),
			builder.WithPredicates(ApplicationChangedPredicate())).
		Complete(r)
}
