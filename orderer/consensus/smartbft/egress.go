/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package smartbft

import (
	"sync/atomic"

	protos "github.com/SmartBFT-Go/consensus/smartbftprotos"
	"github.com/hyperledger/fabric-protos-go/orderer"
)

type Logger interface {
	Warnf(template string, args ...interface{})
	Panicf(template string, args ...interface{})
}

type RPC interface {
	SendConsensus(dest uint64, msg *orderer.ConsensusRequest) error
	SendSubmit(dest uint64, request *orderer.SubmitRequest) error
}

type Egress struct {
	Channel       string
	RPC           RPC
	Logger        Logger
	RuntimeConfig *atomic.Value
}

func (e *Egress) Nodes() []uint64 {
	panic("implement me")
}

func (e *Egress) SendConsensus(targetID uint64, m *protos.Message) {
	panic("implement me")
}

func (e *Egress) SendTransaction(targetID uint64, request []byte) {
	panic("implement me")
}
