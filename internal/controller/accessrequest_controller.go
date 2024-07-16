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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ephemeralaccessv1alpha1 "github.com/argoproj-labs/ephemeral-access/api/v1alpha1"
)

// AccessRequestReconciler reconciles a AccessRequest object
type AccessRequestReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ephemeral-access.argoproj-labs.io,resources=accessrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ephemeral-access.argoproj-labs.io,resources=accessrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ephemeral-access.argoproj-labs.io,resources=accessrequests/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.2/pkg/reconcile
func (r *AccessRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	var accessRequest ephemeralaccessv1alpha1.AccessRequest
	if err := r.Get(ctx, req.NamespacedName, &accessRequest); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// accessRequest.Spec.Subjects[0].Username

	// 1. verify if accessrequest is expired and status is "granted"
	// 1.1 if so, remove the user from the elevated role
	// 1.2 update the accessrequest status to "expired"
	// 1.3 return
	// 2. verify if user has the necessary access to be promoted
	// 2.1 if they don't, update the accessrequest status to "denied"
	// 2.2 return
	// 3. verify if CR is approved
	// 4. retrieve the Application
	// 5. retrieve the AppProject
	// 6. assign user in the desired role in the AppProject
	// 7. update the accessrequest status to "granted"
	// 8. set the RequeueAfter in Result

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AccessRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ephemeralaccessv1alpha1.AccessRequest{}).
		Complete(r)
}
