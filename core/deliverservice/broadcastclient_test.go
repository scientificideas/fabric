/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package deliverservice

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/hyperledger/fabric/core/deliverservice/fake"
	"github.com/hyperledger/fabric/core/deliverservice/mocks"
	mocks2 "github.com/hyperledger/fabric/orderer/consensus/smartbft/mocks"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/stretchr/testify/mock"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric/internal/pkg/peer/blocksprovider"
	"github.com/hyperledger/fabric/internal/pkg/peer/orderers"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

var (
	connWG sync.WaitGroup
)

func newConnection() *grpc.ClientConn {
	// todo: balancer
	cc, _ := grpc.Dial("", grpc.WithInsecure())
	return cc
}

type blocksDelivererConsumer func(streamer blocksprovider.DeliverClient) error

var blockDelivererConsumerWithRecv = func(bd blocksprovider.DeliverClient) error {
	_, err := bd.Recv()
	return err
}

var blockDelivererConsumerWithSend = func(bd blocksprovider.DeliverClient) error {
	return bd.Send(&common.Envelope{})
}

type abc struct {
	shouldFail bool
	grpc.ClientStream
}

func (a *abc) Send(*common.Envelope) error {
	if a.shouldFail {
		return errors.New("Failed sending")
	}
	return nil
}

func (a *abc) Recv() (*orderer.DeliverResponse, error) {
	if a.shouldFail {
		return nil, errors.New("Failed Recv")
	}
	return &orderer.DeliverResponse{}, nil
}

type abclient struct {
	shouldFail bool
	stream     *abc
}

func (ac *abclient) Broadcast(ctx context.Context, opts ...grpc.CallOption) (orderer.AtomicBroadcast_BroadcastClient, error) {
	panic("Not implemented")
}

func (ac *abclient) Deliver(ctx context.Context, opts ...grpc.CallOption) (orderer.AtomicBroadcast_DeliverClient, error) {
	if ac.stream != nil {
		return ac.stream, nil
	}
	if ac.shouldFail {
		return nil, errors.New("Failed creating ABC")
	}
	return &abc{}, nil
}

var connProducerEnpoint = &orderers.Endpoint{
	Address:   "localhost:5611",
	CertPool:  nil,
	Refreshed: nil,
}

type connProducer struct {
	shouldFail   bool
	connAttempts int
	connTime     time.Duration
	real         bool
}

func (cp *connProducer) NewConnection(endpoint *orderers.Endpoint) (*grpc.ClientConn, error) {
	time.Sleep(cp.connTime)
	cp.connAttempts++
	if cp.real {
		return grpc.Dial(endpoint.Address, grpc.WithInsecure())
	}
	if cp.shouldFail {
		return nil, errors.New("Failed connecting")
	}
	return newConnection(), nil
}

// UpdateEndpoints updates the endpoints of the ConnectionProducer
// to be the given endpoints
func (cp *connProducer) UpdateEndpoints(endpoints []*orderers.Endpoint) {
	panic("Not implemented")
}

func (cp *connProducer) GetEndpoints() []*orderers.Endpoint {
	panic("Not implemented")
}

func TestOrderingServiceConnFailure(t *testing.T) {
	testOrderingServiceConnFailure(t, blockDelivererConsumerWithRecv)
	testOrderingServiceConnFailure(t, blockDelivererConsumerWithSend)
	connWG.Wait()
}

func testOrderingServiceConnFailure(t *testing.T, bdc blocksDelivererConsumer) {
	// Scenario: Create a broadcast client and call Recv/Send.
	// Connection to the ordering service should be possible only at second attempt
	cp := &connProducer{shouldFail: true}
	clFactory := func(*grpc.ClientConn) orderer.AtomicBroadcastClient {
		return &abclient{}
	}
	setupInvoked := 0
	setup := func(deliverer blocksprovider.DeliverClient) error {
		setupInvoked++
		return nil
	}
	backoffStrategy := func(attemptNum int, elapsedTime time.Duration) (time.Duration, bool) {
		// When called with the second attempt (iteration number 1),
		// we should be able to connect to the ordering service.
		// Set connection provider mock shouldFail flag to false
		// to enable next connection attempt to succeed
		if attemptNum == 1 {
			cp.shouldFail = false
		}

		return time.Duration(0), attemptNum < 2
	}
	bc := NewBroadcastClient(connProducerEnpoint, cp.NewConnection, clFactory, setup, backoffStrategy)
	defer bc.CloseSend()
	err := bdc(bc)
	assert.NoError(t, err)
	assert.Equal(t, 2, cp.connAttempts)
	assert.Equal(t, 1, setupInvoked)
}

