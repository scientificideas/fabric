/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package deliverservice

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric/internal/pkg/peer/blocksprovider"
	"github.com/hyperledger/fabric/internal/pkg/peer/orderers"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

// broadcastSetup is a function that is called by the broadcastClient immediately after each successful connection to
// the ordering service in order to request the reception of a stream of blocks
type broadcastSetup func(deliverClient blocksprovider.DeliverClient) error

// retryPolicy receives as parameters the number of times the attempt has failed
// and a duration that specifies the total elapsed time passed since the first attempt.
// If further attempts should be made, it returns:
// 	- a time duration after which the next attempt would be made, true
// Else, a zero duration, false
type retryPolicy func(attemptNum int, elapsedTime time.Duration) (time.Duration, bool)

// clientFactory creates a gRPC broadcast client out of a ClientConn
type clientFactory func(*grpc.ClientConn) orderer.AtomicBroadcastClient

// connectionProducer produces connection out of a endpoint
type connectionProducer func(endpoint *orderers.Endpoint) (*grpc.ClientConn, error)

type broadcastClient struct {
	stopFlag           int32
	stopChan           chan struct{}
	createClient       clientFactory
	shouldRetry        retryPolicy
	onConnect          broadcastSetup
	connectionProducer connectionProducer
	ep                 *orderers.Endpoint

	mutex           sync.Mutex
	blocksDeliverer orderer.AtomicBroadcast_DeliverClient
	conn            *connection
	endpoint        string
}

// NewBroadcastClient returns a broadcastClient with the given params
func NewBroadcastClient(ep *orderers.Endpoint, connectionProducer connectionProducer, clFactory clientFactory, onConnect broadcastSetup, bos retryPolicy) *broadcastClient {
	return &broadcastClient{
		ep:                 ep,
		connectionProducer: connectionProducer,
		onConnect:          onConnect,
		shouldRetry:        bos,
		createClient:       clFactory,
		stopChan:           make(chan struct{}, 1),
	}
}

// Recv receives a message from the ordering service
func (bc *broadcastClient) Recv() (*orderer.DeliverResponse, error) {
	o, err := bc.try(func() (interface{}, error) {
		if bc.shouldStop() {
			return nil, errors.New("closing")
		}
		return bc.tryReceive()
	})
	if err != nil {
		return nil, err
	}
	return o.(*orderer.DeliverResponse), nil
}

// Send sends a message to the ordering service
func (bc *broadcastClient) Send(msg *common.Envelope) error {
	_, err := bc.try(func() (interface{}, error) {
		if bc.shouldStop() {
			return nil, errors.New("closing")
		}
		return bc.trySend(msg)
	})
	return err
}

func (bc *broadcastClient) trySend(msg *common.Envelope) (interface{}, error) {
	bc.mutex.Lock()
	stream := bc.blocksDeliverer
	bc.mutex.Unlock()
	if stream == nil {
		return nil, errors.New("client stream has been closed")
	}
	return nil, stream.Send(msg)
}

func (bc *broadcastClient) tryReceive() (*orderer.DeliverResponse, error) {
	bc.mutex.Lock()
	stream := bc.blocksDeliverer
	bc.mutex.Unlock()
	if stream == nil {
		return nil, errors.New("client stream has been closed")
	}
	return stream.Recv()
}

func (bc *broadcastClient) try(action func() (interface{}, error)) (interface{}, error) {
	attempt := 0
	var totalRetryTime time.Duration
	var backoffDuration time.Duration
	retry := true
	resetAttemptCounter := func() {
		attempt = 0
		totalRetryTime = 0
	}
	for retry && !bc.shouldStop() {
		resp, err := bc.doAction(action, resetAttemptCounter)
		if err != nil {
			attempt++
			backoffDuration, retry = bc.shouldRetry(attempt, totalRetryTime)
			if !retry {
				logger.Warning("Got error:", err, "at", attempt, "attempt. Ceasing to retry")
				break
			}
			logger.Warning("Got error:", err, ", at", attempt, "attempt. Retrying in", backoffDuration)
			totalRetryTime += backoffDuration
			bc.sleep(backoffDuration)
			continue
		}
		return resp, nil
	}
	if bc.shouldStop() {
		return nil, errors.New("client is closing")
	}
	return nil, fmt.Errorf("attempts (%d) or elapsed time (%v) exhausted", attempt, totalRetryTime)
}

