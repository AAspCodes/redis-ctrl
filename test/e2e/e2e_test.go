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
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/AAspCodes/redis-ctrl/test/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	timeout  = time.Second * 10
	interval = time.Millisecond * 250

	namespace              = "redis-ctrl-system"
	serviceAccountName     = "redis-ctrl-controller-manager"
	metricsServiceName     = "controller-manager-metrics-service"
	metricsRoleBindingName = "redis-ctrl-metrics-binding"
	projectImage           = "redis-ctrl:test"
)

var _ = ginkgo.Describe("Manager", ginkgo.Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	ginkgo.BeforeAll(func() {
		ginkgo.By("labeling the namespace to enforce the restricted security policy")
		cmd := exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err := utils.Run(cmd)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to label namespace with restricted policy")

		ginkgo.By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to install CRDs")

		ginkgo.By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
		_, err = utils.Run(cmd)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to deploy the controller-manager")
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	ginkgo.AfterAll(func() {
		ginkgo.By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		_, _ = utils.Run(cmd)

		ginkgo.By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, _ = utils.Run(cmd)

		ginkgo.By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		ginkgo.By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	ginkgo.AfterEach(func() {
		specReport := ginkgo.CurrentSpecReport()
		if specReport.Failed() {
			ginkgo.By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			ginkgo.By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			ginkgo.By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			ginkgo.By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "Pod description:\n %s", podDescription)
			} else {
				_, _ = fmt.Fprintf(ginkgo.GinkgoWriter, "Failed to get pod description: %s", err)
			}
		}
	})

	ginkgo.Context("Manager", func() {
		ginkgo.It("should run successfully", func() {
			ginkgo.By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g gomega.Gomega) {
				// Get the name of the controller-manager pod
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "jsonpath={.items[0].metadata.name}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(gomega.HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(gomega.ContainSubstring("controller-manager"))

				// Validate the pod's status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName,
					"-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(output).To(gomega.Equal("Running"), "Incorrect controller-manager pod status")
			}
			gomega.Eventually(verifyControllerUp).Should(gomega.Succeed())
		})

		ginkgo.It("should ensure the metrics endpoint is serving metrics", func() {
			ginkgo.By("waiting for the controller deployment to be ready")
			verifyControllerDeploymentReady := func(g gomega.Gomega) {
				cmd := exec.Command("kubectl", "rollout", "status", "deployment/redis-ctrl-controller-manager", "-n", namespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(gomega.HaveOccurred())
			}
			gomega.Eventually(verifyControllerDeploymentReady, "60s", "5s").Should(gomega.Succeed())

			ginkgo.By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			// Delete existing binding if it exists
			cmd := exec.Command("kubectl", "delete", "clusterrolebinding", metricsRoleBindingName, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			cmd = exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
				"--clusterrole=metrics-reader",
				fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName),
			)
			_, err := utils.Run(cmd)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to create ClusterRoleBinding")

			ginkgo.By("validating that the metrics service is available")
			verifyMetricsServiceReady := func(g gomega.Gomega) {
				cmd := exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(gomega.HaveOccurred())
			}
			gomega.Eventually(verifyMetricsServiceReady, "60s", "5s").Should(gomega.Succeed())

			ginkgo.By("getting the service account token")
			token, err := serviceAccountToken()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(token).NotTo(gomega.BeEmpty())

			ginkgo.By("waiting for the metrics endpoint to be ready")
			verifyMetricsEndpointReady := func(g gomega.Gomega) {
				cmd := exec.Command("kubectl", "get", "endpoints", metricsServiceName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(output).To(gomega.ContainSubstring("8443"), "Metrics endpoint is not ready")
			}
			gomega.Eventually(verifyMetricsEndpointReady).Should(gomega.Succeed())

			ginkgo.By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g gomega.Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(output).To(gomega.ContainSubstring("controller-runtime.metrics\tServing metrics server"),
					"Metrics server not yet started")
			}
			gomega.Eventually(verifyMetricsServerStarted).Should(gomega.Succeed())

			ginkgo.By("creating the curl-metrics pod to access the metrics endpoint")
			cmd = exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
				"--namespace", namespace,
				"--image=curlimages/curl:7.78.0",
				"--overrides", fmt.Sprintf(`{
					"spec": {
						"serviceAccountName": "%s",
						"containers": [{
							"name": "curl",
							"image": "curlimages/curl:7.78.0",
							"command": ["curl", "-k", "-H", "Authorization: Bearer %s", "https://%s.%s.svc:8443/metrics"]
						}]
					}
				}`, serviceAccountName, token, metricsServiceName, namespace))
			_, err = utils.Run(cmd)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to create curl-metrics pod")

			ginkgo.By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g gomega.Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(output).To(gomega.Equal("Succeeded"), "curl pod in wrong status")
			}
			gomega.Eventually(verifyCurlUp, 5*time.Minute).Should(gomega.Succeed())

			ginkgo.By("getting the metrics by checking curl-metrics logs")
			metricsOutput := getMetricsOutput()
			gomega.Expect(metricsOutput).To(gomega.ContainSubstring(
				"controller_runtime_reconcile_total",
			))
		})
	})
})

// serviceAccountToken returns the token for the service account.
func serviceAccountToken() (string, error) {
	var err error
	var out string
	verifyTokenCreation := func(g gomega.Gomega) {
		// Execute kubectl command to create the token
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace, serviceAccountName))
		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(gomega.HaveOccurred())

		// Parse the JSON output to extract the token
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		out = token.Status.Token
	}
	gomega.Eventually(verifyTokenCreation).Should(gomega.Succeed())

	return out, err
}

// tokenRequest represents the structure of the token request response.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() string {
	ginkgo.By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	metricsOutput, err := utils.Run(cmd)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to retrieve logs from curl pod")
	gomega.Expect(metricsOutput).To(gomega.ContainSubstring("< HTTP/1.1 200 OK"))
	return metricsOutput
}
