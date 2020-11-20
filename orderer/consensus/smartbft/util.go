/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package smartbft

import (
	"github.com/SmartBFT-Go/consensus/pkg/types"
	"github.com/SmartBFT-Go/consensus/smartbftprotos"
	"github.com/golang/protobuf/proto"
	cb "github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/orderer/common/cluster"
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
