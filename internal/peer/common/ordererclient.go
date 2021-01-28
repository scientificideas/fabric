/*
Copyright IBM Corp. 2016-2017 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/
package common

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	ab "github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric/internal/pkg/comm"
	cb "github.com/hyperledger/fabric-protos-go/common"
	"github.com/pkg/errors"
)

// OrdererClient represents a client for communicating with an ordering
// service
type OrdererClient struct {
	CommonClient
}

// NewOrdererClientFromEnv creates an instance of an OrdererClient from the
// global Viper instance
func NewOrdererClientFromEnv() (*OrdererClient, error) {
	address, override, clientConfig, err := configFromEnv("orderer")
	if err != nil {
		return nil, errors.WithMessage(err, "failed to load config for OrdererClient")
	}
	gClient, err := comm.NewGRPCClient(clientConfig)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create OrdererClient from config")
	}
	oClient := &OrdererClient{
		CommonClient: CommonClient{
			GRPCClient: gClient,
			Address:    address,
			sn:         override,
		},
	}
	return oClient, nil
}

type BroadcastOrdererClient interface {
    CloseSend() error
    Send(envelope *cb.Envelope) error
    Recv() (*ab.BroadcastResponse, error)
}

// Broadcast returns a broadcast client for the AtomicBroadcast service
func (oc *OrdererClient) Broadcast() (BroadcastOrdererClient, error) {
	return newMulticastBroadcastClient(oc.Address, oc.CommonClient.NewConnection, oc.sn)
}

// Deliver returns a deliver client for the AtomicBroadcast service
func (oc *OrdererClient) Deliver() (ab.AtomicBroadcast_DeliverClient, error) {
	conn, err := oc.CommonClient.NewConnection(oc.Address, comm.ServerNameOverride(oc.sn))
	if err != nil {
		return nil, errors.WithMessagef(err, "orderer client failed to connect to %s", oc.Address)
	}
	// TODO: check to see if we should actually handle error before returning
	return ab.NewAtomicBroadcastClient(conn).Deliver(context.TODO())

}

// Certificate returns the TLS client certificate (if available)
func (oc *OrdererClient) Certificate() tls.Certificate {
	return oc.CommonClient.Certificate()
}

type multicastBroadcastClient struct {
	clients []ab.AtomicBroadcast_BroadcastClient
}

type dialer func(address string, tlsOptions ...comm.TLSOption) (*grpc.ClientConn, error)

func newMulticastBroadcastClient(addressString string, dial dialer, serverName string) (*multicastBroadcastClient, error) {
	addresses := []string{addressString}
	if strings.Contains(addressString, ",") {
		addresses = strings.Split(addressString, ",")
	}

	mbc := &multicastBroadcastClient{}

	for _, addr := range addresses {
		conn, err := dial(addr, comm.ServerNameOverride(serverName))
		if err != nil {
			return nil, errors.WithMessage(err, fmt.Sprintf("orderer client failed to connect to %s", addr))
		}
		cl, err := ab.NewAtomicBroadcastClient(conn).Broadcast(context.TODO())
		if err != nil {
			return nil, err
		}
		mbc.clients = append(mbc.clients, cl)
	}
	return mbc, nil
}

func (m *multicastBroadcastClient) CloseSend() error {
	for _, cl := range m.clients {
		err := cl.CloseSend()
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *multicastBroadcastClient) Send(envelope *cb.Envelope) error {
	for _, cl := range m.clients {
		err := cl.Send(envelope)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *multicastBroadcastClient) Recv() (*ab.BroadcastResponse, error) {
	var resp *ab.BroadcastResponse
	var err error
	for _, cl := range m.clients {
		resp, err = cl.Recv()
		if err != nil {
			return nil, err
		}
	}
	return resp, nil
}
