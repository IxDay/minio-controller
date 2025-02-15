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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	bucketv1alpha1 "github.com/IxDay/api/v1alpha1"
	"github.com/IxDay/internal/minio"
)

const (
	// typeAvailableBucket represents the status of the Bucket reconciliation
	typeAvailableBucket = "Available"
	// name of our custom finalizer
	finalizerName = "bucket.ixday.github.io/finalizer"
)

// BucketReconciler reconciles a Bucket object
type BucketReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	MinioClient minio.Client
}

// +kubebuilder:rbac:groups=minio.ixday.github.io,resources=buckets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=minio.ixday.github.io,resources=buckets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=minio.ixday.github.io,resources=buckets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Bucket object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *BucketReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	bucket := &bucketv1alpha1.Bucket{}
	if err := r.Get(ctx, req.NamespacedName, bucket); err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get bucket")
		return ctrl.Result{}, err
	}

	// https://book.kubebuilder.io/reference/using-finalizers
	// examine DeletionTimestamp to determine if object is under deletion
	if bucket.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// to registering our finalizer.
		if !controllerutil.ContainsFinalizer(bucket, finalizerName) {
			controllerutil.AddFinalizer(bucket, finalizerName)
			if err := r.Update(ctx, bucket); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(bucket, finalizerName) {
			// our finalizer is present, so lets handle any external dependency
			log.Info("Deleting associated users and policies", "Bucket.Name", bucket.BucketName())
			if err := r.MinioClient.UserDelete(ctx, bucket.BucketName()); err != nil {
				log.Error(err, "Failed deleting associated users and policies", "Bucket.Name", bucket.BucketName())
				return ctrl.Result{}, err
			}

			log.Info("Deleting Bucket", "Bucket.Name", bucket.BucketName())
			if err := r.MinioClient.BucketDelete(ctx, bucket.BucketName()); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried.
				log.Error(err, "Failed deleting bucket", "Bucket.Name", bucket.BucketName())
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(bucket, finalizerName)
			if err := r.Update(ctx, bucket); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// Let's just set the status as Unknown when no status is available
	if bucket.Status.Conditions == nil || len(bucket.Status.Conditions) == 0 {
		condition := metav1.Condition{
			Type: typeAvailableBucket, Status: metav1.ConditionUnknown,
			Reason: "Reconciling", Message: "Starting reconciliation",
		}
		meta.SetStatusCondition(&bucket.Status.Conditions, condition)
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

	found, err := r.MinioClient.BucketExists(ctx, bucket.BucketName())
	if err != nil {
		log.Error(err, "Failed to get bucket")
	} else if !found {
		log.Info("Creating a new Bucket", "Bucket.Name", bucket.BucketName())

		if err := r.MinioClient.BucketCreate(ctx, bucket.BucketName()); err != nil {
			log.Error(err, "Failed to create new Bucket",
				"Bucket.Name", bucket.BucketName())
			return ctrl.Result{}, err
		}
		// Bucket created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	}

	secret := &corev1.Secret{ObjectMeta: bucket.ObjectMeta}
	if bucket.Spec.SecretName != "" {
		secret.Name = bucket.Spec.SecretName
	}
	err = r.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, secret)
	if apierrors.IsNotFound(err) {
		sec, err := r.secretForBucket(bucket)
		if err != nil {
			log.Error(err, "Failed to define new Secret resource for Bucket")

			// The following implementation will update the status
			meta.SetStatusCondition(&bucket.Status.Conditions, metav1.Condition{Type: typeAvailableBucket,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create Secret for the custom resource (%s): (%s)", bucket.Name, err)})

			if err := r.Status().Update(ctx, bucket); err != nil {
				log.Error(err, "Failed to update Bucket status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}
		user, password := string(sec.Data["user"]), string(sec.Data["password"])
		if err := r.MinioClient.UserCreate(ctx, user, password, bucket.BucketName()); err != nil {
			log.Error(err, "Failed to create Bucket user, policy and attach")
			return ctrl.Result{}, err
		}

		log.Info("Creating a new Secret",
			"Secret.Namespace", sec.Namespace, "Secret.Name", sec.Name)
		if err = r.Create(ctx, sec); err != nil {
			log.Error(err, "Failed to create new Secret",
				"Secret.Namespace", sec.Namespace, "Secret.Name", sec.Name)
			return ctrl.Result{}, err
		}
		// Secret created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Secret")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
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
func (r *BucketReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&bucketv1alpha1.Bucket{}).
		Named("bucket").
		Complete(r)
}

func (r *BucketReconciler) secretForBucket(bucket *bucketv1alpha1.Bucket) (*corev1.Secret, error) {
	user, err := minio.GenerateAccessKey(0, nil)
	if err != nil {
		return nil, err
	}
	password, err := minio.GenerateSecretKey(0, nil)
	if err != nil {
		return nil, err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bucket.Name,
			Namespace: bucket.Namespace,
		},
		Data: map[string][]byte{
			"user":     user,
			"password": password,
		},
	}
	if bucket.Spec.SecretName != "" {
		secret.Name = bucket.Spec.SecretName
	}

	// Set the ownerRef for the Deployment
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(bucket, secret, r.Scheme); err != nil {
		return nil, err
	}
	return secret, nil
}
