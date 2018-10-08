/*
 * Copyright (C) 2018 Nalej - All Rights Reserved
 */

// Launch a simple test to deploy some components in Kubernetes
// Prerequirements
// 1.- Launch minikube

/*
RUN_INTEGRATION_TEST=true
IT_K8S_KUBECONFIG=/Users/daniel/.kube/config
IT_RKE_BINARY=/Users/daniel/development/rke/rke
 */

package installer

import (
	"context"
	"fmt"
	"github.com/nalej/grpc-infrastructure-go"
	"github.com/nalej/grpc-installer-go"
	"github.com/nalej/grpc-utils/pkg/test"
	"github.com/nalej/installer/internal/pkg/utils"
	cfg "github.com/nalej/installer/internal/pkg/server/config"
	"github.com/nalej/installer/internal/pkg/workflow/commands/sync/k8s"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"
)

const SampleComponent = `
apiVersion: apps/v1
kind: Deployment
metadata:
  cluster: application
  name: NAME
  namespace: NAMESPACE
  labels:
    app: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.7.9
        ports:
        - containerPort: 80
`

func createDeployment(basePath string, namespace string, index int) {
	toWrite := strings.Replace(SampleComponent, "NAMESPACE", namespace, 1)
	toWrite = strings.Replace(toWrite, "NAME", fmt.Sprintf("nginx-%d", index), 1)
	outputPath := path.Join(basePath, fmt.Sprintf("component%d.yaml", index))
	err := ioutil.WriteFile(outputPath, []byte(toWrite), 777)
	gomega.Expect(err).To(gomega.Succeed())
	log.Debug().Str("file", outputPath).Msg("deployment has been created")
}


var _ = ginkgo.Describe("Installer", func(){

	const numDeployments = 2
	const targetNamespace = "test-it-install"

	if ! utils.RunIntegrationTests() {
		log.Warn().Msg("Integration tests are skipped")
		return
	}
	var (
		kubeConfigFile = os.Getenv("IT_K8S_KUBECONFIG")
	)

	if kubeConfigFile == "" {
		ginkgo.Fail("missing environment variables")
	}

	var componentsDir string
	var binaryDir string
	var tempDir string
	var kubeConfigRaw string

	// gRPC server
	var server * grpc.Server
	// grpc test listener
	var listener * bufconn.Listener
	// client
	var client grpc_installer_go.InstallerClient

	ginkgo.BeforeSuite(func(){

		// Load data and ENV variables.
		kubeConfigContent, lErr := utils.GetKubeConfigContent(kubeConfigFile)
		gomega.Expect(lErr).To(gomega.Succeed())
		kubeConfigRaw = kubeConfigContent

		cd, err := ioutil.TempDir("", "installITComponents")
		gomega.Expect(err).To(gomega.Succeed())
		componentsDir = cd

		td, err := ioutil.TempDir("", "installITTemp")
		gomega.Expect(err).To(gomega.Succeed())
		tempDir = td
		// TODO copy RKE binary for raw install.
		binaryDir = td

		for i:= 0; i< numDeployments; i++{
			createDeployment(componentsDir, targetNamespace, i)
		}

		config := cfg.Config{
			ComponentsPath: componentsDir,
			BinaryPath:     binaryDir,
			TempPath:       tempDir,
		}

		// Launch gRPC server
		listener = test.GetDefaultListener()

		server = grpc.NewServer()

		manager := NewManager(config)
		handler := NewHandler(manager)
		grpc_installer_go.RegisterInstallerServer(server, handler)

		test.LaunchServer(server, listener)

		conn, err := test.GetConn(*listener)
		gomega.Expect(err).To(gomega.Succeed())
		client = grpc_installer_go.NewInstallerClient(conn)

	})

	ginkgo.AfterSuite(func(){
		os.RemoveAll(componentsDir)
		tc := k8s.NewTestCleaner(kubeConfigFile, targetNamespace)
		gomega.Expect(tc.DeleteAll()).To(gomega.Succeed())
	})

	ginkgo.PContext("On a base system", func() {
		ginkgo.PIt("should be able to install an application cluster from scratch", func(){

		})
	})

	ginkgo.Context("On a kubernetes cluster", func() {
		ginkgo.It("should be able to install an application cluster", func(){
			ginkgo.By("installing the cluster")
		    installRequest := &grpc_installer_go.InstallRequest{
				InstallId:            "test-install-id",
				OrganizationId:       "test-org-id",
				ClusterId:            "test-cluster-id",
				ClusterType:          grpc_infrastructure_go.ClusterType_KUBERNETES,
				InstallBaseSystem:    false,
				KubeConfigRaw:        kubeConfigRaw,
			}
			response, err := client.InstallCluster(context.Background(), installRequest)
			gomega.Expect(err).To(gomega.Succeed())
			gomega.Expect(response).ToNot(gomega.BeNil())
			gomega.Expect(response.InstallId).Should(gomega.Equal(installRequest.InstallId))

			// Wait for it to finish
			maxWait := 1000
			finished := false

			installID := &grpc_installer_go.InstallId{
				InstallId:            installRequest.InstallId,
			}
			ginkgo.By("checking the install progress")
			for i := 0; i < maxWait && !finished; i++ {
				time.Sleep(time.Second)
				progress, err := client.CheckProgress(context.Background(), installID)
				gomega.Expect(err).To(gomega.Succeed())
				finished = (progress.State == grpc_installer_go.InstallProgress_FINISHED) ||
					(progress.State == grpc_installer_go.InstallProgress_ERROR)
			}
			progress, err := client.CheckProgress(context.Background(), installID)
			gomega.Expect(err).To(gomega.Succeed())
			gomega.Expect(progress.State).Should(gomega.Equal(grpc_installer_go.InstallProgress_FINISHED))
			ginkgo.By("removing the install")
			removeRequest := &grpc_installer_go.RemoveInstallRequest{
				InstallId:            installRequest.InstallId,
			}
			client.RemoveInstall(context.Background(), removeRequest)
		})
	})

})
