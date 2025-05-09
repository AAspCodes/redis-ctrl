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

	"github.com/go-redis/redismock/v9"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redis/go-redis/v9"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	redisv1alpha1 "github.com/AAspCodes/redis-ctrl/api/v1alpha1"
)

var _ = Describe("RedisEntry Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		redisentry := &redisv1alpha1.RedisEntry{}
		var mockRedis *redis.Client
		var mock redismock.ClientMock

		BeforeEach(func() {
			By("creating the custom resource for the Kind RedisEntry")
			err := k8sClient.Get(ctx, typeNamespacedName, redisentry)
			if err != nil && errors.IsNotFound(err) {
				resource := &redisv1alpha1.RedisEntry{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: redisv1alpha1.RedisEntrySpec{
						Key:   "test-key",
						Value: "test-value",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			// Create a new mock Redis client for each test
			mockRedis, mock = redismock.NewClientMock()
			mock.ExpectSet("test-key", "test-value", 0).SetVal("OK")
		})

		AfterEach(func() {
			resource := &redisv1alpha1.RedisEntry{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance RedisEntry")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			// Ensure all Redis expectations were met
			Expect(mock.ExpectationsWereMet()).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &RedisEntryReconciler{
				Client:      k8sClient,
				Scheme:      k8sClient.Scheme(),
				RedisClient: mockRedis,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify the status was updated
			err = k8sClient.Get(ctx, typeNamespacedName, redisentry)
			Expect(err).NotTo(HaveOccurred())
			Expect(redisentry.Status.Conditions).To(HaveLen(1))
			Expect(redisentry.Status.Conditions[0].Type).To(Equal("Available"))
			Expect(redisentry.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
		})
	})
})