func TestOrderingServiceStreamFailure(t *testing.T) {
	testOrderingServiceStreamFailure(t, blockDelivererConsumerWithRecv)
	testOrderingServiceStreamFailure(t, blockDelivererConsumerWithSend)
	connWG.Wait()
}

func testOrderingServiceStreamFailure(t *testing.T, bdc blocksDelivererConsumer) {
	// Scenario: Create a broadcast client and call Recv/Send.
	// Connection to the ordering service should be possible at first attempt,
	// but the atomic broadcast client creation fails at first attempt
	abcClient := &abclient{shouldFail: true}
	cp := &connProducer{}
	clFactory := func(*grpc.ClientConn) orderer.AtomicBroadcastClient {
		return abcClient
	}
	setupInvoked := 0
	setup := func(blocksprovider.DeliverClient) error {
		setupInvoked++
		return nil
	}
	backoffStrategy := func(attemptNum int, elapsedTime time.Duration) (time.Duration, bool) {
		// When called with the second attempt (iteration number 1),
		// we should be able to finally call Deliver() by the atomic broadcast client
		if attemptNum == 1 {
			abcClient.shouldFail = false
		}
		return time.Duration(0), attemptNum < 2
	}
	bc := NewBroadcastClient(connProducerEnpoint, cp.NewConnection, clFactory, setup, backoffStrategy)
	defer bc.CloseSend()
	err := bdc(bc)
	assert.NoError(t, err)
	assert.Equal(t, 2, cp.connAttempts)
	assert.Equal(t, 1, setupInvoked)
}

func TestOrderingServiceSetupFailure(t *testing.T) {
	testOrderingServiceSetupFailure(t, blockDelivererConsumerWithRecv)
	testOrderingServiceSetupFailure(t, blockDelivererConsumerWithSend)
	connWG.Wait()
}

func testOrderingServiceSetupFailure(t *testing.T, bdc blocksDelivererConsumer) {
	// Scenario: Create a broadcast client and call Recv/Send.
	// Connection to the ordering service should be possible,
	// the atomic broadcast client creation succeeds, but invoking the setup function
	// fails at the first attempt and succeeds at the second one.
	cp := &connProducer{}
	clFactory := func(*grpc.ClientConn) orderer.AtomicBroadcastClient {
		return &abclient{}
	}
	setupInvoked := 0
	setup := func(blocksprovider.DeliverClient) error {
		setupInvoked++
		if setupInvoked == 1 {
			return errors.New("Setup failed")
		}
		return nil
	}
	backoffStrategy := func(attemptNum int, elapsedTime time.Duration) (time.Duration, bool) {
		return time.Duration(0), true
	}
	bc := NewBroadcastClient(connProducerEnpoint, cp.NewConnection, clFactory, setup, backoffStrategy)
	defer bc.CloseSend()
	err := bdc(bc)
	assert.NoError(t, err)
	assert.Equal(t, 2, cp.connAttempts)
	assert.Equal(t, 2, setupInvoked)
}

func TestOrderingServiceFirstOperationFailure(t *testing.T) {
	testOrderingServiceFirstOperationFailure(t, blockDelivererConsumerWithRecv)
	testOrderingServiceFirstOperationFailure(t, blockDelivererConsumerWithSend)
	connWG.Wait()
}

