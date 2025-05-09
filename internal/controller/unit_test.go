package controller

import (
	"context"
	"errors"
	"time"

	redisv1alpha1 "github.com/AAspCodes/redis-ctrl/api/v1alpha1"
	redismock "github.com/go-redis/redismock/v9"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	redisv9 "github.com/redis/go-redis/v9"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = ginkgo.Describe("RedisEntry Controller Unit Tests", func() {
	var (
		ctx                  context.Context
		mockRedis            *redisv9.Client
		mock                 redismock.ClientMock
		controllerReconciler *RedisEntryReconciler
		redisEntry           *redisv1alpha1.RedisEntry
		fakeClient           *fake.ClientBuilder
		s                    *runtime.Scheme
	)

	ginkgo.BeforeEach(func() {
		ctx = context.Background()
		s = runtime.NewScheme()
		gomega.Expect(redisv1alpha1.AddToScheme(s)).To(gomega.Succeed())
		gomega.Expect(scheme.AddToScheme(s)).To(gomega.Succeed())

		// Create a new fake client with the scheme and CRD
		fakeClient = fake.NewClientBuilder().
			WithScheme(s).
			WithStatusSubresource(&redisv1alpha1.RedisEntry{})

		// Create a new mock Redis client for each test
		mockRedis, mock = redismock.NewClientMock()

		// Create the controller with the fake client
		controllerReconciler = &RedisEntryReconciler{
			Client:      fakeClient.Build(),
			Scheme:      s,
			RedisClient: mockRedis,
		}
	})

	ginkgo.AfterEach(func() {
		// Ensure all Redis expectations were met
		gomega.Expect(mock.ExpectationsWereMet()).To(gomega.Succeed())
	})

	ginkgo.Context("Basic CRUD operations", func() {
		ginkgo.It("should handle basic key-value operations", func() {
			// Create a RedisEntry
			redisEntry = &redisv1alpha1.RedisEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-basic",
					Namespace: "default",
				},
				Spec: redisv1alpha1.RedisEntrySpec{
					Key:   "test-key",
					Value: "test-value",
				},
			}

			// Create the RedisEntry
			gomega.Expect(controllerReconciler.Client.Create(ctx, redisEntry)).To(gomega.Succeed())

			// Set up Redis mock expectation
			mock.ExpectSet("test-key", "test-value", 0).SetVal("OK")

			// Reconcile
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-basic",
					Namespace: "default",
				},
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Verify the RedisEntry was updated
			updatedEntry := &redisv1alpha1.RedisEntry{}
			err = controllerReconciler.Get(ctx, types.NamespacedName{
				Name:      "test-basic",
				Namespace: "default",
			}, updatedEntry)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(updatedEntry.Status.Conditions).To(gomega.HaveLen(1))
			gomega.Expect(updatedEntry.Status.Conditions[0].Type).To(gomega.Equal("Available"))
			gomega.Expect(updatedEntry.Status.Conditions[0].Status).To(gomega.Equal(metav1.ConditionTrue))
		})

		ginkgo.It("should handle TTL operations", func() {
			ttl := int64(3600)
			redisEntry = &redisv1alpha1.RedisEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ttl",
					Namespace: "default",
				},
				Spec: redisv1alpha1.RedisEntrySpec{
					Key:   "ttl-key",
					Value: "ttl-value",
					TTL:   &ttl,
				},
			}

			// Create the RedisEntry
			gomega.Expect(controllerReconciler.Client.Create(ctx, redisEntry)).To(gomega.Succeed())

			// Set up Redis mock expectation with TTL
			mock.ExpectSet("ttl-key", "ttl-value", time.Duration(ttl)*time.Second).SetVal("OK")

			// Reconcile
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-ttl",
					Namespace: "default",
				},
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Verify the RedisEntry was updated
			updatedEntry := &redisv1alpha1.RedisEntry{}
			err = controllerReconciler.Get(ctx, types.NamespacedName{
				Name:      "test-ttl",
				Namespace: "default",
			}, updatedEntry)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(updatedEntry.Status.Conditions).To(gomega.HaveLen(1))
			gomega.Expect(updatedEntry.Status.Conditions[0].Type).To(gomega.Equal("Available"))
			gomega.Expect(updatedEntry.Status.Conditions[0].Status).To(gomega.Equal(metav1.ConditionTrue))
		})

		ginkgo.It("should handle Redis errors", func() {
			redisEntry = &redisv1alpha1.RedisEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-error",
					Namespace: "default",
				},
				Spec: redisv1alpha1.RedisEntrySpec{
					Key:   "error-key",
					Value: "error-value",
				},
			}

			// Create the RedisEntry
			gomega.Expect(controllerReconciler.Client.Create(ctx, redisEntry)).To(gomega.Succeed())

			// Set up Redis mock to return error
			mock.ExpectSet("error-key", "error-value", 0).SetErr(errors.New("redis error"))

			// Reconcile
			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-error",
					Namespace: "default",
				},
			})
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(result.Requeue).To(gomega.BeTrue())
			gomega.Expect(result.RequeueAfter).To(gomega.Equal(5 * time.Second))

			// Verify error status was set
			updatedEntry := &redisv1alpha1.RedisEntry{}
			err = controllerReconciler.Get(ctx, types.NamespacedName{
				Name:      "test-error",
				Namespace: "default",
			}, updatedEntry)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(updatedEntry.Status.Conditions).To(gomega.HaveLen(1))
			gomega.Expect(updatedEntry.Status.Conditions[0].Type).To(gomega.Equal("Error"))
			gomega.Expect(updatedEntry.Status.Conditions[0].Status).To(gomega.Equal(metav1.ConditionTrue))
		})
	})
})
