/*
 *
 * Copyright IBM Corp. All Rights Reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 * /
 *
 */

package smartbft

import (
	"errors"

	"github.com/golang/protobuf/proto"
	cb "github.com/hyperledger/fabric-protos-go/common"
	ab "github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/common/metrics"
	"github.com/hyperledger/fabric/common/policies"
	"github.com/hyperledger/fabric/internal/pkg/comm"
	"github.com/hyperledger/fabric/orderer/common/cluster"
	"github.com/hyperledger/fabric/orderer/common/localconfig"
	"github.com/hyperledger/fabric/orderer/common/multichannel"
	"github.com/hyperledger/fabric/orderer/consensus"
	"github.com/mitchellh/mapstructure"
)

// CreateChainCallback creates a new chain
type CreateChainCallback func()

// ChainGetter obtains instances of ChainSupport for the given channel
type ChainGetter interface {
	// GetChain obtains the ChainSupport for the given channel.
	// Returns nil, false when the ChainSupport for the given channel
	// isn't found.
	GetChain(chainID string) *multichannel.ChainSupport
}

type PolicyManagerRetriever func(channel string) policies.Manager

//go:generate mockery -dir . -name InactiveChainRegistry -case underscore -output mocks

// InactiveChainRegistry registers chains that are inactive
type InactiveChainRegistry interface {
	// TrackChain tracks a chain with the given name, and calls the given callback
	// when this chain should be created.
	TrackChain(chainName string, genesisBlock *cb.Block, createChain func())
	// Stop stops the InactiveChainRegistry. This is used when removing the
	// system channel.
	Stop()
}

// Consenter implementation of the BFT smart based consenter
type Consenter struct {
	CreateChain           func(chainName string)
	InactiveChainRegistry InactiveChainRegistry
	GetPolicyManager      PolicyManagerRetriever
	Logger                *flogging.FabricLogger
	Cert                  []byte
	Comm                  *cluster.Comm
	Chains                ChainGetter
	SignerSerializer      SignerSerializer
	Registrar             *multichannel.Registrar
	WALBaseDir            string
	ClusterDialer         *cluster.PredicateDialer
	Conf                  *localconfig.TopLevel
	//Metrics               *Metrics
}

// New creates Consenter of type smart bft
func New(
	icr InactiveChainRegistry,
	pmr PolicyManagerRetriever,
	signerSerializer SignerSerializer,
	clusterDialer *cluster.PredicateDialer,
	conf *localconfig.TopLevel,
	srvConf comm.ServerConfig,
	srv *comm.GRPCServer,
	r *multichannel.Registrar,
	metricsProvider metrics.Provider,
) *Consenter {
	logger := flogging.MustGetLogger("orderer.consensus.smartbft")

	metrics := cluster.NewMetrics(metricsProvider)

	var walConfig WALConfig
	err := mapstructure.Decode(conf.Consensus, &walConfig)
	if err != nil {
		logger.Panicf("Failed to decode consensus configuration: %s", err)
	}

	logger.Infof("WAL Directory is %s", walConfig.WALDir)

	consenter := &Consenter{
		InactiveChainRegistry: icr,
		Registrar:             r,
		GetPolicyManager:      pmr,
		Conf:                  conf,
		ClusterDialer:         clusterDialer,
		Logger:                logger,
		Cert:                  srvConf.SecOpts.Certificate,
		Chains:                r,
		SignerSerializer:      signerSerializer,
		WALBaseDir:            walConfig.WALDir,
		//Metrics:               NewMetrics(metricsProvider),
		CreateChain: r.CreateChain,
	}

	consenter.Comm = &cluster.Comm{
		MinimumExpirationWarningInterval: cluster.MinimumExpirationWarningInterval,
		CertExpWarningThreshold:          conf.General.Cluster.CertExpirationWarningThreshold,
		SendBufferSize:                   conf.General.Cluster.SendBufferSize,
		Logger:                           flogging.MustGetLogger("orderer.common.cluster"),
		Chan2Members:                     make(map[string]cluster.MemberMapping),
		Connections:                      cluster.NewConnectionStore(clusterDialer, metrics.EgressTLSConnectionCount),
		Metrics:                          metrics,
		ChanExt:                          consenter,
		H: &Ingreess{
			Logger:        logger,
			ChainSelector: consenter,
		},
	}

	svc := &cluster.Service{
		CertExpWarningThreshold:          conf.General.Cluster.CertExpirationWarningThreshold,
		MinimumExpirationWarningInterval: cluster.MinimumExpirationWarningInterval,
		StreamCountReporter: &cluster.StreamCountReporter{
			Metrics: metrics,
		},
		StepLogger: flogging.MustGetLogger("orderer.common.cluster.step"),
		Logger:     flogging.MustGetLogger("orderer.common.cluster"),
		Dispatcher: consenter.Comm,
	}

	ab.RegisterClusterServer(srv.Server(), svc)

	return consenter
}

// ReceiverByChain returns the MessageReceiver for the given channelID or nil
// if not found.
func (c *Consenter) ReceiverByChain(channelID string) MessageReceiver {
	return nil
}

func (c *Consenter) HandleChain(support consensus.ConsenterSupport, metadata *cb.Metadata) (consensus.Chain, error) {
	//TODO fully construct a follower.Chain
	return nil, errors.New("not implemented")
}

func (c *Consenter) JoinChain(support consensus.ConsenterSupport, joinBlock *cb.Block) (consensus.Chain, error) {
	//TODO fully construct a follower.Chain
	return nil, errors.New("not implemented")
}

// TargetChannel extracts the channel from the given proto.Message.
// Returns an empty string on failure.
func (c *Consenter) TargetChannel(message proto.Message) string {
	switch req := message.(type) {
	case *ab.ConsensusRequest:
		return req.Channel
	case *ab.SubmitRequest:
		return req.Channel
	default:
		return ""
	}
}
