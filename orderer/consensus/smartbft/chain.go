/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package smartbft

import (
	"time"
	"github.com/hyperledger/fabric/bccsp"
	"sync/atomic"

	smartbft "github.com/SmartBFT-Go/consensus/pkg/consensus"
	"github.com/SmartBFT-Go/consensus/pkg/types"
	"github.com/SmartBFT-Go/consensus/pkg/wal"
	cb "github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/common/policies"
	"github.com/hyperledger/fabric/orderer/common/cluster"
	"github.com/hyperledger/fabric/orderer/consensus"
	"github.com/hyperledger/fabric-protos-go/msp"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

//go:generate counterfeiter -o mocks/mock_blockpuller.go . BlockPuller

// BlockPuller is used to pull blocks from other OSN
type BlockPuller interface {
	PullBlock(seq uint64) *cb.Block
	HeightsByEndpoints() (map[string]uint64, error)
	Close()
}

// WALConfig consensus specific configuration parameters from orderer.yaml; for SmartBFT only WALDir is relevant.
type WALConfig struct {
	WALDir            string // WAL data of <my-channel> is stored in WALDir/<my-channel>
	SnapDir           string // Snapshots of <my-channel> are stored in SnapDir/<my-channel>
	EvictionSuspicion string // Duration threshold that the node samples in order to suspect its eviction from the channel.
}

type ConfigValidator interface {
	ValidateConfig(env *cb.Envelope) error
}

type signerSerializer interface {
	// Sign a message and return the signature over the digest, or error on failure
	Sign(message []byte) ([]byte, error)

	// Serialize converts an identity to bytes
	Serialize() ([]byte, error)
}

// BFTChain implements Chain interface to wire with
// BFT smart library
type BFTChain struct {
	RuntimeConfig    *atomic.Value
	Channel          string
	Config           types.Configuration
	BlockPuller      BlockPuller
	Comm             cluster.Communicator
	SignerSerializer signerSerializer
	PolicyManager    policies.Manager
	Logger           *flogging.FabricLogger
	WALDir           string
	consensus        *smartbft.Consensus
	support          consensus.ConsenterSupport
	verifier         *Verifier
	assembler        *Assembler
	Metrics          *Metrics
	bccsp            bccsp.BCCSP
}

// NewChain creates new BFT Smart chain
func NewChain(
	cv ConfigValidator,
	selfID uint64,
	config types.Configuration,
	walDir string,
	blockPuller BlockPuller,
	comm cluster.Communicator,
	signerSerializer signerSerializer,
	policyManager policies.Manager,
	support consensus.ConsenterSupport,
	metrics *Metrics,
	bccsp bccsp.BCCSP,

) (*BFTChain, error) {

	requestInspector := &RequestInspector{
		ValidateIdentityStructure: func(_ *msp.SerializedIdentity) error {
			return nil
		},
	}

	logger := flogging.MustGetLogger("orderer.consensus.smartbft.chain").With(zap.String("channel", support.ChannelID()))

	c := &BFTChain{
		RuntimeConfig:    &atomic.Value{},
		Channel:          support.ChannelID(),
		Config:           config,
		WALDir:           walDir,
		Comm:             comm,
		support:          support,
		SignerSerializer: signerSerializer,
		PolicyManager:    policyManager,
		BlockPuller:      blockPuller,
		Logger:           logger,
		// todo: BFT metrics
		//Metrics: &Metrics{
		//	ClusterSize:          metrics.ClusterSize.With("channel", support.ChannelID()),
		//	CommittedBlockNumber: metrics.CommittedBlockNumber.With("channel", support.ChannelID()),
		//	IsLeader:             metrics.IsLeader.With("channel", support.ChannelID()),
		//	LeaderID:             metrics.LeaderID.With("channel", support.ChannelID()),
		//},
	}

	lastBlock := LastBlockFromLedgerOrPanic(support, c.Logger)
	lastConfigBlock := LastConfigBlockFromLedgerOrPanic(support, c.Logger)

	rtc := RuntimeConfig{
		logger: logger,
		id:     selfID,
	}
	rtc, err := rtc.BlockCommitted(lastConfigBlock, bccsp)
	if err != nil {
		return nil, errors.Wrap(err, "failed constructing RuntimeConfig")
	}
	rtc, err = rtc.BlockCommitted(lastBlock, bccsp)
	if err != nil {
		return nil, errors.Wrap(err, "failed constructing RuntimeConfig")
	}

	c.RuntimeConfig.Store(rtc)

	//c.verifier = buildVerifier(cv, c.RuntimeConfig, support, requestInspector, policyManager)
	c.consensus = bftSmartConsensusBuild(c, requestInspector)

	// Setup communication with list of remotes notes for the new channel
	c.Comm.Configure(c.support.ChannelID(), rtc.RemoteNodes)

	if err := c.consensus.ValidateConfiguration(rtc.Nodes); err != nil {
		return nil, errors.Wrap(err, "failed to verify SmartBFT-Go configuration")
	}

	logger.Infof("SmartBFT-v3 is now servicing chain %s", support.ChannelID())

	return c, nil
}

func bftSmartConsensusBuild(
	c *BFTChain,
	requestInspector *RequestInspector,
) *smartbft.Consensus {
	var err error

	rtc := c.RuntimeConfig.Load().(RuntimeConfig)

	latestMetadata, err := getViewMetadataFromBlock(rtc.LastBlock)
	if err != nil {
		c.Logger.Panicf("Failed extracting view metadata from ledger: %v", err)
	}

	var consensusWAL *wal.WriteAheadLogFile
	var walInitState [][]byte

	c.Logger.Infof("Initializing a WAL for chain %s, on dir: %s", c.support.ChannelID(), c.WALDir)
	consensusWAL, walInitState, err = wal.InitializeAndReadAll(c.Logger, c.WALDir, wal.DefaultOptions())
	if err != nil {
		c.Logger.Panicf("failed to initialize a WAL for chain %s, err %s", c.support.ChannelID(), err)
	}

	clusterSize := uint64(len(rtc.Nodes))

	// report cluster size
	// TODO: implement ClusterSize for Metrics and uncomment the line below
	// c.Metrics.ClusterSize.Set(float64(clusterSize))

	sync := &Synchronizer{
		selfID:          rtc.id,
		BlockToDecision: c.blockToDecision,
		OnCommit:        c.updateRuntimeConfig,
		Support:         c.support,
		BlockPuller:     c.BlockPuller,
		ClusterSize:     clusterSize,
		Logger:          c.Logger,
		LatestConfig: func() (types.Configuration, []uint64) {
			rtc := c.RuntimeConfig.Load().(RuntimeConfig)
			return rtc.BFTConfig, rtc.Nodes
		},
	}

	channelDecorator := zap.String("channel", c.support.ChannelID())
	logger := flogging.MustGetLogger("orderer.consensus.smartbft.consensus").With(channelDecorator)

	c.assembler = &Assembler{
		RuntimeConfig:   c.RuntimeConfig,
		VerificationSeq: c.verifier.VerificationSequence,
		Logger:          flogging.MustGetLogger("orderer.consensus.smartbft.assembler").With(channelDecorator),
	}

	consensus := &smartbft.Consensus{
		Config:   c.Config,
		Logger:   logger,
		Verifier: c.verifier,
		Signer: &Signer{
			// TODO: implement the fields in the structure
			// ID:               c.Config.SelfID,
			// Logger:           flogging.MustGetLogger("orderer.consensus.smartbft.signer").With(channelDecorator),
			// SignerSerializer: c.SignerSerializer,
			// LastConfigBlockNum: func(block *common.Block) uint64 {
			// 	if isConfigBlock(block) {
			// 		return block.Header.Number
			// 	}

			// 	return c.RuntimeConfig.Load().(RuntimeConfig).LastConfigBlock.Header.Number
			// },
		},
		Metadata:          *latestMetadata,
		WAL:               consensusWAL,
		WALInitialContent: walInitState, // Read from WAL entries
		Application:       c,
		Assembler:         c.assembler,
		RequestInspector:  requestInspector,
		Synchronizer:      sync,
		Comm: &Egress{
			RuntimeConfig: c.RuntimeConfig,
			Channel:       c.support.ChannelID(),
			Logger:        flogging.MustGetLogger("orderer.consensus.smartbft.egress").With(channelDecorator),
			RPC: &cluster.RPC{
				Logger:        flogging.MustGetLogger("orderer.consensus.smartbft.rpc").With(channelDecorator),
				Channel:       c.support.ChannelID(),
				StreamsByType: cluster.NewStreamsByType(),
				Comm:          c.Comm,
				Timeout:       5 * time.Minute, // Externalize configuration
			},
		},
		Scheduler:         time.NewTicker(time.Second).C,
		ViewChangerTicker: time.NewTicker(time.Second).C,
	}

	proposal, signatures := c.lastPersistedProposalAndSignatures()
	if proposal != nil {
		consensus.LastProposal = *proposal
		consensus.LastSignatures = signatures
	}

	c.reportIsLeader(proposal) // report the leader

	return consensus
}

func (B BFTChain) Order(env *cb.Envelope, configSeq uint64) error {
	panic("implement me")
}

func (B BFTChain) Configure(config *cb.Envelope, configSeq uint64) error {
	panic("implement me")
}

func (c *BFTChain) Deliver(proposal types.Proposal, signatures []types.Signature) types.Reconfig {
	panic("implement me")
}

func (B BFTChain) WaitReady() error {
	panic("implement me")
}

func (B BFTChain) Errored() <-chan struct{} {
	panic("implement me")
}

func (B BFTChain) Start() {
	panic("implement me")
}

func (B BFTChain) Halt() {
	panic("implement me")
}

func (c *BFTChain) blockToDecision(block *cb.Block) *types.Decision {
	panic("implement me")
}

func (c *BFTChain) updateRuntimeConfig(block *cb.Block) types.Reconfig {
	panic("implement me")
}

func (c *BFTChain) lastPersistedProposalAndSignatures() (*types.Proposal, []types.Signature) {
	panic("implement me")
}

func (c *BFTChain) reportIsLeader(proposal *types.Proposal) {
	panic("implement me")
}

