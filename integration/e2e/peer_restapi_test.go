/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package e2e

import (
	"os"
	"path/filepath"
	"syscall"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hyperledger/fabric/integration/nwo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
)

var _ = Describe("PeerRestAPI", func() {
	var (
		client  *docker.Client
		tempDir string
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "nwo")
		Expect(err).NotTo(HaveOccurred())

		client, err = docker.NewClientFromEnv()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := os.RemoveAll(tempDir)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Rest tests", func() {
		var (
			network                     *nwo.Network
			ordererProcess, peerProcess ifrit.Process
			ordererRunner               *ginkgomon.Runner
			chaincode                   nwo.Chaincode
		)

		BeforeEach(func() {
			network = nwo.New(nwo.BasicEtcdRaft(), tempDir, client, StartPort(), components)

			// Generate config and bootstrap the network
			network.GenerateConfigTree()
			network.Bootstrap()

			// Start all the fabric processes
			ordererRunner, ordererProcess, peerProcess = network.StartSingleOrdererNetwork("orderer")
		})

		AfterEach(func() {
			if ordererProcess != nil {
				ordererProcess.Signal(syscall.SIGTERM)
				Eventually(ordererProcess.Wait(), network.EventuallyTimeout).Should(Receive())
			}

			if peerProcess != nil {
				peerProcess.Signal(syscall.SIGTERM)
				Eventually(peerProcess.Wait(), network.EventuallyTimeout).Should(Receive())
			}

			network.Cleanup()
		})

		Describe("Join Channel", func() {
			BeforeEach(func() {
				chaincode = nwo.Chaincode{
					Name:            "mycc",
					Version:         "0.0",
					Path:            components.Build("github.com/hyperledger/fabric/integration/chaincode/simple/cmd"),
					Lang:            "binary",
					PackageFile:     filepath.Join(tempDir, "simplecc.tar.gz"),
					Ctor:            `{"Args":["init","a","100","b","200"]}`,
					SignaturePolicy: `AND ('Org1MSP.member','Org2MSP.member')`,
					Sequence:        "1",
					InitRequired:    true,
					Label:           "my_prebuilt_chaincode",
				}
			})
			It("Ok", func() {
				orderer := network.Orderer("orderer")
				nwo.JoinOrdererAppChannel(network, "testchannel", orderer, ordererRunner)

				By("joining all peers to channel")
				peers := network.PeersWithChannel("testchannel")
				appGenesisBlock := network.LoadAppChannelGenesisBlock("testchannel")
				// network.JoinChannel("testchannel", orderer, peers...)
				nwo.JoinChannelPeersWithBlock(network, appGenesisBlock, "", peers...)

				nwo.EnableCapabilities(network, "testchannel", "Application", "V2_5", orderer, network.Peer("Org1", "peer0"), network.Peer("Org2", "peer0"))
				nwo.DeployChaincode(network, "testchannel", orderer, chaincode)
				peer := network.Peer("Org1", "peer0")
				RunQueryInvokeQuery(network, orderer, peer, "testchannel")
			})
		})
	})
})
