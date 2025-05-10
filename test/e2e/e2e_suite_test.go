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

package e2e

import (
	"fmt"
	"os/exec"
	"testing"

	redisv1alpha1 "github.com/AAspCodes/redis-ctrl/api/v1alpha1"
	"github.com/AAspCodes/redis-ctrl/test/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	k8sClient client.Client
	testEnv   *envtest.Environment
)

func init() {
	logf.SetLogger(zap.New(zap.WriteTo(ginkgo.GinkgoWriter), zap.UseDevMode(true)))
}

// TestE2E runs the end-to-end (e2e) test suite for the project. These tests execute in an isolated,
// temporary environment to validate project changes with the purposed to be used in CI jobs.
// The default setup requires Kind, builds/loads the Manager Docker image locally, and installs
// CertManager.
func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "Starting redis-ctrl integration test suite\n")
	ginkgo.RunSpecs(t, "e2e suite")
}

var _ = ginkgo.BeforeSuite(func() {
	ginkgo.By("building the manager(Operator) image")
	// Build the operator image
	cmd := exec.Command("make", "docker-build", "IMG=redis-ctrl:test")
	_, err := utils.Run(cmd)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred(), "Failed to build the manager(Operator) image")

	ginkgo.By("loading the manager(Operator) image on Kind")
	err = utils.LoadImageToKindClusterWithName("redis-ctrl:test")
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred(), "Failed to load the manager(Operator) image into Kind")

	// Install cert-manager if not already installed
	if !utils.IsCertManagerCRDsInstalled() {
		ginkgo.By("checking if cert manager is installed already")
		if !utils.IsCertManagerCRDsInstalled() {
			_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "Installing CertManager...\n")
			gomega.Expect(utils.InstallCertManager()).To(gomega.Succeed(), "Failed to install CertManager")
		}
	}

	// Register the RedisEntry type with the scheme
	err = redisv1alpha1.AddToScheme(scheme.Scheme)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to register RedisEntry type with scheme")

	// Get the kubeconfig from the test environment
	cfg, err := config.GetConfig()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	// Create a new client using the kubeconfig
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(k8sClient).NotTo(gomega.BeNil())

	// Create namespace
	ginkgo.By("creating manager namespace")
	cmd = exec.Command("kubectl", "get", "ns", namespace)
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("kubectl", "create", "ns", namespace)
		_, err = utils.Run(cmd)
		gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred(), "Failed to create namespace")
	}

	// Install CRDs
	ginkgo.By("installing CRDs")
	cmd = exec.Command("make", "install")
	_, err = utils.Run(cmd)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred(), "Failed to install CRDs")

	// Deploy Redis
	ginkgo.By("deploying Redis")
	cmd = exec.Command("kubectl", "get", "pod", "redis", "-n", namespace)
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("kubectl", "run", "redis", "-n", namespace, "--image=redis:7")
		_, err = utils.Run(cmd)
		gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred(), "Failed to deploy Redis")

		cmd = exec.Command("kubectl", "expose", "pod", "redis", "-n", namespace, "--name=redis-redis-service", "--port=6379")
		_, err = utils.Run(cmd)
		gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred(), "Failed to expose Redis service")
	}

	// Wait for Redis to be ready
	ginkgo.By("waiting for Redis to be ready")
	gomega.Eventually(func() error {
		cmd = exec.Command("kubectl", "get", "pod", "redis", "-n", namespace, "-o", "jsonpath={.status.phase}")
		output, err := utils.Run(cmd)
		if err != nil {
			return err
		}
		if output != "Running" {
			return fmt.Errorf("Redis pod not running, status: %s", output)
		}
		return nil
	}, "2m", "5s").Should(gomega.Succeed(), "Redis failed to become ready")

	// Deploy controller
	ginkgo.By("deploying the controller-manager")
	cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
	_, err = utils.Run(cmd)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred(), "Failed to deploy the controller-manager")
})

var _ = ginkgo.AfterSuite(func() {
	ginkgo.By("tearing down the test environment")
	if testEnv != nil {
		err := testEnv.Stop()
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	}

	// Clean up Redis
	ginkgo.By("cleaning up Redis")
	cmd := exec.Command("kubectl", "delete", "ns", "redis")
	_, _ = utils.Run(cmd)
})