func testOrderingServiceFirstOperationFailure(t *testing.T, bdc blocksDelivererConsumer) {
	// Scenario: Creation and connection to the ordering service succeeded
	// but the first Recv/Send failed.
	// The client should reconnect to the ordering service
	cp := &connProducer{}
	abStream := &abc{shouldFail: true}
	abcClient := &abclient{stream: abStream}
	clFactory := func(*grpc.ClientConn) orderer.AtomicBroadcastClient {
		return abcClient
	}

	setupInvoked := 0
	setup := func(blocksprovider.DeliverClient) error {
		// Fix stream success logic at 2nd attempt
		if setupInvoked == 1 {
			abStream.shouldFail = false
		}
		setupInvoked++
		return nil
	}
	backoffStrategy := func(attemptNum int, elapsedTime time.Duration) (time.Duration, bool) {
		return time.Duration(0), true
	}
	bc := NewBroadcastClient(connProducerEnpoint, cp.NewConnection, clFactory, setup, backoffStrategy)
	defer bc.CloseSend()
	err := bdc(bc)
	assert.NoError(t, err)
	assert.Equal(t, 2, setupInvoked)
	assert.Equal(t, cp.connAttempts, 2)
}

func TestOrderingServiceCrashAndRecover(t *testing.T) {
	testOrderingServiceCrashAndRecover(t, blockDelivererConsumerWithRecv)
	testOrderingServiceCrashAndRecover(t, blockDelivererConsumerWithSend)
	connWG.Wait()
}

func testOrderingServiceCrashAndRecover(t *testing.T, bdc blocksDelivererConsumer) {
	// Scenario: The ordering service is OK at first usage of Recv/Send,
	// but subsequent calls fails because of connection error.
	// A reconnect is needed and only then Recv/Send should succeed
	cp := &connProducer{}
	abStream := &abc{}
	abcClient := &abclient{stream: abStream}
	clFactory := func(*grpc.ClientConn) orderer.AtomicBroadcastClient {
		return abcClient
	}

	setupInvoked := 0
	setup := func(blocksprovider.DeliverClient) error {
		// Fix stream success logic at 2nd attempt
		if setupInvoked == 1 {
			abStream.shouldFail = false
		}
		setupInvoked++
		return nil
	}
	backoffStrategy := func(attemptNum int, elapsedTime time.Duration) (time.Duration, bool) {
		return time.Duration(0), true
	}
	bc := NewBroadcastClient(connProducerEnpoint, cp.NewConnection, clFactory, setup, backoffStrategy)
	defer bc.CloseSend()
	err := bdc(bc)
	assert.NoError(t, err)
	// Now fail the subsequent Recv/Send
	abStream.shouldFail = true
	err = bdc(bc)
	assert.NoError(t, err)
	assert.Equal(t, 2, cp.connAttempts)
	assert.Equal(t, 2, setupInvoked)
}

func TestOrderingServicePermanentCrash(t *testing.T) {
	testOrderingServicePermanentCrash(t, blockDelivererConsumerWithRecv)
	testOrderingServicePermanentCrash(t, blockDelivererConsumerWithSend)
	connWG.Wait()
}

func testOrderingServicePermanentCrash(t *testing.T, bdc blocksDelivererConsumer) {
	// Scenario: The ordering service is OK at first usage of Recv/Send,
	// but subsequent calls fail because it crashes.
	// The client should give up after 10 attempts in the second reconnect
	cp := &connProducer{}
	abStream := &abc{}
	abcClient := &abclient{stream: abStream}
	clFactory := func(*grpc.ClientConn) orderer.AtomicBroadcastClient {
		return abcClient
	}

	setupInvoked := 0
	setup := func(blocksprovider.DeliverClient) error {
		setupInvoked++
		return nil
	}
	backoffStrategy := func(attemptNum int, elapsedTime time.Duration) (time.Duration, bool) {
		return time.Duration(0), attemptNum < 10
	}
	bc := NewBroadcastClient(connProducerEnpoint, cp.NewConnection, clFactory, setup, backoffStrategy)
	defer bc.CloseSend()
	err := bdc(bc)
	assert.NoError(t, err)
	// Now fail the subsequent Recv/Send
	abStream.shouldFail = true
	cp.shouldFail = true
	err = bdc(bc)
	assert.Error(t, err)
	assert.Equal(t, 10, cp.connAttempts)
	assert.Equal(t, 1, setupInvoked)
}