func (bc *broadcastClient) doAction(action func() (interface{}, error), actionOnNewConnection func()) (interface{}, error) {
	bc.mutex.Lock()
	conn := bc.conn
	bc.mutex.Unlock()
	if conn == nil {
		err := bc.connect()
		if err != nil {
			return nil, err
		}
		actionOnNewConnection()
	}
	resp, err := action()
	if err != nil {
		bc.Disconnect()
		return nil, err
	}
	return resp, nil
}

func (bc *broadcastClient) sleep(duration time.Duration) {
	select {
	case <-time.After(duration):
	case <-bc.stopChan:
	}
}

func (bc *broadcastClient) connect() error {
	bc.mutex.Lock()
	bc.endpoint = ""
	bc.mutex.Unlock()
	conn, err := bc.connectionProducer(bc.ep)
	if err != nil {
		logger.Errorf("Failed obtaining connection to %s: %s", bc.ep.Address, err)
		return err
	}
	logger.Debug("Connected to", bc.ep.Address)
	ctx, cf := context.WithCancel(context.Background())
	logger.Debug("Establishing gRPC stream with", bc.ep.Address, "...")
	abc, err := bc.createClient(conn).Deliver(ctx)
	if err != nil {
		logger.Error("Connection to ", bc.ep.Address, "established but was unable to create gRPC stream:", err)
		conn.Close()
		cf()
		return err
	}
	err = bc.afterConnect(conn, abc, cf)
	if err == nil {
		return nil
	}
	logger.Warning("Failed running post-connection procedures:", err)
	// If we reached here, lets make sure connection is closed
	// and nullified before we return
	bc.Disconnect()
	return err
}

func (bc *broadcastClient) afterConnect(conn *grpc.ClientConn, deliverClient orderer.AtomicBroadcast_DeliverClient, cf context.CancelFunc) error {
	logger.Debug("Entering")
	defer logger.Debug("Exiting")
	bc.mutex.Lock()
	bc.endpoint = bc.ep.Address
	bc.conn = &connection{ClientConn: conn, cancel: cf}
	bc.blocksDeliverer = deliverClient
	if bc.shouldStop() {
		bc.mutex.Unlock()
		return errors.New("closing")
	}
	bc.mutex.Unlock()

	// If the client is closed at this point- before onConnect,
	// any use of this object by onConnect would return an error.
	err := bc.onConnect(bc)

	// If the client is closed right after onConnect, but before
	// the following lock- this method would return an error because
	// the client has been closed.
	bc.mutex.Lock()
	defer bc.mutex.Unlock()
	if bc.shouldStop() {
		return errors.New("closing")
	}
	// If the client is closed right after this method exits,
	// it's because this method returned nil and not an error.
	// So- connect() would return nil also, and the flow of the goroutine
	// is returned to doAction(), where action() is invoked - and is configured
	// to check whether the client has closed or not.
	if err == nil {
		return nil
	}
	logger.Error("Failed setting up broadcast:", err)
	return err
}

func (bc *broadcastClient) shouldStop() bool {
	return atomic.LoadInt32(&bc.stopFlag) == int32(1)
}

// CloseSend makes the client close its connection and shut down
func (bc *broadcastClient) CloseSend() error {
	logger.Debugf("Entering for ep=%s", bc.ep.Address)
	defer logger.Debugf("Exiting for ep=%s", bc.ep.Address)
	bc.mutex.Lock()
	defer bc.mutex.Unlock()
	if bc.shouldStop() {
		return nil
	}
	atomic.StoreInt32(&bc.stopFlag, int32(1))
	bc.stopChan <- struct{}{}
	if bc.conn == nil {
		return nil
	}
	bc.endpoint = ""
	if err := bc.conn.Close(); err != nil {
		// todo:
	}
	return nil
}

// Disconnect makes the client close the existing connection and makes current endpoint unavailable for time interval, if disableEndpoint set to true
func (bc *broadcastClient) Disconnect() {
	logger.Debug("Entering")
	defer logger.Debug("Exiting")
	bc.mutex.Lock()
	defer bc.mutex.Unlock()
	bc.endpoint = ""
	if bc.conn == nil {
		return
	}
	bc.conn.Close()
	bc.conn = nil
	bc.blocksDeliverer = nil
}

// GetEndpoint returns the endpoint the client is currently connected to.
func (bc *broadcastClient) GetEndpoint() string {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()

	return bc.endpoint
}

type connection struct {
	sync.Once
	*grpc.ClientConn
	cancel context.CancelFunc
}

// Close closes client connection
func (c *connection) Close() error {
	var err error
	c.Once.Do(func() {
		c.cancel()
		err = c.ClientConn.Close()
	})
	return err
}
