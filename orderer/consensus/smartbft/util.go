/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package smartbft

import (
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"sort"
	"time"

	"github.com/SmartBFT-Go/consensus/pkg/types"
	"github.com/SmartBFT-Go/consensus/smartbftprotos"
	"github.com/golang/protobuf/proto"
	cb "github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/msp"
	"github.com/hyperledger/fabric-protos-go/orderer/smartbft"
	"github.com/hyperledger/fabric/bccsp"
	"github.com/hyperledger/fabric/common/channelconfig"
	"github.com/hyperledger/fabric/common/crypto"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/orderer/common/cluster"
	"github.com/hyperledger/fabric/orderer/common/localconfig"
	"github.com/hyperledger/fabric/orderer/consensus"
	"github.com/hyperledger/fabric/orderer/consensus/etcdraft"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
)

// RuntimeConfig defines the configuration of the consensus
// that is related to runtime.
type RuntimeConfig struct {
	BFTConfig              types.Configuration
	isConfig               bool
	logger                 *flogging.FabricLogger
	id                     uint64
	LastCommittedBlockHash string
	RemoteNodes            []cluster.RemoteNode
	ID2Identities          NodeIdentitiesByID
	LastBlock              *cb.Block
	LastConfigBlock        *cb.Block
	Nodes                  []uint64
}

func (rtc RuntimeConfig) BlockCommitted(block *cb.Block, bccsp bccsp.BCCSP) (RuntimeConfig, error) {
	if _, err := cluster.ConfigFromBlock(block); err == nil {
		return rtc.configBlockCommitted(block, bccsp)
	}
	return RuntimeConfig{
		BFTConfig:              rtc.BFTConfig,
		id:                     rtc.id,
		logger:                 rtc.logger,
		LastCommittedBlockHash: hex.EncodeToString(block.Header.GetDataHash()),
		Nodes:                  rtc.Nodes,
		ID2Identities:          rtc.ID2Identities,
		RemoteNodes:            rtc.RemoteNodes,
		LastBlock:              block,
		LastConfigBlock:        rtc.LastConfigBlock,
	}, nil
}

func (rtc RuntimeConfig) configBlockCommitted(block *cb.Block, bccsp bccsp.BCCSP) (RuntimeConfig, error) {
	nodeConf, err := RemoteNodesFromConfigBlock(block, rtc.id, rtc.logger, bccsp)
	if err != nil {
		return rtc, errors.Wrap(err, "remote nodes cannot be computed, rejecting config block")
	}

	bftConfig, err := configBlockToBFTConfig(rtc.id, block, bccsp)
	if err != nil {
		return RuntimeConfig{}, err
	}

	return RuntimeConfig{
		BFTConfig:              bftConfig,
		isConfig:               true,
		id:                     rtc.id,
		logger:                 rtc.logger,
		LastCommittedBlockHash: hex.EncodeToString(block.Header.GetDataHash()),
		Nodes:                  nodeConf.nodeIDs,
		ID2Identities:          nodeConf.id2Identities,
		RemoteNodes:            nodeConf.remoteNodes,
		LastBlock:              block,
		LastConfigBlock:        block,
	}, nil
}

func configBlockToBFTConfig(selfID uint64, block *cb.Block, bccsp bccsp.BCCSP) (types.Configuration, error) {
	if block == nil || block.Data == nil || len(block.Data.Data) == 0 {
		return types.Configuration{}, errors.New("empty block")
	}

	env, err := protoutil.UnmarshalEnvelope(block.Data.Data[0])
	if err != nil {
		return types.Configuration{}, err
	}
	bundle, err := channelconfig.NewBundleFromEnvelope(env, bccsp)
	if err != nil {
		return types.Configuration{}, err
	}

	oc, ok := bundle.OrdererConfig()
	if !ok {
		return types.Configuration{}, errors.New("no orderer config")
	}

	consensusMD := &smartbft.ConfigMetadata{}
	if err := proto.Unmarshal(oc.ConsensusMetadata(), consensusMD); err != nil {
		return types.Configuration{}, err
	}

	return configFromMetadataOptions(selfID, consensusMD.Options)
}

func isConfigBlock(block *cb.Block) bool {
	if block.Data == nil || len(block.Data.Data) != 1 {
		return false
	}
	env, err := protoutil.UnmarshalEnvelope(block.Data.Data[0])
	if err != nil {
		return false
	}
	payload, err := protoutil.GetPayload(env)
	if err != nil {
		return false
	}

	if payload.Header == nil {
		return false
	}

	hdr, err := protoutil.UnmarshalChannelHeader(payload.Header.ChannelHeader)
	if err != nil {
		return false
	}
	return cb.HeaderType(hdr.Type) == cb.HeaderType_CONFIG
}