func TestLimitedConnAttempts(t *testing.T) {
	testLimitedConnAttempts(t, blockDelivererConsumerWithRecv)
	testLimitedConnAttempts(t, blockDelivererConsumerWithSend)
	connWG.Wait()
}

func testLimitedConnAttempts(t *testing.T, bdc blocksDelivererConsumer) {
	// Scenario: The ordering service isn't available, and the backoff strategy
	// specifies an upper bound of 10 connection attempts
	cp := &connProducer{shouldFail: true}
	clFactory := func(*grpc.ClientConn) orderer.AtomicBroadcastClient {
		return &abclient{}
	}
	setupInvoked := 0
	setup := func(blocksprovider.DeliverClient) error {
		setupInvoked++
		return nil
	}
	backoffStrategy := func(attemptNum int, elapsedTime time.Duration) (time.Duration, bool) {
		return time.Duration(0), attemptNum < 10
	}
	bc := NewBroadcastClient(connProducerEnpoint, cp.NewConnection, clFactory, setup, backoffStrategy)
	defer bc.CloseSend()
	err := bdc(bc)
	assert.Error(t, err)
	assert.Equal(t, 10, cp.connAttempts)
	assert.Equal(t, 0, setupInvoked)
}

func TestLimitedTotalConnTimeRcv(t *testing.T) {
	testLimitedTotalConnTime(t, blockDelivererConsumerWithRecv)
	connWG.Wait()
}

func TestLimitedTotalConnTimeSnd(t *testing.T) {
	testLimitedTotalConnTime(t, blockDelivererConsumerWithSend)
	connWG.Wait()
}

func testLimitedTotalConnTime(t *testing.T, bdc blocksDelivererConsumer) {
	// Scenario: The ordering service isn't available, and the backoff strategy
	// specifies an upper bound of 1 second
	// The first attempt fails, and the second attempt doesn't take place
	// becuse the creation of connection takes 1.5 seconds.
	cp := &connProducer{shouldFail: true, connTime: 1500 * time.Millisecond}
	clFactory := func(*grpc.ClientConn) orderer.AtomicBroadcastClient {
		return &abclient{}
	}
	setupInvoked := 0
	setup := func(blocksprovider.DeliverClient) error {
		setupInvoked++
		return nil
	}
	backoffStrategy := func(attemptNum int, elapsedTime time.Duration) (time.Duration, bool) {
		return time.Millisecond * 500, elapsedTime.Nanoseconds() < time.Second.Nanoseconds()
	}
	bc := NewBroadcastClient(connProducerEnpoint, cp.NewConnection, clFactory, setup, backoffStrategy)
	defer bc.CloseSend()
	err := bdc(bc)
	assert.Error(t, err)
	assert.Equal(t, 3, cp.connAttempts)
	assert.Equal(t, 0, setupInvoked)
}

func TestGreenPath(t *testing.T) {
	testGreenPath(t, blockDelivererConsumerWithRecv)
	testGreenPath(t, blockDelivererConsumerWithSend)
	connWG.Wait()
}

func testGreenPath(t *testing.T, bdc blocksDelivererConsumer) {
	// Scenario: Everything succeeds
	cp := &connProducer{}
	abcClient := &abclient{}
	clFactory := func(*grpc.ClientConn) orderer.AtomicBroadcastClient {
		return abcClient
	}

	setupInvoked := 0
	setup := func(blocksprovider.DeliverClient) error {
		setupInvoked++
		return nil
	}
	backoffStrategy := func(attemptNum int, elapsedTime time.Duration) (time.Duration, bool) {
		return time.Duration(0), attemptNum < 1
	}
	bc := NewBroadcastClient(connProducerEnpoint, cp.NewConnection, clFactory, setup, backoffStrategy)
	defer bc.CloseSend()
	err := bdc(bc)
	assert.NoError(t, err)
	assert.Equal(t, 1, cp.connAttempts)
	assert.Equal(t, 1, setupInvoked)
}

