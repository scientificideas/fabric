/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package deliverservice

import (
	"context"
	"crypto/x509"
	"math/rand"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/deliverservice/mocks"
	"github.com/hyperledger/fabric/internal/pkg/comm"
	"github.com/hyperledger/fabric/internal/pkg/peer/blocksprovider/fake"
	"github.com/hyperledger/fabric/internal/pkg/peer/orderers"
	mocks2 "github.com/hyperledger/fabric/orderer/consensus/smartbft/mocks"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

var endpoints = []*orderers.Endpoint{
	{Address: "localhost:5611"},
	{Address: "localhost:5612"},
	{Address: "localhost:5613"},
	{Address: "localhost:5614"},
}

var endpoints3 = []*orderers.Endpoint{
	{Address: "localhost:5615"},
	{Address: "localhost:5616"},
	{Address: "localhost:5617"},
}

var endpointMap = map[int]*orderers.Endpoint{
	5611: {Address: "localhost:5611"},
	5612: {Address: "localhost:5612"},
	5613: {Address: "localhost:5613"},
	5614: {Address: "localhost:5614"},
}

var endpointMap3 = map[int]*orderers.Endpoint{
	5615: {Address: "localhost:5615"},
	5616: {Address: "localhost:5616"},
	5617: {Address: "localhost:5617"},
}

const goRoutineTestWaitTimeout = time.Second * 15

// Scenario: create an deliver adapter.
func TestNewBftDeliverAdapter(t *testing.T) {
	fakeLedgerInfo := &fake.LedgerInfo{}
	fakeBlockVerifier := &fake.BlockVerifier{}
	fakeOrdererConnectionSource := &fake.OrdererConnectionSource{}

	grpcClient, err := comm.NewGRPCClient(comm.ClientConfig{
		SecOpts: comm.SecureOptions{
			UseTLS: true,
		},
	})
	require.NoError(t, err)

	da := NewBftDeliverAdapter("test-chain", fakeLedgerInfo, fakeBlockVerifier, fakeOrdererConnectionSource, nil, &fake.Dialer{}, grpcClient)
	require.NotNil(t, da)
}

// Scenario: create an delivery client.
func TestNewBFTDeliveryClient(t *testing.T) {
	flogging.ActivateSpec("bftDeliveryClient=DEBUG")

	grpcClient, err := comm.NewGRPCClient(comm.ClientConfig{
		SecOpts: comm.SecureOptions{
			UseTLS: true,
		},
	})
	require.NoError(t, err)

	fakeLedgerInfo := &fake.LedgerInfo{}
	fakeBlockVerifier := &fake.BlockVerifier{}
	fakeOrdererConnectionSource := &fake.OrdererConnectionSource{}
	mockSignerSerializer := &mocks2.SignerSerializer{}
	fakeDialer := &fake.Dialer{}
	fakeDialer.DialStub = func(string, *x509.CertPool) (*grpc.ClientConn, error) {
		cc, err := grpc.Dial("", grpc.WithInsecure())
		require.Nil(t, err)
		require.NotEqual(t, cc.GetState(), connectivity.Shutdown)
		return cc, nil
	}

	conn, err := fakeDialer.Dial("", x509.NewCertPool())
	require.Nil(t, err)
	require.NotNil(t, conn)

	ctx, cancel := context.WithTimeout(context.Background(), getConnectionTimeout())
	defer cancel()
	deliverClient, err := orderer.NewAtomicBroadcastClient(conn).Deliver(ctx)

	bc, err := NewBFTDeliveryClient(ctx, deliverClient, "test-chain", fakeLedgerInfo, fakeBlockVerifier, fakeOrdererConnectionSource, mockSignerSerializer, fakeDialer, grpcClient)
	require.NotNil(t, bc)
	require.Nil(t, err)
}

// Scenario: create a client against a set of orderer mocks. Receive several blocks and check block & header reception.
func Test_bftDeliveryClient_Recv(t *testing.T) {
	flogging.ActivateSpec("bftDeliveryClient=DEBUG")

	osArray := make([]*mocks.Orderer, 0)
	for port := range endpointMap {
		osArray = append(osArray, mocks.NewOrderer(port, t))
	}
	for _, os := range osArray {
		os.SetNextExpectedSeek(5)
	}

	grpcClient, err := comm.NewGRPCClient(comm.ClientConfig{
		SecOpts: comm.SecureOptions{
			UseTLS: true,
		},
	})
	require.NoError(t, err)

	fakeLedgerInfo := &fake.LedgerInfo{}
	fakeLedgerInfo.LedgerHeightReturns(5, nil)
	fakeBlockVerifier := &fake.BlockVerifier{}
	fakeOrdererConnectionSource := &fake.OrdererConnectionSource{}
	fakeOrdererConnectionSource.RandomEndpointReturns(endpoints[rand.Intn(len(endpoints))], nil)
	fakeOrdererConnectionSource.GetAllEndpointsReturns(endpoints)
	mockSignerSerializer := &mocks2.SignerSerializer{}
	mockSignerSerializer.On("Sign", mock.Anything).Return([]byte{1, 2, 3}, nil)
	mockSignerSerializer.On("Serialize", mock.Anything).Return([]byte{0, 2, 4, 6}, nil)
	fakeDialer := &fake.Dialer{}
	fakeDialer.DialCalls(func(ep string, cp *x509.CertPool) (*grpc.ClientConn, error) {
		cc, err := grpc.Dial(ep, grpc.WithInsecure())
		require.Nil(t, err)
		require.NotEqual(t, cc.GetState(), connectivity.Shutdown)
		return cc, nil
	})

	conn, err := fakeDialer.Dial("", x509.NewCertPool())
	require.Nil(t, err)
	require.NotNil(t, conn)

	ctx, cancel := context.WithTimeout(context.Background(), getConnectionTimeout())
	defer cancel()
	deliverClient, err := orderer.NewAtomicBroadcastClient(conn).Deliver(ctx)

	bc, err := NewBFTDeliveryClient(ctx, deliverClient, "test-chain", fakeLedgerInfo, fakeBlockVerifier, fakeOrdererConnectionSource, mockSignerSerializer, fakeDialer, grpcClient)
	require.NotNil(t, bc)
	require.Nil(t, err)

	go func() {
		for {
			resp, err := bc.Recv()
			if err != nil {
				assert.EqualError(t, err, errClientClosing.Error())
				return
			}
			block := resp.GetBlock()
			assert.NotNil(t, block)
			if block == nil {
				return
			}
			bc.UpdateReceived(block.Header.Number)
		}
	}()

	// all orderers send something: block/header
	beforeSend := time.Now()
	for seq := uint64(5); seq < uint64(10); seq++ {
		for _, os := range osArray {
			os.SendBlock(seq)
		}
	}

	time.Sleep(time.Second)
	bc.Close()

	lastN, lastT := bc.GetNextBlockNumTime()
	assert.Equal(t, uint64(10), lastN)
	assert.True(t, lastT.After(beforeSend))

	headersNum, headerT, headerErr := bc.GetHeadersBlockNumTime()
	for i, n := range headersNum {
		assert.Equal(t, uint64(10), n)
		assert.True(t, headerT[i].After(beforeSend))
		assert.NoError(t, headerErr[i])
	}

	for _, os := range osArray {
		os.Shutdown()
	}
}

type mockOrderer struct {
	t *testing.T
}

func (*mockOrderer) Broadcast(orderer.AtomicBroadcast_BroadcastServer) error {
	panic("not implemented")
}

func (o *mockOrderer) Deliver(stream orderer.AtomicBroadcast_DeliverServer) error {
	envelope, _ := stream.Recv()
	inspectTLSBinding := comm.NewBindingInspector(true, func(msg proto.Message) []byte {
		env, isEnvelope := msg.(*common.Envelope)
		if !isEnvelope || env == nil {
			assert.Fail(o.t, "not an envelope")
		}
		ch, err := protoutil.ChannelHeader(env)
		assert.NoError(o.t, err)
		return ch.TlsCertHash
	})

	err := inspectTLSBinding(stream.Context(), envelope)
	assert.NoError(o.t, err, "orderer rejected TLS binding")

	payload, err := protoutil.UnmarshalPayload(envelope.Payload)
	assert.NoError(o.t, err, "cannot unmarshal payload")
	seekInfo := &orderer.SeekInfo{}
	err = proto.Unmarshal(payload.Data, seekInfo)
	assert.NoError(o.t, err, "cannot unmarshal payload.Data")

	block := &common.Block{
		Header: &common.BlockHeader{
			Number: 100,
		},
		Data: &common.BlockData{
			Data: [][]byte{{1, 2}, {3, 4}},
		},
		Metadata: &common.BlockMetadata{
			Metadata: [][]byte{{5, 6}, {7, 8}},
		},
	}

	if oldest := seekInfo.Start.GetOldest(); oldest != nil {
		block.Header.Number = 0
	}

	if seekInfo.ContentType == orderer.SeekInfo_HEADER_WITH_SIG {
		block.Data = nil
	}

	response := &orderer.DeliverResponse{
		Type: &orderer.DeliverResponse_Block{
			Block: block,
		},
	}

	stream.Send(response)
	return nil
}
