/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package smartbft

import (
	"errors"

	protos "github.com/SmartBFT-Go/consensus/smartbftprotos"
	ab "github.com/hyperledger/fabric-protos-go/orderer"
)

//go:generate mockery -dir . -name MessageReceiver -case underscore -output mocks

// MessageReceiver receives messages
type MessageReceiver interface {
	HandleMessage(sender uint64, m *protos.Message)
	HandleRequest(sender uint64, req []byte)
}

//go:generate mockery -dir . -name ReceiverGetter -case underscore -output mocks

// ReceiverGetter obtains instances of MessageReceiver given a channel ID
type ReceiverGetter interface {
	// ReceiverByChain returns the MessageReceiver if it exists, or nil if it doesn't
	ReceiverByChain(channelID string) MessageReceiver
}

type WarningLogger interface {
	Warningf(template string, args ...interface{})
}

// Ingreess dispatches Submit and Step requests to the designated per chain instances
type Ingreess struct {
	Logger        WarningLogger
	ChainSelector ReceiverGetter
}

// OnConsensus notifies the Ingreess for a reception of a StepRequest from a given sender on a given channel
func (in *Ingreess) OnConsensus(channel string, sender uint64, request *ab.ConsensusRequest) error {
	return errors.New("not implemented")
}

// OnSubmit notifies the Ingreess for a reception of a SubmitRequest from a given sender on a given channel
func (in *Ingreess) OnSubmit(channel string, sender uint64, request *ab.SubmitRequest) error {
	return errors.New("not implemented")
}