func TestCloseWhileRecv(t *testing.T) {
	// Scenario: Recv is being called and after a while,
	// the connection is closed.
	// The Recv should return immediately in such a case
	fakeOrderer := mocks.NewOrderer(5611, t)
	fakeOrderer.SetNextExpectedSeek(5)
	defer fakeOrderer.Shutdown()

	time.Sleep(time.Second)
	cp := &connProducer{real: true}
	clFactory := func(conn *grpc.ClientConn) orderer.AtomicBroadcastClient {
		return orderer.NewAtomicBroadcastClient(conn)
	}

	fakeLedgerInfo := &fake.LedgerInfo{}
	fakeLedgerInfo.LedgerHeightReturns(5, nil)

	mockSignerSerializer := &mocks2.SignerSerializer{}
	mockSignerSerializer.On("Sign", mock.Anything).Return([]byte{1, 2, 3}, nil)
	mockSignerSerializer.On("Serialize", mock.Anything).Return([]byte{0, 2, 4, 6}, nil)

	broadcastSetup := func(bd blocksprovider.DeliverClient) error {
		requester := &blocksRequester{
			chainID:           "TEST_CHAIN",
			signer:            mockSignerSerializer,
		}
		seekInfoEnv, err := requester.RequestBlocks(fakeLedgerInfo)
		if err != nil {
			return fmt.Errorf("could not create header seek info for channel %s: %w", requester.chainID, err)
		}
		return bd.Send(seekInfoEnv)
	}
	backoffStrategy := func(attemptNum int, elapsedTime time.Duration) (time.Duration, bool) {
		return 0, true
	}

	bc := NewBroadcastClient(connProducerEnpoint, cp.NewConnection, clFactory, broadcastSetup, backoffStrategy)

	var flag int32
	go func() {
		for i := 0; fakeOrderer.ConnCount() == 0; i++ {
			time.Sleep(time.Second)
			if i > 10 {
				assert.Fail(t, "no connection")
				break
			}
		}
		atomic.StoreInt32(&flag, int32(1))
		bc.CloseSend()
		bc.CloseSend() // Try to close a second time
	}()
	resp, err := bc.Recv()
	// Ensure we returned because bc.Close() was called and not because some other reason
	assert.Equal(t, int32(1), atomic.LoadInt32(&flag), "Recv returned before bc.Close() was called")
	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Contains(t, "client is closing", err.Error())
}

func TestCloseWhileSleep(t *testing.T) {
	testCloseWhileSleep(t, blockDelivererConsumerWithRecv)
	testCloseWhileSleep(t, blockDelivererConsumerWithSend)
	connWG.Wait()
}

func testCloseWhileSleep(t *testing.T, bdc blocksDelivererConsumer) {
	// Scenario: Recv/Send is being called while sleeping because
	// of the backoff policy is programmed to sleep 1000000 seconds
	// between retries.
	// The Recv/Send should return pretty soon
	cp := &connProducer{}
	abcClient := &abclient{shouldFail: true}
	clFactory := func(*grpc.ClientConn) orderer.AtomicBroadcastClient {
		return abcClient
	}

	setupInvoked := 0
	setup := func(blocksprovider.DeliverClient) error {
		setupInvoked++
		return nil
	}
	var wg sync.WaitGroup
	wg.Add(1)
	backoffStrategy := func(attemptNum int, elapsedTime time.Duration) (time.Duration, bool) {
		if attemptNum == 1 {
			go func() {
				time.Sleep(time.Second)
				wg.Done()
			}()
		}
		return time.Second * 1000000, true
	}
	bc := NewBroadcastClient(connProducerEnpoint, cp.NewConnection, clFactory, setup, backoffStrategy)
	go func() {
		wg.Wait()
		bc.CloseSend()
		bc.CloseSend() // Try to close a second time
	}()
	err := bdc(bc)
	assert.Error(t, err)
	assert.Equal(t, 1, cp.connAttempts)
	assert.Equal(t, 0, setupInvoked)
}

type signerMock struct {
}

func (s *signerMock) NewSignatureHeader() (*common.SignatureHeader, error) {
	return &common.SignatureHeader{}, nil
}

func (s *signerMock) Sign(message []byte) ([]byte, error) {
	hasher := sha256.New()
	hasher.Write(message)
	return hasher.Sum(nil), nil
}