//go:generate counterfeiter -o mocks/mock_blockpuller.go . BlockPuller

// newBlockPuller creates a new block puller
func newBlockPuller(
	support consensus.ConsenterSupport,
	baseDialer *cluster.PredicateDialer,
	clusterConfig localconfig.Cluster,
	bccsp bccsp.BCCSP) (BlockPuller, error) {

	verifyBlockSequence := func(blocks []*cb.Block, _ string) error {
		return cluster.VerifyBlocks(blocks, support)
	}

	stdDialer := &cluster.StandardDialer{
		Config: baseDialer.Config.Clone(),
	}
	stdDialer.Config.AsyncConnect = false
	stdDialer.Config.SecOpts.VerifyCertificate = nil

	// Extract the TLS CA certs and endpoints from the configuration,
	endpoints, err := etcdraft.EndpointconfigFromSupport(support, bccsp)
	if err != nil {
		return nil, err
	}

	der, _ := pem.Decode(stdDialer.Config.SecOpts.Certificate)
	if der == nil {
		return nil, errors.Errorf("client certificate isn't in PEM format: %v",
			string(stdDialer.Config.SecOpts.Certificate))
	}

	bp := &cluster.BlockPuller{
		VerifyBlockSequence: verifyBlockSequence,
		Logger:              flogging.MustGetLogger("orderer.common.cluster.puller"),
		RetryTimeout:        clusterConfig.ReplicationRetryTimeout,
		MaxTotalBufferBytes: clusterConfig.ReplicationBufferSize,
		FetchTimeout:        clusterConfig.ReplicationPullTimeout,
		Endpoints:           endpoints,
		Signer:              support,
		TLSCert:             der.Bytes,
		Channel:             support.ChannelID(),
		Dialer:              stdDialer,
	}

	return bp, nil
}

func getViewMetadataFromBlock(block *cb.Block) (*smartbftprotos.ViewMetadata, error) {
	if block.Header.Number == 0 {
		// Genesis block has no prior metadata so we just return an un-initialized metadata
		return nil, nil
	}

	signatureMetadata := protoutil.GetMetadataFromBlockOrPanic(block, cb.BlockMetadataIndex_SIGNATURES)
	ordererMD := &cb.OrdererBlockMetadata{}
	if err := proto.Unmarshal(signatureMetadata.Value, ordererMD); err != nil {
		return nil, errors.Wrap(err, "failed unmarshaling OrdererBlockMetadata")
	}

	var viewMetadata smartbftprotos.ViewMetadata
	if err := proto.Unmarshal(ordererMD.ConsenterMetadata, &viewMetadata); err != nil {
		return nil, err
	}

	return &viewMetadata, nil
}

func configFromMetadataOptions(selfID uint64, options *smartbft.Options) (types.Configuration, error) {
	var err error

	config := types.DefaultConfig
	config.SelfID = selfID

	if options == nil {
		return config, errors.New("config metadata options field is nil")
	}

	config.RequestBatchMaxCount = options.RequestBatchMaxCount
	config.RequestBatchMaxBytes = options.RequestBatchMaxBytes
	if config.RequestBatchMaxInterval, err = time.ParseDuration(options.RequestBatchMaxInterval); err != nil {
		return config, errors.Wrap(err, "bad config metadata option RequestBatchMaxInterval")
	}
	config.IncomingMessageBufferSize = options.IncomingMessageBufferSize
	config.RequestPoolSize = options.RequestPoolSize
	if config.RequestForwardTimeout, err = time.ParseDuration(options.RequestForwardTimeout); err != nil {
		return config, errors.Wrap(err, "bad config metadata option RequestForwardTimeout")
	}
	if config.RequestComplainTimeout, err = time.ParseDuration(options.RequestComplainTimeout); err != nil {
		return config, errors.Wrap(err, "bad config metadata option RequestComplainTimeout")
	}
	if config.RequestAutoRemoveTimeout, err = time.ParseDuration(options.RequestAutoRemoveTimeout); err != nil {
		return config, errors.Wrap(err, "bad config metadata option RequestAutoRemoveTimeout")
	}
	if config.ViewChangeResendInterval, err = time.ParseDuration(options.ViewChangeResendInterval); err != nil {
		return config, errors.Wrap(err, "bad config metadata option ViewChangeResendInterval")
	}
	if config.ViewChangeTimeout, err = time.ParseDuration(options.ViewChangeTimeout); err != nil {
		return config, errors.Wrap(err, "bad config metadata option ViewChangeTimeout")
	}
	if config.LeaderHeartbeatTimeout, err = time.ParseDuration(options.LeaderHeartbeatTimeout); err != nil {
		return config, errors.Wrap(err, "bad config metadata option LeaderHeartbeatTimeout")
	}
	config.LeaderHeartbeatCount = options.LeaderHeartbeatCount
	if config.CollectTimeout, err = time.ParseDuration(options.CollectTimeout); err != nil {
		return config, errors.Wrap(err, "bad config metadata option CollectTimeout")
	}
	config.SyncOnStart = options.SyncOnStart
	config.SpeedUpViewChange = options.SpeedUpViewChange

	// todo: BFT leader rotation
	//if options.DecisionsPerLeader == 0 {
	//	config.DecisionsPerLeader = 1
	//}
	//
	//// Enable rotation by default, but optionally disable it
	//switch options.LeaderRotation {
	//case smartbft.Options_OFF:
	//	config.LeaderRotation = false
	//	config.DecisionsPerLeader = 0
	//default:
	//	config.LeaderRotation = true
	//}

	if err = config.Validate(); err != nil {
		return config, errors.Wrap(err, "config validation failed")
	}

	return config, nil
}

