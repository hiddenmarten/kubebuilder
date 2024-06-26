/*
Copyright 2022 The Kubernetes Authors.

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
	"sigs.k8s.io/kubebuilder/v4/pkg/machinery"
)

var _ machinery.Template = &SuiteTest{}

type Test struct {
	machinery.TemplateMixin
	machinery.BoilerplateMixin
	machinery.RepositoryMixin
	machinery.ProjectNameMixin
}

func (f *Test) SetTemplateDefaults() error {
	if f.Path == "" {
		f.Path = "test/e2e/e2e_test.go"
	}

	f.TemplateBody = TestTemplate
	return nil
}

var TestTemplate = `{{ .Boilerplate }}


package e2e

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"{{ .Repo }}/test/utils"
)

const namespace = "{{ .ProjectName }}-system"
const deploymentName = "{{ .ProjectName }}-controller-manager"

var _ = Describe("controller", Ordered, func() {
	BeforeAll(func() {
		var err error
		By("create kind environment", func() {
			cmd := exec.Command("make", "kind-create")
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())
		})

		By("upload latest image to kind cluster", func() {
			cmd := exec.Command("make", "kind-load")
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())
		})

		By("prepare k8s", func() {
			cmd := exec.Command("make", "k8s-prepare")
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())
		})

		By("deploy controller-manager", func() {
			cmd := exec.Command("make", "deploy")
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())
		})

	})

	AfterAll(func() {
		var err error
		By("cleanup", func() {
			cmd := exec.Command("make", "undeploy")
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())
		})
	})

	Context("Operator", func() {
		It("should run successfully", func() {
			kubeconfig := config.GetConfigOrDie()
			clientset, err := utils.GetClientset(kubeconfig)

			deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
			if err != nil {
				panic(err.Error())
			}

			// Get the pods under the deployment
			selector := deployment.Spec.Selector.MatchLabels
			labelSelector := metav1.LabelSelector{MatchLabels: selector}

			pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
				LabelSelector: metav1.FormatLabelSelector(&labelSelector),
			})

			if err != nil {
				panic(err.Error())
			}

			stopCh := make(chan struct{}, 1)
			readyCh := make(chan struct{})
			localPort, err := utils.GetFreePort()
			localPortStr := strconv.Itoa(localPort)

			// Call the function to run port forward
			err = utils.RunPortForward(kubeconfig, namespace, pods.Items[0].Name, []string{fmt.Sprintf("%s:8081", localPortStr)}, stopCh, readyCh)
			if err != nil {
				panic(err.Error())
			}
			<-readyCh

			readyzURL := fmt.Sprintf("http://localhost:%s/readyz", localPortStr)
			client := resty.New().SetTimeout(5 * time.Second)
			resp, err := client.R().Get(readyzURL)
			if err != nil {
				panic(err.Error())
			}

			Expect(resp.StatusCode()).To(Equal(http.StatusOK))
			Expect(resp.Body()).To(Equal([]uint8{'o', 'k'}))

			close(stopCh)
			<-stopCh
		})
	})
})
`