func TestProductionUsage(t *testing.T) {
	// This test configures the client in a similar fashion as will be
	// in production, and tests against a live gRPC server.
	os := mocks.NewOrderer(5612, t)
	os.SetNextExpectedSeek(5)

	connFact := func(endpoint *orderers.Endpoint) (*grpc.ClientConn, error) {
		return grpc.Dial(endpoint.Address, grpc.WithInsecure(), grpc.WithBlock())
	}

	endpoint := &orderers.Endpoint{Address: "localhost:5612"}
	clFact := func(cc *grpc.ClientConn) orderer.AtomicBroadcastClient {
		return orderer.NewAtomicBroadcastClient(cc)
	}

	mockSignerSerializer := &mocks2.SignerSerializer{}
	mockSignerSerializer.On("Sign", mock.Anything).Return([]byte{1, 2, 3}, nil)
	mockSignerSerializer.On("Serialize", mock.Anything).Return([]byte{0, 2, 4, 6}, nil)

	onConnect := func(bd blocksprovider.DeliverClient) error {
		env, err := protoutil.CreateSignedEnvelope(common.HeaderType_CONFIG_UPDATE, "TEST", mockSignerSerializer, newTestSeekInfo(), 0, 0)
		assert.NoError(t, err)
		return bd.Send(env)
	}
	retryPol := func(attemptNum int, elapsedTime time.Duration) (time.Duration, bool) {
		return time.Second * 3, attemptNum < 2
	}
	bc := NewBroadcastClient(endpoint, connFact, clFact, onConnect, retryPol)
	go os.SendBlock(5)
	resp, err := bc.Recv()
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, uint64(5), resp.GetBlock().Header.Number)
	os.Shutdown()
	bc.CloseSend()
}

func TestDisconnect(t *testing.T) {
	// Scenario: spawn ordering service instance and a client.
	// Have the client try to Recv() and disconnect the client.

	os1 := mocks.NewOrderer(5613, t)
	os1.SetNextExpectedSeek(5)

	defer os1.Shutdown()

	connFact := func(endpoint *orderers.Endpoint) (*grpc.ClientConn, error) {
		return grpc.Dial(endpoint.Address, grpc.WithInsecure(), grpc.WithBlock())
	}

	endpoint := &orderers.Endpoint{Address: "localhost:5613"}
	clFact := func(cc *grpc.ClientConn) orderer.AtomicBroadcastClient {
		return orderer.NewAtomicBroadcastClient(cc)
	}

	mockSignerSerializer := &mocks2.SignerSerializer{}
	mockSignerSerializer.On("Sign", mock.Anything).Return([]byte{1, 2, 3}, nil)
	mockSignerSerializer.On("Serialize", mock.Anything).Return([]byte{0, 2, 4, 6}, nil)


	fakeLedgerInfo := &fake.LedgerInfo{}
	fakeLedgerInfo.LedgerHeightReturns(5, nil)

	broadcastSetup := func(bd blocksprovider.DeliverClient) error {
		requester := &blocksRequester{
			chainID:           "TEST_CHAIN",
			signer:            mockSignerSerializer,
		}
		seekInfoEnv, err := requester.RequestBlocks(fakeLedgerInfo)
		if err != nil {
			return fmt.Errorf("could not create header seek info for channel %s: %w", requester.chainID, err)
		}
		return bd.Send(seekInfoEnv)
	}

	retryPol := func(attemptNum int, elapsedTime time.Duration) (time.Duration, bool) {
		return time.Millisecond * 10, attemptNum < 100
	}

	bc := NewBroadcastClient(endpoint, connFact, clFact, broadcastSetup, retryPol)
	stopChan := make(chan struct{})
	go func() {
		bc.Recv()
		stopChan <- struct{}{}
	}()
	bc.Disconnect()
	bc.CloseSend()
	select {
	case <-stopChan:
	case <-time.After(time.Second * 20):
		assert.Fail(t, "Didn't stop within a timely manner")
	}
}

func newTestSeekInfo() *orderer.SeekInfo {
	return &orderer.SeekInfo{Start: &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: 5}}},
		Stop:     &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: math.MaxUint64}}},
		Behavior: orderer.SeekInfo_BLOCK_UNTIL_READY,
	}
}
