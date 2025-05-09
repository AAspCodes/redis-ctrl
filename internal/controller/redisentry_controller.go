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

	"github.com/redis/go-redis/v9"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	redisv1alpha1 "github.com/AAspCodes/redis-ctrl/api/v1alpha1"
)

const (
	// Redis connection details - these will be configurable via environment variables
	redisHost     = "redis-redis-service"
	redisPort     = "6379"
	redisPassword = "" // No password for now

	// Condition types
	typeAvailable = "Available"
	typeError     = "Error"

	// Condition reasons
	reasonSuccess        = "Success"
	reasonRedisError     = "RedisError"
	reasonReconcileError = "ReconcileError"
)

// RedisEntryReconciler reconciles a RedisEntry object
type RedisEntryReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	RedisClient *redis.Client
}

// +kubebuilder:rbac:groups=redis.aaspcodes.github.io,resources=redisentries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redis.aaspcodes.github.io,resources=redisentries/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=redis.aaspcodes.github.io,resources=redisentries/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RedisEntry object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *RedisEntryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Starting reconciliation", "namespacedName", req.NamespacedName)

	// Fetch the RedisEntry instance
	redisEntry := &redisv1alpha1.RedisEntry{}
	err := r.Get(ctx, req.NamespacedName, redisEntry)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			logger.Info("RedisEntry resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		logger.Error(err, "Failed to get RedisEntry")
		return ctrl.Result{}, err
	}

	// Initialize Redis client if not already initialized
	if r.RedisClient == nil {
		r.RedisClient = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", redisHost, redisPort),
			Password: redisPassword,
			DB:       0,
		})
	}

	// Set the key-value pair in Redis
	var ttl time.Duration
	if redisEntry.Spec.TTL != nil {
		ttl = time.Duration(*redisEntry.Spec.TTL) * time.Second
	}

	err = r.RedisClient.Set(ctx, redisEntry.Spec.Key, redisEntry.Spec.Value, ttl).Err()
	if err != nil {
		logger.Error(err, "Failed to set value in Redis")
		r.setCondition(ctx, redisEntry, typeError, reasonRedisError, err.Error())
		return ctrl.Result{RequeueAfter: time.Second * 5}, err
	}

	// Update status
	now := metav1.Now()
	redisEntry.Status.LastUpdated = &now
	redisEntry.Status.CurrentValue = redisEntry.Spec.Value

	// Set success condition
	r.setCondition(ctx, redisEntry, typeAvailable, reasonSuccess, "Successfully updated Redis")

	if err := r.Status().Update(ctx, redisEntry); err != nil {
		logger.Error(err, "Failed to update RedisEntry status")
		return ctrl.Result{RequeueAfter: time.Second}, err
	}

	logger.Info("Successfully reconciled RedisEntry", "key", redisEntry.Spec.Key)
	return ctrl.Result{}, nil
}

// setCondition updates the RedisEntry status conditions
func (r *RedisEntryReconciler) setCondition(ctx context.Context, redisEntry *redisv1alpha1.RedisEntry, conditionType string, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	// Find and update existing condition or append new one
	existingConditions := redisEntry.Status.Conditions
	for i, cond := range existingConditions {
		if cond.Type == conditionType {
			if cond.Status != condition.Status || cond.Reason != condition.Reason || cond.Message != condition.Message {
				existingConditions[i] = condition
			}
			return
		}
	}
	redisEntry.Status.Conditions = append(existingConditions, condition)
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisEntryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&redisv1alpha1.RedisEntry{}).
		Named("redisentry").
		Complete(r)
}
