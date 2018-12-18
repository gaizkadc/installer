/*
 * Copyright (C) 2018 Nalej - All Rights Reserved
 */

/*
RUN_INTEGRATION_TEST=true
IT_K8S_KUBECONFIG=/Users/daniel/.kube/config
*/

package k8s

import (
	"github.com/nalej/installer/internal/pkg/utils"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/rs/zerolog/log"
)

var _ = ginkgo.Describe("A create cluster config command", func() {

	if !utils.RunIntegrationTests() {
		log.Warn().Msg("Integration tests are skipped")
		return
	}

	if itKubeConfigFile == "" {
		ginkgo.Fail("missing environment variables")
	}

	ginkgo.It("should be able to create the config map", func() {
		ccc := NewCreateClusterConfig(
			itKubeConfigFile, "testOrg", "testCluster",
			"managementPublicHost", "managementPublicPort",
			"clusterPublicHostname",
			"dnsPublicHost", "53",
			"MINIKUBE")
		result, err := ccc.Run("createClusterConfig")
		gomega.Expect(err).To(gomega.Succeed())
		gomega.Expect(result.Success).Should(gomega.BeTrue())
	})

})