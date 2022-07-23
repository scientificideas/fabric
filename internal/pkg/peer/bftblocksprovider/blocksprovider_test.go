/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package bftblocksprovider_test

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/gossip"
	"github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric/common/flogging"
	gossipcommon "github.com/hyperledger/fabric/gossip/common"
	"github.com/hyperledger/fabric/internal/pkg/peer/bftblocksprovider"
	"github.com/hyperledger/fabric/internal/pkg/peer/bftblocksprovider/fake"
	"github.com/hyperledger/fabric/internal/pkg/peer/orderers"
	"github.com/hyperledger/fabric/protoutil"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

var _ = Describe("BlocksproviderBFT", func() {
	var (
		d                           *bftblocksprovider.Deliverer
		ccs                         []*grpc.ClientConn
		fakeDialer                  *fake.Dialer
		fakeGossipServiceAdapter    *fake.GossipServiceAdapter
		fakeOrdererConnectionSource *fake.OrdererConnectionSource
		fakeLedgerInfo              *fake.LedgerInfo
		fakeBlockVerifier           *fake.BlockVerifier
		fakeSigner                  *fake.Signer
		fakeDeliverStreamer         *fake.DeliverStreamer
		fakeDeliverClient           *fake.DeliverClient
		mutex                       sync.Mutex
		updateEndpointsCh           chan []*orderers.Endpoint

		endC     chan struct{}
		recvStep chan struct{}
		ctx      context.Context
		cancel   context.CancelFunc
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		recvStep = make(chan struct{})

		// appease the race detector
		recvStep := recvStep
		ctx := ctx
		cancel := cancel

		mutex.Lock()
		fakeDialer = &fake.Dialer{}
		ccs = nil

		fakeDialer.DialStub = func(string, [][]byte) (*grpc.ClientConn, error) {
			mutex.Lock()
			defer mutex.Unlock()
			cc, err := grpc.Dial("", grpc.WithInsecure())
			ccs = append(ccs, cc)
			Expect(err).NotTo(HaveOccurred())
			Expect(cc.GetState()).NotTo(Equal(connectivity.Shutdown))
			return cc, nil
		}
		mutex.Unlock()

		fakeGossipServiceAdapter = &fake.GossipServiceAdapter{}
		fakeBlockVerifier = &fake.BlockVerifier{}
		fakeSigner = &fake.Signer{}

		fakeLedgerInfo = &fake.LedgerInfo{}
		fakeLedgerInfo.LedgerHeightReturns(7, nil)

		fakeOrdererConnectionSource = &fake.OrdererConnectionSource{}
		fakeOrdererConnectionSource.GetAllEndpointsReturns([]*orderers.Endpoint{
			{Address: "orderer-address1"},
			{Address: "orderer-address2"},
			{Address: "orderer-address3"},
			{Address: "orderer-address4"},
		})
		updateEndpointsCh = make(chan []*orderers.Endpoint)
		fakeOrdererConnectionSource.InitUpdateEndpointsChannelReturns(updateEndpointsCh)

		mutex.Lock()
		fakeDeliverClient = &fake.DeliverClient{}
		fakeDeliverClient.RecvStub = func() (*orderer.DeliverResponse, error) {
			select {
			case <-recvStep:
				return nil, fmt.Errorf("fake-recv-step-error")
			case <-ctx.Done():
				return nil, fmt.Errorf("end-of-work")
			}
		}

		fakeDeliverClient.CloseSendStub = func() error {
			select {
			case recvStep <- struct{}{}:
			case <-ctx.Done():
			}
			return nil
		}

		fakeDeliverStreamer = &fake.DeliverStreamer{}
		fakeDeliverStreamer.DeliverReturns(fakeDeliverClient, nil)
		mutex.Unlock()

		d = &bftblocksprovider.Deliverer{
			Ctx:                    ctx,
			Cancel:                 cancel,
			ChainID:                "channel-id",
			Gossip:                 fakeGossipServiceAdapter,
			Ledger:                 fakeLedgerInfo,
			BlockVerifier:          fakeBlockVerifier,
			Signer:                 fakeSigner,
			DeliverStreamer:        fakeDeliverStreamer,
			TLSCertHash:            []byte("tls-cert-hash"),
			Dialer:                 fakeDialer,
			Logger:                 flogging.MustGetLogger("bftblocksprovider"),
			MinBackoffDelay:        10 * time.Millisecond,
			MaxBackoffDelay:        10 * time.Second,
			BlockCensorshipTimeout: 20 * time.Second,
			UpdateEndpointsCh:      fakeOrdererConnectionSource.InitUpdateEndpointsChannel(),
			Endpoints:              fakeOrdererConnectionSource.GetAllEndpoints(),
			BlockGossipDisabled:    false,
		}
	})

	JustBeforeEach(func() {
		endC = make(chan struct{})
		go func() {
			d.DeliverBlocks()
			close(endC)
		}()
	})

	AfterEach(func() {
		cancel()
		<-endC
	})

	It("waits patiently for new blocks from the orderer", func() {
		Consistently(endC).ShouldNot(BeClosed())
		mutex.Lock()
		defer mutex.Unlock()
		Expect(ccs[0].GetState()).NotTo(Equal(connectivity.Shutdown))
	})

	It("checks the ledger height", func() {
		Eventually(fakeLedgerInfo.LedgerHeightCallCount).Should(Equal(1))
	})

	When("the ledger returns an error", func() {
		BeforeEach(func() {
			fakeLedgerInfo.LedgerHeightReturns(0, fmt.Errorf("fake-ledger-error"))
		})

		It("exits the loop", func() {
			Eventually(endC).Should(BeClosed())
		})
	})

	It("signs the seek info request", func() {
		Eventually(fakeSigner.SignCallCount).Should(Equal(2))
		// Note, the signer is used inside a util method
		// which has its own set of tests, so checking the args
		// in this test is unnecessary
	})

	When("the signer returns an error", func() {
		BeforeEach(func() {
			fakeSigner.SignReturns(nil, fmt.Errorf("fake-signer-error"))
		})

		It("exits the loop", func() {
			Eventually(endC).Should(BeClosed())
		})
	})

	It("gets a random endpoint to connect to from the orderer connection source", func() {
		Eventually(fakeOrdererConnectionSource.GetAllEndpointsCallCount).Should(Equal(1))
		Eventually(fakeOrdererConnectionSource.InitUpdateEndpointsChannelCallCount).Should(Equal(1))
	})

	When("the orderer connect is refreshed", func() {
		BeforeEach(func() {
			refreshedC := make(chan struct{})
			close(refreshedC)
			fakeOrdererConnectionSource.GetAllEndpointsReturns([]*orderers.Endpoint{
				{Address: "orderer-address1"},
				{Address: "orderer-address2", Refreshed: refreshedC},
				{Address: "orderer-address3"},
				{Address: "orderer-address4"},
			})
			d.Endpoints = fakeOrdererConnectionSource.GetAllEndpoints()
		})

		It("disconnects and immediately tries to reconnect", func() {
			Eventually(fakeOrdererConnectionSource.GetAllEndpointsCallCount).Should(Equal(2))
		})
	})

	It("dials the endpoint", func() {
		Eventually(fakeDialer.DialCallCount).Should(Equal(4))
		addr, tlsCerts := fakeDialer.DialArgsForCall(0)
		Expect(addr).To(HavePrefix("orderer-address"))
		Expect(tlsCerts).To(BeNil())
	})

	When("the dialer returns an error", func() {
		BeforeEach(func() {
			fakeDialer.DialCalls(func(string, [][]byte) (*grpc.ClientConn, error) {
				mutex.Lock()
				defer mutex.Unlock()
				if fakeDialer.DialCallCount() == 1 {
					return nil, fmt.Errorf("fake-dial-error")
				}

				cc, err := grpc.Dial("", grpc.WithInsecure())
				Expect(err).NotTo(HaveOccurred())

				return cc, nil
			})
		})

		It("retries until dial is successful", func() {
			Eventually(fakeDialer.DialCallCount).Should(Equal(4))
		})
	})

	It("constructs a deliver client", func() {
		Eventually(fakeDeliverStreamer.DeliverCallCount).Should(Equal(4))
	})

	When("the deliver client cannot be created", func() {
		BeforeEach(func() {
			fakeDeliverStreamer.DeliverCalls(func(context.Context, *grpc.ClientConn) (orderer.AtomicBroadcast_DeliverClient, error) {
				mutex.Lock()
				defer mutex.Unlock()
				if fakeDialer.DialCallCount() == 1 {
					return nil, fmt.Errorf("deliver-error")
				}
				return fakeDeliverClient, nil
			})
		})

		It("closes the grpc connection, and tries again", func() {
			Eventually(fakeDeliverStreamer.DeliverCallCount).Should(Equal(4))
		})
	})

	When("there are consecutive errors", func() {
		BeforeEach(func() {
			fakeDeliverStreamer.DeliverCalls(func(context.Context, *grpc.ClientConn) (orderer.AtomicBroadcast_DeliverClient, error) {
				mutex.Lock()
				defer mutex.Unlock()
				if fakeDialer.DialCallCount() <= 12 {
					return nil, fmt.Errorf("deliver-error")
				}
				return fakeDeliverClient, nil
			})
		})

		It("sleeps in an exponential fashion and retries until dial is successful", func() {
			Eventually(fakeDeliverStreamer.DeliverCallCount).Should(Equal(12))
		})
	})

	It("sends a request to the deliver client for new blocks", func() {
		Eventually(fakeDeliverClient.SendCallCount).Should(Equal(4))
		mutex.Lock()
		defer mutex.Unlock()
		Expect(len(ccs)).To(Equal(4))
	})

	When("the send fails", func() {
		BeforeEach(func() {
			fakeDeliverClient.SendReturnsOnCall(0, fmt.Errorf("fake-send-error"))
			fakeDeliverClient.SendReturnsOnCall(1, nil)
			fakeDeliverClient.CloseSendStub = nil
		})

		It("disconnects, sleeps and retries until the send is successful", func() {
			Eventually(fakeDeliverClient.SendCallCount).Should(Equal(5))
			Expect(fakeDeliverClient.CloseSendCallCount()).To(Equal(1))
			mutex.Lock()
			defer mutex.Unlock()
			Expect(len(ccs)).To(Equal(5))
			Eventually(ccs[0].GetState).Should(Equal(connectivity.Shutdown))
		})
	})

	It("attempts to read blocks from the deliver stream", func() {
		Eventually(fakeDeliverClient.RecvCallCount).Should(Equal(4))
	})

	When("reading blocks from the deliver stream fails", func() {
		BeforeEach(func() {
			// appease the race detector
			ctx := ctx
			recvStep := recvStep
			fakeDeliverClient := fakeDeliverClient

			fakeDeliverClient.CloseSendStub = nil
			fakeDeliverClient.RecvStub = func() (*orderer.DeliverResponse, error) {
				if fakeDeliverClient.RecvCallCount() == 1 {
					return nil, fmt.Errorf("fake-recv-error")
				}
				select {
				case <-recvStep:
					return nil, fmt.Errorf("fake-recv-step-error")
				case <-ctx.Done():
					return nil, fmt.Errorf("end-of-work")
				}
			}
		})

		It("disconnects, and retries until the recv is successful", func() {
			Eventually(fakeDeliverClient.RecvCallCount).Should(Equal(5))
		})
	})

	When("reading blocks from the deliver stream fails and then recovers", func() {
		BeforeEach(func() {
			// appease the race detector
			ctx := ctx
			recvStep := recvStep
			fakeDeliverClient := fakeDeliverClient

			fakeDeliverClient.CloseSendStub = func() error {
				if fakeDeliverClient.CloseSendCallCount() >= 5 {
					select {
					case <-ctx.Done():
					case recvStep <- struct{}{}:
					}
				}
				return nil
			}
			fakeDeliverClient.RecvStub = func() (*orderer.DeliverResponse, error) {
				switch fakeDeliverClient.RecvCallCount() {
				case 1, 2, 4:
					return nil, fmt.Errorf("fake-recv-error")
				case 3:
					return &orderer.DeliverResponse{
						Type: &orderer.DeliverResponse_Block{
							Block: &common.Block{
								Header: &common.BlockHeader{
									Number: 8,
								},
							},
						},
					}, nil
				default:
					select {
					case <-recvStep:
						return nil, fmt.Errorf("fake-recv-step-error")
					case <-ctx.Done():
						return nil, fmt.Errorf("end-of-work")
					}
				}
			}
		})

		It("disconnects, and retries until the recv is successful and resets the failure count", func() {
			Eventually(fakeDeliverClient.RecvCallCount).Should(Equal(5))
		})
	})

	When("the deliver client returns a block", func() {
		var fakeDeliverClients []*fake.DeliverClient

		BeforeEach(func() {
			// appease the race detector
			mutex.Lock()
			fakeDeliverClients = nil
			mutex.Unlock()
			ctx := ctx
			recvStep := recvStep
			fakeDeliverStreamer := fakeDeliverStreamer
			fakeDeliverStreamer.DeliverCalls(func(context.Context, *grpc.ClientConn) (orderer.AtomicBroadcast_DeliverClient, error) {
				fakeDeliverClient := &fake.DeliverClient{}
				fakeDeliverClient.RecvStub = func() (*orderer.DeliverResponse, error) {
					if fakeDeliverClient.RecvCallCount() == 1 {
						return &orderer.DeliverResponse{
							Type: &orderer.DeliverResponse_Block{
								Block: &common.Block{
									Header: &common.BlockHeader{
										Number: 7,
									},
								},
							},
						}, nil
					}
					select {
					case <-recvStep:
						return nil, fmt.Errorf("fake-recv-step-error")
					case <-ctx.Done():
						return nil, fmt.Errorf("end-of-work")
					}
				}

				fakeDeliverClient.CloseSendStub = func() error {
					select {
					case recvStep <- struct{}{}:
					case <-ctx.Done():
					}
					return nil
				}

				mutex.Lock()
				fakeDeliverClients = append(fakeDeliverClients, fakeDeliverClient)
				mutex.Unlock()

				return fakeDeliverClient, nil
			})
		})

		It("receives the block and loops, not sleeping", func() {
			Eventually(func() int {
				mutex.Lock()
				defer mutex.Unlock()
				return len(fakeDeliverClients)
			}).Should(Equal(4))
			Eventually(func() int {
				mutex.Lock()
				defer mutex.Unlock()
				return fakeDeliverClients[0].RecvCallCount()
			}).Should(Equal(2))
			Eventually(func() int {
				mutex.Lock()
				defer mutex.Unlock()
				return fakeDeliverClients[1].RecvCallCount()
			}).Should(Equal(2))
			Eventually(func() int {
				mutex.Lock()
				defer mutex.Unlock()
				return fakeDeliverClients[2].RecvCallCount()
			}).Should(Equal(2))
			Eventually(func() int {
				mutex.Lock()
				defer mutex.Unlock()
				return fakeDeliverClients[3].RecvCallCount()
			}).Should(Equal(2))
		})

		It("checks the validity of the block", func() {
			Eventually(fakeBlockVerifier.VerifyBlockCallCount).Should(Equal(1))
			channelID, blockNum, block := fakeBlockVerifier.VerifyBlockArgsForCall(0)
			Expect(channelID).To(Equal(gossipcommon.ChannelID("channel-id")))
			Expect(blockNum).To(Equal(uint64(7)))
			Expect(proto.Equal(block, &common.Block{
				Header: &common.BlockHeader{
					Number: 7,
				},
			})).To(BeTrue())
		})

		When("the block is invalid", func() {
			BeforeEach(func() {
				fakeBlockVerifier.VerifyBlockReturns(fmt.Errorf("fake-verify-error"))
			})

			It("disconnects, sleeps, and tries again", func() {
				Eventually(fakeBlockVerifier.VerifyBlockCallCount).Should(Equal(1))
				Eventually(func() int {
					mutex.Lock()
					defer mutex.Unlock()
					return fakeDeliverClients[0].RecvCallCount()
				}).Should(Equal(2))
				mutex.Lock()
				defer mutex.Unlock()
				Expect(len(ccs)).To(Equal(4))
			})
		})

		It("adds the payload to gossip", func() {
			Eventually(fakeGossipServiceAdapter.AddPayloadCallCount).Should(Equal(1))
			channelID, payload := fakeGossipServiceAdapter.AddPayloadArgsForCall(0)
			Expect(channelID).To(Equal("channel-id"))
			Expect(payload).To(Equal(&gossip.Payload{
				Data: protoutil.MarshalOrPanic(&common.Block{
					Header: &common.BlockHeader{
						Number: 7,
					},
				}),
				SeqNum: 7,
			}))
		})

		When("adding the payload fails", func() {
			BeforeEach(func() {
				fakeGossipServiceAdapter.AddPayloadReturns(fmt.Errorf("payload-error"))
			})

			It("disconnects, sleeps, and tries again", func() {
				Eventually(fakeBlockVerifier.VerifyBlockCallCount).Should(Equal(1))
				Eventually(func() int {
					runtime.Gosched()
					mutex.Lock()
					defer mutex.Unlock()
					return fakeDeliverClients[0].CloseSendCallCount()
				}).Should(Equal(1))
				mutex.Lock()
				defer mutex.Unlock()
				Expect(len(ccs)).To(Equal(4))
			})
		})

		It("gossips the block to the other peers", func() {
			Eventually(fakeGossipServiceAdapter.GossipCallCount).Should(Equal(1))
			msg := fakeGossipServiceAdapter.GossipArgsForCall(0)
			Expect(msg).To(Equal(&gossip.GossipMessage{
				Nonce:   0,
				Tag:     gossip.GossipMessage_CHAN_AND_ORG,
				Channel: []byte("channel-id"),
				Content: &gossip.GossipMessage_DataMsg{
					DataMsg: &gossip.DataMessage{
						Payload: &gossip.Payload{
							Data: protoutil.MarshalOrPanic(&common.Block{
								Header: &common.BlockHeader{
									Number: 7,
								},
							}),
							SeqNum: 7,
						},
					},
				},
			}))
		})

		When("gossip dissemination is disabled", func() {
			BeforeEach(func() {
				d.BlockGossipDisabled = true
			})

			It("doesn't gossip, only adds to the payload buffer", func() {
				Eventually(fakeGossipServiceAdapter.AddPayloadCallCount).Should(Equal(1))
				channelID, payload := fakeGossipServiceAdapter.AddPayloadArgsForCall(0)
				Expect(channelID).To(Equal("channel-id"))
				Expect(payload).To(Equal(&gossip.Payload{
					Data: protoutil.MarshalOrPanic(&common.Block{
						Header: &common.BlockHeader{
							Number: 7,
						},
					}),
					SeqNum: 7,
				}))

				Consistently(fakeGossipServiceAdapter.GossipCallCount).Should(Equal(0))
			})
		})
	})

	When("the deliver client returns a status", func() {
		var (
			status             common.Status
			fakeDeliverClients []*fake.DeliverClient
		)

		BeforeEach(func() {
			status := status
			status = common.Status_SUCCESS

			// appease the race detector
			mutex.Lock()
			fakeDeliverClients = nil
			mutex.Unlock()

			ctx := ctx
			recvStep := recvStep
			fakeDeliverStreamer := fakeDeliverStreamer
			fakeDeliverStreamer.DeliverCalls(func(context.Context, *grpc.ClientConn) (orderer.AtomicBroadcast_DeliverClient, error) {
				fakeDeliverClient := &fake.DeliverClient{}
				fakeDeliverClient.RecvStub = func() (*orderer.DeliverResponse, error) {
					if fakeDeliverClient.RecvCallCount() == 1 {
						return &orderer.DeliverResponse{
							Type: &orderer.DeliverResponse_Status{
								Status: status,
							},
						}, nil
					}
					select {
					case <-recvStep:
						return nil, fmt.Errorf("fake-recv-step-error")
					case <-ctx.Done():
						return nil, fmt.Errorf("end-of-work")
					}
				}

				fakeDeliverClient.CloseSendStub = func() error {
					select {
					case recvStep <- struct{}{}:
					case <-ctx.Done():
					}
					return nil
				}

				mutex.Lock()
				fakeDeliverClients = append(fakeDeliverClients, fakeDeliverClient)
				mutex.Unlock()

				return fakeDeliverClient, nil
			})
		})

		It("disconnects with an error, because the block request is infinite and should never complete", func() {
			Eventually(func() int {
				mutex.Lock()
				defer mutex.Unlock()
				return len(fakeDeliverClients)
			}).Should(Equal(4))
			Eventually(func() int {
				mutex.Lock()
				defer mutex.Unlock()
				return fakeDeliverClients[0].RecvCallCount()
			}).Should(Equal(2))
			Eventually(func() int {
				mutex.Lock()
				defer mutex.Unlock()
				return fakeDeliverClients[1].RecvCallCount()
			}).Should(Equal(2))
			Eventually(func() int {
				mutex.Lock()
				defer mutex.Unlock()
				return fakeDeliverClients[2].RecvCallCount()
			}).Should(Equal(2))
			Eventually(func() int {
				mutex.Lock()
				defer mutex.Unlock()
				return fakeDeliverClients[3].RecvCallCount()
			}).Should(Equal(2))
		})

		When("the status is not successful", func() {
			BeforeEach(func() {
				status = common.Status_FORBIDDEN
			})

			It("still disconnects with an error", func() {
				Eventually(func() int {
					mutex.Lock()
					defer mutex.Unlock()
					return len(fakeDeliverClients)
				}).Should(Equal(4))
				Eventually(func() int {
					mutex.Lock()
					defer mutex.Unlock()
					return fakeDeliverClients[0].RecvCallCount()
				}).Should(Equal(2))
				Eventually(func() int {
					mutex.Lock()
					defer mutex.Unlock()
					return fakeDeliverClients[1].RecvCallCount()
				}).Should(Equal(2))
				Eventually(func() int {
					mutex.Lock()
					defer mutex.Unlock()
					return fakeDeliverClients[2].RecvCallCount()
				}).Should(Equal(2))
				Eventually(func() int {
					mutex.Lock()
					defer mutex.Unlock()
					return fakeDeliverClients[3].RecvCallCount()
				}).Should(Equal(2))
			})
		})
	})
})
