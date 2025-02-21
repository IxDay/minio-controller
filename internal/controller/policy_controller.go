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
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	miniov1alpha1 "github.com/IxDay/api/v1alpha1"
	"github.com/IxDay/internal/minio"
)

const (
	// typeAvailableBucket represents the status of the Bucket reconciliation
	typeAvailablePolicy = "Available"
	typeBucketExists    = "BucketExists"
	// name of our custom finalizer
	finalizerNamePolicy = "policy.ixday.github.io/finalizer"
)

// PolicyReconciler reconciles a Policy object
type PolicyReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	MinioClient minio.Client
}

// +kubebuilder:rbac:groups=minio.ixday.github.io,resources=policies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=minio.ixday.github.io,resources=policies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=minio.ixday.github.io,resources=policies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Policy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *PolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	policy := &miniov1alpha1.Policy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get policy")
		return ctrl.Result{}, err
	}

	// https://book.kubebuilder.io/reference/using-finalizers
	// examine DeletionTimestamp to determine if object is under deletion
	if policy.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// to registering our finalizer.
		if !controllerutil.ContainsFinalizer(policy, finalizerNamePolicy) {
			controllerutil.AddFinalizer(policy, finalizerNamePolicy)
			if err := r.Update(ctx, policy); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(policy, finalizerNamePolicy) {
			// our finalizer is present, so lets handle any external dependency
			log.Info("Deleting associated users", "Policy.Name", policy.PolicyName())
			if err := r.MinioClient.PolicyDelete(ctx, policy.PolicyName()); err != nil {
				log.Error(err, "Failed deleting associated users and policies", "Policy.Name", policy.PolicyName())
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(policy, finalizerNamePolicy)
			if err := r.Update(ctx, policy); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// Let's just set the status as Unknown when no status is available
	if len(policy.Status.Conditions) == 0 {
		condition := metav1.Condition{
			Type: typeAvailablePolicy, Status: metav1.ConditionUnknown,
			Reason: "Reconciling", Message: "Starting reconciliation",
		}
		meta.SetStatusCondition(&policy.Status.Conditions, condition)
		if err := r.Status().Update(ctx, policy); err != nil {
			log.Error(err, "Failed to update policy status")
			return ctrl.Result{}, err
		}

		// Let's re-fetch the bucket Custom Resource after updating the status
		// so that we have the latest state of the resource on the cluster and we will avoid
		// raising the error "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
			log.Error(err, "Failed to re-fetch policy")
			return ctrl.Result{}, err
		}
	}

	// Retrieve associated bucket
	bucket := &miniov1alpha1.Bucket{}
	bucketKey := types.NamespacedName{Namespace: req.Namespace, Name: policy.Spec.BucketName}
	if err := r.Get(ctx, bucketKey, bucket); err != nil {
		if apierrors.IsNotFound(err) {
			condition := metav1.Condition{
				Type:    typeBucketExists,
				Status:  metav1.ConditionFalse,
				Reason:  "BucketDoesNotExist",
				Message: "BucketRef must reference an existing bucket to be attached",
			}
			meta.SetStatusCondition(&policy.Status.Conditions, condition)
			if err := r.Status().Update(ctx, policy); err != nil {
				log.Error(err, "Failed to update policy status")
			}
		}
		log.Error(err, "Failed to get associated bucket")
		return ctrl.Result{}, err
	}

	secret, err := r.getSecret(ctx, policy)
	if apierrors.IsNotFound(err) {
		secret, err = r.secretForPolicy(policy)
		if err != nil {
			log.Error(err, "Failed to define new secret resource for policy",
				"Policy.Name", policy.PolicyName())

			// The following implementation will update the status
			meta.SetStatusCondition(&bucket.Status.Conditions, metav1.Condition{Type: typeAvailableBucket,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create secret for the custom resource: (%s)", err)})

			if err := r.Status().Update(ctx, bucket); err != nil {
				log.Error(err, "Failed to update policy status",
					"Policy.Name", policy.PolicyName(),
				)
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}
		log.Info("Creating a new Secret", "Secret.Name", secret.Name)
		if err = r.Create(ctx, secret); err != nil {
			log.Error(err, "Failed to create new Secret", "Secret.Name", secret.Name)
			return ctrl.Result{}, err
		}
		// Secret created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Secret")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	log.V(2).Info("Reconciling policy")
	policyMinio := &minio.Policy{
		Bucket: bucket.BucketName(), Name: policy.PolicyName(),
	}
	if err := policyMinio.SetUser(secret.Data["user"], secret.Data["password"]); err != nil {
		log.Error(err, "invalid credentials", "Secret.Name", secret.Name)
		return ctrl.Result{}, err
	}
	if err = policyMinio.SetPolicy(policy.Spec.Statements); err != nil {
		log.Error(err, "invalid policy", "Policy.Name", policy.PolicyName())
		return ctrl.Result{}, err
	}
	if err := r.MinioClient.PolicyReconcile(ctx, policyMinio); err != nil {
		log.Error(err, "failed to create user, policy and attach")
		return ctrl.Result{}, err
	}

	// The following implementation will update the status
	meta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{Type: typeAvailablePolicy,
		Status: metav1.ConditionTrue, Reason: "Reconciling",
		Message: fmt.Sprintf("Policy %s created successfully", policy.PolicyName())})

	if err := r.Status().Update(ctx, policy); err != nil {
		log.Error(err, "Failed to update Policy status")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&miniov1alpha1.Policy{}).
		Named("policy").
		Complete(r)
}

func (r *PolicyReconciler) getSecret(
	ctx context.Context, policy *miniov1alpha1.Policy,
) (*corev1.Secret, error) {

	secret := &corev1.Secret{ObjectMeta: policy.ObjectMeta}
	if policy.Spec.SecretName != "" {
		secret.Name = policy.Spec.SecretName
	}
	namespacedName := types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}
	err := r.Get(ctx, namespacedName, secret)
	return secret, err
}

func (r *PolicyReconciler) secretForPolicy(policy *miniov1alpha1.Policy) (*corev1.Secret, error) {
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
			Name:      policy.Name,
			Namespace: policy.Namespace,
		},
		Data: map[string][]byte{
			"user":     user,
			"password": password,
		},
	}
	if policy.Spec.SecretName != "" {
		secret.Name = policy.Spec.SecretName
	}

	// Set the ownerRef for the Deployment
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(policy, secret, r.Scheme); err != nil {
		return nil, err
	}
	return secret, nil
}
