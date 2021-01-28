/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package protoutil

import (
	"bytes"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric/common/util"
	"github.com/pkg/errors"
)

// SignedData is used to represent the general triplet required to verify a signature
// This is intended to be generic across crypto schemes, while most crypto schemes will
// include the signing identity and a nonce within the Data, this is left to the crypto
// implementation.
type SignedData struct {
	Data      []byte
	Identity  []byte
	Signature []byte
}

// ConfigUpdateEnvelopeAsSignedData returns the set of signatures for the
// ConfigUpdateEnvelope as SignedData or an error indicating why this was not
// possible.
func ConfigUpdateEnvelopeAsSignedData(ce *common.ConfigUpdateEnvelope) ([]*SignedData, error) {
	if ce == nil {
		return nil, fmt.Errorf("No signatures for nil SignedConfigItem")
	}

	result := make([]*SignedData, len(ce.Signatures))
	for i, configSig := range ce.Signatures {
		sigHeader := &common.SignatureHeader{}
		err := proto.Unmarshal(configSig.SignatureHeader, sigHeader)
		if err != nil {
			return nil, err
		}

		result[i] = &SignedData{
			Data:      bytes.Join([][]byte{configSig.SignatureHeader, ce.ConfigUpdate}, nil),
			Identity:  sigHeader.Creator,
			Signature: configSig.Signature,
		}

	}

	return result, nil
}

// EnvelopeAsSignedData returns the signatures for the Envelope as SignedData
// slice of length 1 or an error indicating why this was not possible.
func EnvelopeAsSignedData(env *common.Envelope) ([]*SignedData, error) {
	if env == nil {
		return nil, fmt.Errorf("No signatures for nil Envelope")
	}

	payload := &common.Payload{}
	err := proto.Unmarshal(env.Payload, payload)
	if err != nil {
		return nil, err
	}

	if payload.Header == nil /* || payload.Header.SignatureHeader == nil */ {
		return nil, fmt.Errorf("Missing Header")
	}

	shdr := &common.SignatureHeader{}
	err = proto.Unmarshal(payload.Header.SignatureHeader, shdr)
	if err != nil {
		return nil, fmt.Errorf("GetSignatureHeaderFromBytes failed, err %s", err)
	}

	return []*SignedData{{
		Data:      env.Payload,
		Identity:  shdr.Creator,
		Signature: env.Signature,
	}}, nil
}

// SignatureSetFromBlock creates a signature set out of a block.
func SignatureSetFromBlock(block *common.Block, id2identities map[uint64][]byte) ([]*SignedData, error) {
	if block.Metadata == nil || len(block.Metadata.Metadata) <= int(common.BlockMetadataIndex_SIGNATURES) {
		return nil, errors.New("no metadata in block")
	}
	metadata, err := GetMetadataFromBlock(block, common.BlockMetadataIndex_SIGNATURES)
	if err != nil {
		return nil, errors.Errorf("failed unmarshaling medatata for signatures: %v", err)
	}

	var signatureSet []*SignedData
	for _, metadataSignature := range metadata.Signatures {
		identity := id2identities[metadataSignature.SignerId]
		if len(metadataSignature.SignatureHeader) > 0 {
			sigHdr, err := UnmarshalSignatureHeader(metadataSignature.SignatureHeader)
			if err != nil {
				return nil, errors.Errorf("failed unmarshaling signature header for block with id %d: %v",
					block.Header.Number, err)
			}
			identity = sigHdr.Creator
		} else {
			metadataSignature.SignatureHeader = MarshalOrPanic(&common.SignatureHeader{
				Creator: identity,
				Nonce:   metadataSignature.Nonce,
			})
		}

		signatureSet = append(signatureSet,
			&SignedData{
				Identity: identity,
				Data: util.ConcatenateBytes(metadata.Value,
					metadataSignature.SignatureHeader, BlockHeaderBytes(block.Header), metadataSignature.AuxiliaryInput),
				Signature: metadataSignature.Signature,
			},
		)
	}
	return signatureSet, nil
}
