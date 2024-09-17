/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hyperledger/fabric/integration/channelparticipation"
	"github.com/hyperledger/fabric/integration/nwo"
	"github.com/hyperledger/fabric/integration/nwo/commands"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/gmeasure"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
)

var _ = Describe("Network", func() {
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
		os.RemoveAll(tempDir)
	})

	DescribeTableSubtree("etcdraft network", func(desc string, usePutStateBatch bool) {
		var network *nwo.Network
		var ordererRunner *ginkgomon.Runner
		var ordererProcess, peerProcess ifrit.Process

		BeforeEach(func() {
			network = nwo.New(nwo.BasicEtcdRaft(), tempDir, client, StartPort(), components)
			network.UsePutStateBatch = usePutStateBatch

			// Generate config and bootstrap the network
			network.GenerateConfigTree()
			network.Bootstrap()

			// Start all of the fabric processes
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

		It("deploys and executes chaincode (simple) using _lifecycle", func() {
			orderer := network.Orderer("orderer")
			channelparticipation.JoinOrdererJoinPeersAppChannel(network, "testchannel", orderer, ordererRunner)
			peer := network.Peer("Org1", "peer0")

			chaincode := nwo.Chaincode{
				Name:            "mycc",
				Version:         "0.0",
				Path:            "github.com/hyperledger/fabric/integration/chaincode/onlyput/cmd",
				Lang:            "golang",
				PackageFile:     filepath.Join(tempDir, "onlyput.tar.gz"),
				Ctor:            `{"Args":["init"]}`,
				SignaturePolicy: `AND ('Org1MSP.member','Org2MSP.member')`,
				Sequence:        "1",
				InitRequired:    true,
				Label:           "my_onlyput_chaincode",
			}

			network.VerifyMembership(network.PeersWithChannel("testchannel"), "testchannel")

			nwo.EnableCapabilities(
				network,
				"testchannel",
				"Application", "V2_0",
				orderer,
				network.PeersWithChannel("testchannel")...,
			)
			nwo.DeployChaincode(network, "testchannel", orderer, chaincode)

			By("run invoke where onli put state")
			experiment := gmeasure.NewExperiment("Only PUT State " + desc)
			AddReportEntry(experiment.Name, experiment)

			experiment.SampleDuration("invoke N-5 cycle-1", func(idx int) {
				RunInvokeOnlyPutState(network, orderer, peer, "invoke", 1)
			}, gmeasure.SamplingConfig{N: 5})

			experiment.SampleDuration("invoke N-10 cycle-1000", func(idx int) {
				RunInvokeOnlyPutState(network, orderer, peer, "invoke", 1000)
			}, gmeasure.SamplingConfig{N: 10})

			experiment.SampleDuration("invoke N-10 cycle-10000", func(idx int) {
				RunInvokeOnlyPutState(network, orderer, peer, "invoke", 10000)
			}, gmeasure.SamplingConfig{N: 10})
		})
	},
		Entry("without batch", "without batch", false),
		Entry("with batch", "with batch", true),
	)
})

func RunInvokeOnlyPutState(n *nwo.Network, orderer *nwo.Orderer, peer *nwo.Peer, fn string, initial int) {
	sess, err := n.PeerUserSession(peer, "User1", commands.ChaincodeInvoke{
		ChannelID: "testchannel",
		Orderer:   n.OrdererAddress(orderer, nwo.ListenPort),
		Name:      "mycc",
		Ctor:      `{"Args":["` + fn + `","` + fmt.Sprint(initial) + `"]}`,
		PeerAddresses: []string{
			n.PeerAddress(n.Peer("Org1", "peer0"), nwo.ListenPort),
			n.PeerAddress(n.Peer("Org2", "peer0"), nwo.ListenPort),
		},
		WaitForEvent: true,
	})
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess, n.EventuallyTimeout).Should(gexec.Exit(0))
	Expect(sess.Err).To(gbytes.Say("Chaincode invoke successful. result: status:200"))
}
