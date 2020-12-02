/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package smartbft

import (
	"encoding/hex"
	"sync/atomic"

	"github.com/SmartBFT-Go/consensus/pkg/types"
	cb "github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
)

type NodeIdentitiesByID map[uint64][]byte

func verifyHashChain(block *cb.Block, prevHeaderHash string) error {
	thisHdrHashOfPrevHdr := hex.EncodeToString(block.Header.PreviousHash)
	if prevHeaderHash != thisHdrHashOfPrevHdr {
		return errors.Errorf("previous header hash is %s but expected %s", thisHdrHashOfPrevHdr, prevHeaderHash)
	}

	dataHash := hex.EncodeToString(block.Header.DataHash)
	actualHashOfData := hex.EncodeToString(protoutil.BlockDataHash(block.Data))
	if dataHash != actualHashOfData {
		return errors.Errorf("data hash is %s but expected %s", dataHash, actualHashOfData)
	}
	return nil
}

//go:generate mockery -dir . -name Sequencer -case underscore -output mocks

// Sequencer returns sequences
type Sequencer interface {
	Sequence() uint64
}

//go:generate mockery -dir . -name ConsenterVerifier -case underscore -output mocks

// ConsenterVerifier is used to determine whether a signature from one of the consenters is valid
type ConsenterVerifier interface {
	// Evaluate takes a set of SignedData and evaluates whether this set of signatures satisfies the policy
	// todo: BFT verifier
	//Evaluate(signatureSet []*common.SignedData) error
}

//go:generate mockery -dir . -name AccessController -case underscore -output mocks

// AccessController is used to determine if a signature of a certain client is valid
type AccessController interface {
	// Evaluate takes a set of SignedData and evaluates whether this set of signatures satisfies the policy
	// todo: BFT verifier
	//Evaluate(signatureSet []*common.SignedData) error
}

type requestVerifier func(req []byte, isolated bool) (types.RequestInfo, error)

// Verifier is used to determine whether a signature from one of the consenters is valid
type Verifier struct {
	RuntimeConfig         *atomic.Value
	ReqInspector          *RequestInspector
	ConsenterVerifier     ConsenterVerifier
	AccessController      AccessController
	VerificationSequencer Sequencer
	Ledger                Ledger
	Logger                *flogging.FabricLogger
	ConfigValidator       ConfigValidator
}

func (*Verifier) RequestsFromProposal(types.Proposal) []types.RequestInfo {
	panic("implement me")
}

func (*Verifier) VerifyProposal(proposal types.Proposal) ([]types.RequestInfo, error) {
	panic("implement me")
}

func (*Verifier) VerifyRequest(val []byte) (types.RequestInfo, error) {
	panic("implement me")
}

func (*Verifier) VerifyConsenterSig(signature types.Signature, prop types.Proposal) error {
	panic("implement me")
}

func (*Verifier) VerifySignature(signature types.Signature) error {
	panic("implement me")
}

func (*Verifier) VerificationSequence() uint64 {
	panic("implement me")
}
