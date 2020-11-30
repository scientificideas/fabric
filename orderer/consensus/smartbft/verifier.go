/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package smartbft

import (
	"sync/atomic"

	"github.com/SmartBFT-Go/consensus/pkg/types"
	"github.com/hyperledger/fabric/common/flogging"
)

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


type NodeIdentitiesByID map[uint64][]byte

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