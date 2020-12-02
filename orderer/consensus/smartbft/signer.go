/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package smartbft

import (
	"github.com/hyperledger/fabric/common/crypto"
	"github.com/SmartBFT-Go/consensus/pkg/types"
)

//go:generate mockery -dir . -name SignerSerializer -case underscore -output ./mocks/

type SignerSerializer interface {
	crypto.Signer
	crypto.IdentitySerializer
}

type Signer struct {
}

func (*Signer) Sign([]byte) []byte {
	panic("implement me")
}

func (*Signer) SignProposal(types.Proposal) *types.Signature {
	panic("implement me")
}