// RequestInspector inspects incomming requests and validates serialized identity
type RequestInspector struct {
	ValidateIdentityStructure func(identity *msp.SerializedIdentity) error
}

func RemoteNodesFromConfigBlock(block *cb.Block, selfID uint64, logger *flogging.FabricLogger, bccsp bccsp.BCCSP) (*nodeConfig, error) {
	env := &cb.Envelope{}
	if err := proto.Unmarshal(block.Data.Data[0], env); err != nil {
		return nil, errors.Wrap(err, "failed unmarshaling envelope of config block")
	}
	bundle, err := channelconfig.NewBundleFromEnvelope(env, bccsp)
	if err != nil {
		return nil, errors.Wrap(err, "failed getting a new bundle from envelope of config block")
	}

	oc, ok := bundle.OrdererConfig()
	if !ok {
		return nil, errors.New("no orderer config in config block")
	}

	m := &smartbft.ConfigMetadata{}
	if err := proto.Unmarshal(oc.ConsensusMetadata(), m); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal consensus metadata")
	}
	if m.Options == nil {
		return nil, errors.New("failed to retrieve consensus metadata options")
	}

	var nodeIDs []uint64
	var remoteNodes []cluster.RemoteNode
	id2Identies := map[uint64][]byte{}
	for _, consenter := range m.Consenters {
		sanitizedID, err := crypto.SanitizeIdentity(consenter.Identity)
		if err != nil {
			logger.Panicf("Failed to sanitize identity: %v", err)
		}
		id2Identies[consenter.ConsenterId] = sanitizedID
		logger.Infof("%s %d ---> %s", bundle.ConfigtxValidator().ChannelID(), consenter.ConsenterId, string(consenter.Identity))

		nodeIDs = append(nodeIDs, consenter.ConsenterId)

		// No need to know yourself
		if selfID == consenter.ConsenterId {
			continue
		}
		serverCertAsDER, err := pemToDER(consenter.ServerTlsCert, consenter.ConsenterId, "server", logger)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		clientCertAsDER, err := pemToDER(consenter.ClientTlsCert, consenter.ConsenterId, "client", logger)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		// Validate certificate structure
		for _, cert := range [][]byte{serverCertAsDER, clientCertAsDER} {
			if _, err := x509.ParseCertificate(cert); err != nil {
				pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})
				logger.Errorf("Invalid certificate: %s", string(pemBytes))
				return nil, err
			}
		}

		remoteNodes = append(remoteNodes, cluster.RemoteNode{
			ID:            consenter.ConsenterId,
			ClientTLSCert: clientCertAsDER,
			ServerTLSCert: serverCertAsDER,
			Endpoint:      fmt.Sprintf("%s:%d", consenter.Host, consenter.Port),
		})
	}

	sort.Slice(nodeIDs, func(i, j int) bool {
		return nodeIDs[i] < nodeIDs[j]
	})

	return &nodeConfig{
		remoteNodes:   remoteNodes,
		id2Identities: id2Identies,
		nodeIDs:       nodeIDs,
	}, nil

}

type nodeConfig struct {
	id2Identities NodeIdentitiesByID
	remoteNodes   []cluster.RemoteNode
	nodeIDs       []uint64
}
