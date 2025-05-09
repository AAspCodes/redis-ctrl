// Package controller implements the Redis controller and its test suite.
// It contains both unit and integration tests for the Redis controller functionality.
package controller

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	redisv1alpha1 "github.com/AAspCodes/redis-ctrl/api/v1alpha1"
	redismock "github.com/go-redis/redismock/v9"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	redisv9 "github.com/redis/go-redis/v9"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	testEnv              *envtest.Environment
	cfg                  *rest.Config
	k8sClient            client.Client
	ctx                  context.Context
	mockRedis            *redisv9.Client
	mock                 redismock.ClientMock
	controllerReconciler *RedisEntryReconciler
	redisEntry           *redisv1alpha1.RedisEntry
)

// getFirstFoundEnvTestBinaryDir locates the first binary in the specified path.
func getFirstFoundEnvTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		logf.Log.Error(err, "Failed to read directory", "path", basePath)
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}
	return ""
}

var _ = ginkgo.BeforeSuite(func() {
	ginkgo.By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	// Set up the envtest binary directory
	if binaryPath := getFirstFoundEnvTestBinaryDir(); binaryPath != "" {
		testEnv.BinaryAssetsDirectory = binaryPath
	}

	var err error
	cfg, err = testEnv.Start()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(cfg).NotTo(gomega.BeNil())

	err = redisv1alpha1.AddToScheme(scheme.Scheme)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(k8sClient).NotTo(gomega.BeNil())
})

var _ = ginkgo.AfterSuite(func() {
	ginkgo.By("tearing down the test environment")
	if testEnv != nil {
		err := testEnv.Stop()
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	}
})

var _ = ginkgo.Describe("RedisEntry Controller Integration Tests", func() {
	ginkgo.BeforeEach(func() {
		ctx = context.Background()

		// Create a new mock Redis client for each test
		mockRedis, mock = redismock.NewClientMock()

		// Create the controller with the test client
		controllerReconciler = &RedisEntryReconciler{
			Client:      k8sClient,
			Scheme:      scheme.Scheme,
			RedisClient: mockRedis,
		}
	})

	ginkgo.AfterEach(func() {
		// Cleanup RedisEntry if it exists
		if redisEntry != nil {
			err := k8sClient.Delete(ctx, redisEntry)
			if err != nil && !apierrors.IsNotFound(err) {
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			}
		}

		// Ensure all Redis expectations were met
		gomega.Expect(mock.ExpectationsWereMet()).To(gomega.Succeed())
	})

	ginkgo.Context("Validation tests", func() {
		ginkgo.It("should reject missing key field", func() {
			redisEntry = &redisv1alpha1.RedisEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-validation",
					Namespace: "default",
				},
				Spec: redisv1alpha1.RedisEntrySpec{
					Value: "test-value",
				},
			}
			err := k8sClient.Create(ctx, redisEntry)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("spec.key"))
		})

		ginkgo.It("should handle status updates correctly", func() {
			redisEntry = &redisv1alpha1.RedisEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-status",
					Namespace: "default",
				},
				Spec: redisv1alpha1.RedisEntrySpec{
					Key:   "status-key",
					Value: "status-value",
				},
			}
			gomega.Expect(k8sClient.Create(ctx, redisEntry)).To(gomega.Succeed())

			mock.ExpectSet("status-key", "status-value", 0).SetVal("OK")

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-status",
					Namespace: "default",
				},
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Verify status was updated
			updatedEntry := &redisv1alpha1.RedisEntry{}
			gomega.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-status",
				Namespace: "default",
			}, updatedEntry)).To(gomega.Succeed())

			gomega.Expect(updatedEntry.Status.Conditions).To(gomega.HaveLen(1))
			gomega.Expect(updatedEntry.Status.Conditions[0].Type).To(gomega.Equal("Available"))
			gomega.Expect(updatedEntry.Status.Conditions[0].Status).To(gomega.Equal(metav1.ConditionTrue))
		})

		ginkgo.It("should handle error conditions correctly", func() {
			redisEntry = &redisv1alpha1.RedisEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-error-status",
					Namespace: "default",
				},
				Spec: redisv1alpha1.RedisEntrySpec{
					Key:   "error-key",
					Value: "error-value",
				},
			}
			gomega.Expect(k8sClient.Create(ctx, redisEntry)).To(gomega.Succeed())

			mock.ExpectSet("error-key", "error-value", 0).SetErr(errors.New("redis error"))

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-error-status",
					Namespace: "default",
				},
			})
			gomega.Expect(err).To(gomega.HaveOccurred())

			// Verify error status was set
			updatedEntry := &redisv1alpha1.RedisEntry{}
			gomega.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-error-status",
				Namespace: "default",
			}, updatedEntry)).To(gomega.Succeed())

			gomega.Expect(updatedEntry.Status.Conditions).To(gomega.HaveLen(1))
			gomega.Expect(updatedEntry.Status.Conditions[0].Type).To(gomega.Equal("Error"))
			gomega.Expect(updatedEntry.Status.Conditions[0].Status).To(gomega.Equal(metav1.ConditionTrue))
		})
	})
})
