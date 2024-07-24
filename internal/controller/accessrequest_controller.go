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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/argoproj-labs/ephemeral-access/api/v1alpha1"
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
)

// +kubebuilder:rbac:groups=ephemeral-access.argoproj-labs.io,resources=accessrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ephemeral-access.argoproj-labs.io,resources=accessrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ephemeral-access.argoproj-labs.io,resources=accessrequests/finalizers,verbs=update

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
	logger := log.FromContext(ctx).
		WithValues("controller", "AccessRequest", "reconcile", req.NamespacedName)

	logger.Info("retrieving AccessRequest k8s state")
	var ar api.AccessRequest
	if err := r.Get(ctx, req.NamespacedName, &ar); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "error retrieving AccessRequest from k8s")
		return ctrl.Result{}, err
	}

	// check if the object is being deleted and properly handle it
	deleted, err := r.handleFinalizer(ctx, &ar)
	if err != nil {
		logger.Error(err, "handleFinalizer error")
		return ctrl.Result{}, fmt.Errorf("error handling finalizer: %w", err)
	}
	// stop the reconciliation as the object was deleted
	if deleted {
		logger.Info("Object deleted")
		return ctrl.Result{}, nil
	}

	err = ar.Validate()
	if err != nil {
		logger.Info("validation error: %s", err)
		return ctrl.Result{}, fmt.Errorf("error validating the AccessRequest: %w", err)
	}

	// initialize the status if not done yet
	if ar.Status.RequestState == "" {
		ar.UpdateStatus(api.RequestedStatus, "")
		r.Status().Update(ctx, &ar)
	}

	// check if the access is expired
	expired, err := r.handleAccessExpired(ctx, &ar)
	if err != nil {
		logger.Error(err, "handleAccessExpired error")
		return ctrl.Result{}, fmt.Errorf("error handling access expired: %w", err)
	}
	// Stop the reconciliation if the access is expired
	if expired {
		logger.Info("AccessRequest expired")
		return ctrl.Result{}, nil
	}

	// check subject is sudoer
	status, err := r.handlePermission(ctx, &ar)
	if err != nil {
		logger.Error(err, "handlePermission error")
		return ctrl.Result{}, fmt.Errorf("error handling permission: %w", err)
	}

	return buildResult(status, &ar), nil
}

// buildResult will verify the given status and determine when this access
// request should be requeued.
func buildResult(status api.Status, ar *api.AccessRequest) ctrl.Result {
	result := ctrl.Result{}
	switch status {
	case api.RequestedStatus:
		result.Requeue = true
		// TODO add a controller configuration
		result.RequeueAfter = time.Minute * 3
	case api.GrantedStatus:
		result.Requeue = true
		result.RequeueAfter = ar.Spec.Duration.Duration
	}
	return result
}

func (r *AccessRequestReconciler) handlePermission(ctx context.Context, ar *api.AccessRequest) (api.Status, error) {

	resp, err := r.Allowed(ctx, ar)
	if err != nil {
		return "", fmt.Errorf("error verifying if subject is allowed: %w", err)
	}
	if !resp.Allowed {
		err = r.updateStatusWithRetry(ctx, ar, api.DeniedStatus, resp.Message)
		if err != nil {
			return "", fmt.Errorf("error updating access request status to denied: %w", err)
		}
		return api.DeniedStatus, nil
	}

	err = r.updateAppProjectRBAC(ctx, ar)
	if err != nil {
		return "", fmt.Errorf("error updating AppProject RBAC: %w", err)
	}
	err = r.updateStatusWithRetry(ctx, ar, api.GrantedStatus, resp.Message)
	if err != nil {
		return "", fmt.Errorf("error updating access request status to granted: %w", err)
	}
	return api.GrantedStatus, nil
}

func (r *AccessRequestReconciler) updateStatusWithRetry(ctx context.Context, ar *api.AccessRequest, status api.Status, details string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := r.Get(ctx, client.ObjectKeyFromObject(ar), ar)
		if err != nil {
			return err
		}
		ar.UpdateStatus(status, details)
		return r.Status().Update(ctx, ar)
	})
}

// TODO
func (r *AccessRequestReconciler) updateAppProjectRBAC(ctx context.Context, ar *api.AccessRequest) error {
	// 1. retrieve the Application
	// 2. verify if CR is approved
	// 3. retrieve the AppProject
	// 4. assign user in the desired role in the AppProject
	return nil
}

type AllowedResponse struct {
	Allowed bool
	Message string
}

// TODO
func (r *AccessRequestReconciler) Allowed(ctx context.Context, ar *api.AccessRequest) (AllowedResponse, error) {
	return AllowedResponse{}, nil
}

// handleAccessExpired will verify if the given AccessRequest is expired. If
// so, it will remove the Argo CD access for the subject and update the
// AccessRequest status field. It will return a boolean to determine if the
// given AccessRequest is expired. Note that if there is an error, the returned
// boolean must be ignored.
func (r *AccessRequestReconciler) handleAccessExpired(ctx context.Context, ar *api.AccessRequest) (bool, error) {
	expired := false
	if ar.Status.ExpiresAt != nil &&
		ar.Status.ExpiresAt.Time.After(time.Now()) {

		err := r.removeArgoCDAccess(ar)
		if err != nil {
			return expired, fmt.Errorf("error removing access for expired request: %w", err)
		}
		err = r.updateStatusWithRetry(ctx, ar, api.ExpiredStatus, "")
		if err != nil {
			return expired, fmt.Errorf("error updating access request status to expired: %w", err)
		}
		expired = true
	}
	return expired, nil
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
		// execute the cleanup procedure before removing the finalizer
		if err := r.removeArgoCDAccess(ar); err != nil {
			// if fail to delete the external dependency here, return with error
			// so that it can be retried.
			return false, fmt.Errorf("error cleaning up Argo CD access: %w", err)
		}

		// remove our finalizer from the list and update it.
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			err := r.Get(ctx, client.ObjectKeyFromObject(ar), ar)
			if err != nil {
				return err
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

// TODO
func (r *AccessRequestReconciler) removeArgoCDAccess(ar *api.AccessRequest) error {
	return fmt.Errorf("cleanupArgoCDAccess not implemented")
}

// SetupWithManager sets up the controller with the Manager.
func (r *AccessRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.AccessRequest{}).
		Complete(r)
}
