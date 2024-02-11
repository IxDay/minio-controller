/*
Copyright 2025.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	bucketv1alpha1 "github.com/IxDay/api/v1alpha1"
	"github.com/IxDay/internal/minio"
)

const (
	// typeAvailableBucket represents the status of the Bucket reconciliation
	typeAvailableBucket = "Available"
)

// MinioReconciler reconciles a Minio object
type MinioReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	MinioClient *minio.Client
}

// +kubebuilder:rbac:groups=bucket.ixday.github.io,resources=minios,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=bucket.ixday.github.io,resources=minios/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=bucket.ixday.github.io,resources=minios/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Minio object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *MinioReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	bucket := &bucketv1alpha1.Minio{}
	if err := r.Get(ctx, req.NamespacedName, bucket); err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("minio resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get minio")
		return ctrl.Result{}, err
	}

	// Let's just set the status as Unknown when no status is available
	if bucket.Status.Conditions == nil || len(bucket.Status.Conditions) == 0 {
		meta.SetStatusCondition(&bucket.Status.Conditions, metav1.Condition{Type: typeAvailableBucket, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err := r.Status().Update(ctx, bucket); err != nil {
			log.Error(err, "Failed to update bucket status")
			return ctrl.Result{}, err
		}

		// Let's re-fetch the bucket Custom Resource after updating the status
		// so that we have the latest state of the resource on the cluster and we will avoid
		// raising the error "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		if err := r.Get(ctx, req.NamespacedName, bucket); err != nil {
			log.Error(err, "Failed to re-fetch bucket")
			return ctrl.Result{}, err
		}
	}

	found, err := r.MinioClient.BucketExists(ctx, req.Name)
	if err != nil {
		log.Error(err, "Failed to get bucket")
	} else if !found {
		log.Info("Creating a new Bucket", "Bucket.Name", req.Name)

		if err := r.MinioClient.NewBucket(ctx, req.Name); err != nil {
			log.Error(err, "Failed to create new Bucket",
				"Bucket.Name", req.Name)
			return ctrl.Result{}, err
		}
		// Bucket created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}
	// The following implementation will update the status
	meta.SetStatusCondition(&bucket.Status.Conditions, metav1.Condition{Type: typeAvailableBucket,
		Status: metav1.ConditionTrue, Reason: "Reconciling",
		Message: fmt.Sprintf("Bucket %s created successfully", bucket.Name)})

	if err := r.Status().Update(ctx, bucket); err != nil {
		log.Error(err, "Failed to update Bucket status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MinioReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&bucketv1alpha1.Minio{}).
		Named("minio").
		Complete(r)
}
