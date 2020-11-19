/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package smartbft

import (
	"github.com/SmartBFT-Go/consensus/smartbftprotos"
	"github.com/golang/protobuf/proto"
	cb "github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
)

//go:generate counterfeiter -o mocks/mock_blockpuller.go . BlockPuller

func getViewMetadataFromBlock(block *cb.Block) (smartbftprotos.ViewMetadata, error) {
	if block.Header.Number == 0 {
		// Genesis block has no prior metadata so we just return an un-initialized metadata
		return smartbftprotos.ViewMetadata{}, nil
	}

	signatureMetadata := protoutil.GetMetadataFromBlockOrPanic(block, cb.BlockMetadataIndex_SIGNATURES)
	ordererMD := &cb.OrdererBlockMetadata{}
	if err := proto.Unmarshal(signatureMetadata.Value, ordererMD); err != nil {
		return smartbftprotos.ViewMetadata{}, errors.Wrap(err, "failed unmarshaling OrdererBlockMetadata")
	}

	var viewMetadata smartbftprotos.ViewMetadata
	if err := proto.Unmarshal(ordererMD.ConsenterMetadata, &viewMetadata); err != nil {
		return smartbftprotos.ViewMetadata{}, err
	}

	return viewMetadata, nil
}
