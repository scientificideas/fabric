/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package deliverservice

import (
	"math"
	"math/rand"
	"reflect"
	"sync"
	"time"

	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/gossip/util"
	"github.com/hyperledger/fabric/internal/pkg/identity"
	"github.com/hyperledger/fabric/internal/pkg/peer/blocksprovider"
	"github.com/hyperledger/fabric/internal/pkg/peer/orderers"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

var bftLogger = flogging.MustGetLogger("bftDeliveryClient")

const (
	defaultReConnectTotalTimeThreshold = time.Second * 60 * 60
	defaultConnectionTimeout           = time.Second * 3
	defaultReConnectBackoffThreshold   = time.Hour
)

func getReConnectTotalTimeThreshold() time.Duration {
	return util.GetDurationOrDefault("peer.deliveryclient.reconnectTotalTimeThreshold", defaultReConnectTotalTimeThreshold)
}

func getConnectionTimeout() time.Duration {
	return util.GetDurationOrDefault("peer.deliveryclient.connTimeout", defaultConnectionTimeout)
}

const (
	bftMinBackoffDelay           = 10 * time.Millisecond
	bftMaxBackoffDelay           = 10 * time.Second
	bftBlockRcvTotalBackoffDelay = 20 * time.Second
	bftBlockCensorshipTimeout    = 20 * time.Second
)

var (
	errNoBlockReceiver        = errors.New("no block receiver")
	errDuplicateBlock         = errors.New("duplicate block")
	errOutOfOrderBlock        = errors.New("out-of-order block")
	errClientClosing          = errors.New("client is closing")
	errClientReconnectTimeout = errors.New("client reconnect timeout")
)

// BlockReceiver is the deliver client which also returns the endpoint address
type BlockReceiver interface {
	blocksprovider.DeliverClient
	GetEndpoint() string
}

// bft delivery client
type bftDeliveryClient struct {
	mutex     sync.Mutex
	startOnce sync.Once
	stopFlag  bool
	stopChan  chan struct{}

	chainID            string                       // a.k.a. Channel ID
	ledgerInfoProvider blocksprovider.LedgerInfo    // provides access to the ledger height
	msgCryptoVerifier  blocksprovider.BlockVerifier // verifies headers

	signer      identity.SignerSerializer // signer for blocks requester
	tlsCertHash []byte                    // tlsCertHash
	dialer      blocksprovider.Dialer     // broadcast client dialer

	// The total time a bft client tries to connect (to all available endpoints) before giving up leadership
	reconnectTotalTimeThreshold time.Duration
	// the total time a block receiver tries to connect before giving up and letting the client to try another
	// endpoint as a block receiver
	reconnectBlockRcvTotalTimeThreshold time.Duration
	// The minimal time between connection retries. This interval grows exponentially until maxBackoffDelay, below.
	minBackoffDelay time.Duration
	// The maximal time between connection retries.
	maxBackoffDelay time.Duration
	// The block censorship timeout. A block censorship suspicion is declared if more than f header receivers are
	// ahead of the block receiver for a period larger than this timeout.
	blockCensorshipTimeout time.Duration

	updateEndpointsCh  chan []*orderers.Endpoint // channel receives new endpoints
	endpoints          []*orderers.Endpoint      // a set of endpoints
	blockReceiverIndex int                       // index of the current block receiver endpoint

	blockReceiver   BlockReceiver // block receiver
	nextBlockNumber uint64        // next block number
	lastBlockTime   time.Time     // last block time

	headerReceivers map[string]*bftHeaderReceiver
}

// NewBFTDeliveryClient creates an instance of an bftDeliveryClient
func NewBFTDeliveryClient(
	chainID string,
	orderers blocksprovider.OrdererConnectionSource,
	ledgerInfoProvider blocksprovider.LedgerInfo,
	msgVerifier blocksprovider.BlockVerifier,
	signer identity.SignerSerializer,
	tlsCertHash []byte,
	dialer blocksprovider.Dialer,
) (*bftDeliveryClient, error) {
	c := &bftDeliveryClient{
		stopChan:                            make(chan struct{}, 1),
		chainID:                             chainID,
		ledgerInfoProvider:                  ledgerInfoProvider,
		msgCryptoVerifier:                   msgVerifier,
		reconnectTotalTimeThreshold:         getReConnectTotalTimeThreshold(),
		reconnectBlockRcvTotalTimeThreshold: util.GetDurationOrDefault("peer.deliveryclient.bft.blockRcvTotalBackoffDelay", bftBlockRcvTotalBackoffDelay),
		minBackoffDelay:                     util.GetDurationOrDefault("peer.deliveryclient.bft.minBackoffDelay", bftMinBackoffDelay),
		maxBackoffDelay:                     util.GetDurationOrDefault("peer.deliveryclient.bft.maxBackoffDelay", bftMaxBackoffDelay),
		blockCensorshipTimeout:              util.GetDurationOrDefault("peer.deliveryclient.bft.blockCensorshipTimeout", bftBlockCensorshipTimeout),
		endpoints:                           shuffle(orderers.GetAllEndpoints()),
		blockReceiverIndex:                  -1,
		headerReceivers:                     make(map[string]*bftHeaderReceiver),
		signer:                              signer,
		tlsCertHash:                         tlsCertHash,
		dialer:                              dialer,
		updateEndpointsCh:                   orderers.InitUpdateEndpointsChannel(),
	}

	bftLogger.Infof("[%s] Created BFT Delivery Client", chainID)
	for i, e := range c.endpoints {
		bftLogger.Infof("[%s] %d: %s", chainID, i, e.Address)
	}
	return c, nil
}

// Send does nothing
func (c *bftDeliveryClient) Send(envelope *common.Envelope) error {
	// ignore
	return nil
}

// Recv receives a chaincode message
func (c *bftDeliveryClient) Recv() (response *orderer.DeliverResponse, err error) {
	bftLogger.Debugf("[%s] Entry", c.chainID)

	c.startOnce.Do(func() {
		var num uint64
		num, err = c.ledgerInfoProvider.LedgerHeight()
		if err != nil {
			return
		}
		c.nextBlockNumber = num
		c.lastBlockTime = time.Now()
		bftLogger.Debugf("[%s] Starting monitor routine; Initial ledger height: %d", c.chainID, num)
		go c.monitor()
		bftLogger.Debugf("[%s] Starting update endpoints routine; Current num of endpoints: %d", c.chainID, len(c.endpoints))
		go c.updateEndpoints()
	})
	// can only happen once, after first invocation
	if err != nil {
		bftLogger.Debugf("[%s] Exit: error=%v", c.chainID, err)
		return nil, errors.New("cannot access ledger height")
	}

	var numEP int
	var numRetries int
	var stopRetries = time.Now().Add(c.reconnectTotalTimeThreshold)

	for !c.shouldStop() {
		if numEP, err = c.assignReceivers(); err != nil {
			bftLogger.Debugf("[%s] Exit: error=%s", c.chainID, err)
			return nil, err
		}

		c.launchHeaderReceivers()

		response, err = c.receiveBlock()
		if err == nil {
			bftLogger.Debugf("[%s] Exit: response=%v", c.chainID, response)
			return response, nil // the normal return path
		}

		if stopRetries.Before(time.Now()) {
			bftLogger.Debugf("[%s] Exit: reconnectTotalTimeThreshold: %s, expired; error: %s",
				c.chainID, c.reconnectTotalTimeThreshold, errClientReconnectTimeout)
			return nil, errClientReconnectTimeout
		}

		c.closeBlockReceiver(false)
		numRetries++
		if numRetries%numEP == 0 { // double the back-off delay on every round of attempts.
			dur := backOffDuration(2.0, uint(numRetries/numEP), c.minBackoffDelay, c.maxBackoffDelay)
			bftLogger.Debugf("[%s] Got receive error: %s; going to retry another endpoint in: %s", c.chainID, err, dur)
			backOffSleep(dur, c.stopChan)
		} else {
			bftLogger.Debugf("[%s] Got receive error: %s; going to retry another endpoint now", c.chainID, err)
		}
	}

	bftLogger.Debugf("[%s] Exit: %s", c.chainID, errClientClosing.Error())
	return nil, errClientClosing
}

func backOffDuration(base float64, exponent uint, minDur, maxDur time.Duration) time.Duration {
	if base < 1.0 {
		base = 1.0
	}
	if minDur <= 0 {
		minDur = bftMinBackoffDelay
	}
	if maxDur < minDur {
		maxDur = minDur
	}

	fDurNano := math.Pow(base, float64(exponent)) * float64(minDur.Nanoseconds())
	fDurNano = math.Min(fDurNano, float64(maxDur.Nanoseconds()))
	return time.Duration(fDurNano)
}

func backOffSleep(backOffDur time.Duration, stopChan <-chan struct{}) {
	select {
	case <-time.After(backOffDur):
	case <-stopChan:
	}
}

// Check block reception progress relative to header reception progress.
// If the orderer associated with the block receiver is suspected of censorship, replace it with another orderer.
func (c *bftDeliveryClient) monitor() {
	bftLogger.Debugf("[%s] Entry", c.chainID)

	ticker := time.NewTicker(bftBlockCensorshipTimeout / 100)
	for !c.shouldStop() {
		if suspicion := c.detectBlockCensorship(); suspicion {
			c.closeBlockReceiver(true)
		}

		select {
		case <-ticker.C:
		case <-c.stopChan:
		}
	}
	ticker.Stop()

	bftLogger.Debugf("[%s] Exit", c.chainID)
}

// Check endpoints changes.
func (c *bftDeliveryClient) updateEndpoints() {
	bftLogger.Debugf("[%s] Entry", c.chainID)

	ticker := time.NewTicker(bftBlockCensorshipTimeout / 100)
	for !c.shouldStop() {
		select {
		case endpoints := <-c.updateEndpointsCh:
			bftLogger.Debugf("[%s] Received endpoints: %d", c.chainID, len(endpoints))
			c.UpdateEndpoints(endpoints)
		case <-c.stopChan:
		case <-ticker.C:
		}
	}

	bftLogger.Debugf("[%s] Exit", c.chainID)
}

func (c *bftDeliveryClient) detectBlockCensorship() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.blockReceiver == nil {
		return false
	}

	now := time.Now()
	suspicionThreshold := c.lastBlockTime.Add(c.blockCensorshipTimeout)
	if now.Before(suspicionThreshold) {
		return false
	}

	var numAhead int
	for ep, hRcv := range c.headerReceivers {
		blockNum, _, err := hRcv.LastBlockNum()
		if err != nil {
			continue
		}
		if blockNum >= c.nextBlockNumber {
			bftLogger.Debugf("[%s] header receiver: %s, is ahead of block receiver, headr-rcv=%d, block-rcv=%d", c.chainID, ep, blockNum, c.nextBlockNumber-1)
			numAhead++
		}
	}

	numEP := uint64(len(c.endpoints))
	_, f := computeQuorum(numEP)
	if numAhead > f {
		bftLogger.Warnf("[%s] suspected block censorship: %d header receivers are ahead of block receiver, out of %d endpoints",
			c.chainID, numAhead, numEP)
		return true
	}

	return false
}

func computeQuorum(N uint64) (Q int, F int) {
	F = int((int(N) - 1) / 3)
	Q = int(math.Ceil((float64(N) + float64(F) + 1) / 2.0))
	return
}

// (re)-assign a block delivery client and header delivery clients
func (c *bftDeliveryClient) assignReceivers() (int, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	numEP := len(c.endpoints)
	if numEP <= 0 {
		return numEP, errors.New("no endpoints")
	}

	if c.blockReceiver == nil {
		c.blockReceiverIndex = (c.blockReceiverIndex + 1) % numEP
		ep := c.endpoints[c.blockReceiverIndex]
		if headerReceiver, exists := c.headerReceivers[ep.Address]; exists {
			headerReceiver.CloseSend()
			delete(c.headerReceivers, ep.Address)
			bftLogger.Debugf("[%s] Closed header receiver to: %s", c.chainID, ep.Address)
		}

		var err error
		if c.blockReceiver, err = c.newBlockClient(ep); err != nil {
			return 0, errors.New("unable to create new block client")
		}
		bftLogger.Debugf("[%s] Created block receiver to: %s", c.chainID, ep.Address)
	}

	hRcvToCreate := make([]*orderers.Endpoint, 0)
	for i, ep := range c.endpoints {
		if i == c.blockReceiverIndex {
			continue
		}

		if hRcv, exists := c.headerReceivers[ep.Address]; exists {
			if hRcv.isStopped() {
				delete(c.headerReceivers, ep.Address)
			} else {
				continue
			}
		}

		hRcvToCreate = append(hRcvToCreate, ep)
	}

	for _, ep := range hRcvToCreate {
		headerClient, err := c.newHeaderClient(ep)
		if err != nil {
			return 0, errors.New("unable to create new header client")
		}
		headerReceiver := newBFTHeaderReceiver(c.chainID, ep.Address, headerClient, c.msgCryptoVerifier, c.minBackoffDelay, c.maxBackoffDelay)
		c.headerReceivers[ep.Address] = headerReceiver
		bftLogger.Debugf("[%s] Created header receiver to: %s", c.chainID, ep.Address)
	}

	bftLogger.Debugf("Exit: number of endpoints: %d", numEP)
	return numEP, nil
}

func (c *bftDeliveryClient) launchHeaderReceivers() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	var launched int
	for ep, hRcv := range c.headerReceivers {
		if !hRcv.isStarted() {
			bftLogger.Debugf("[%s] launching a header receiver to endpoint: %s", c.chainID, ep)
			launched++
			go hRcv.DeliverHeaders()
		}
	}

	bftLogger.Debugf("[%s] header receivers: launched=%d, total running=%d ", c.chainID, launched, len(c.headerReceivers))
}

func (c *bftDeliveryClient) receiveBlock() (*orderer.DeliverResponse, error) {
	c.mutex.Lock()
	receiver := c.blockReceiver
	nextBlockNumber := c.nextBlockNumber
	c.mutex.Unlock()

	// call Recv() without a lock
	if receiver == nil {
		return nil, errNoBlockReceiver
	}
	response, err := receiver.Recv()

	if err != nil {
		return response, err
	}
	// ignore older blocks, filter out-of-order blocks
	switch t := response.Type.(type) {
	case *orderer.DeliverResponse_Block:
		endpoint := receiver.GetEndpoint()
		if t.Block.Header.Number > nextBlockNumber {
			bftLogger.Warnf("[%s] Ignoring out-of-order block from orderer: %s; received block number: %d, expected: %d",
				c.chainID, endpoint, t.Block.Header.Number, nextBlockNumber)
			return nil, errOutOfOrderBlock
		}
		if t.Block.Header.Number < nextBlockNumber {
			bftLogger.Warnf("[%s] Ignoring duplicate block from orderer: %s; received block number: %d, expected: %d",
				c.chainID, endpoint, t.Block.Header.Number, nextBlockNumber)
			return nil, errDuplicateBlock
		}
		bftLogger.Debugf("[%s] Received block from orderer: %s; received block number: %d", c.chainID, endpoint, t.Block.Header.Number)
	}

	return response, err
}

func (c *bftDeliveryClient) closeBlockReceiver(updateLastBlockTime bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if updateLastBlockTime {
		c.lastBlockTime = time.Now()
	}

	if c.blockReceiver != nil {
		c.blockReceiver.CloseSend()
		c.blockReceiver = nil
	}
}

// LedgerHeight returns the next block number
func (c *bftDeliveryClient) LedgerHeight() (uint64, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.nextBlockNumber, nil
}

// CloseSend closes all receivers connections
func (c *bftDeliveryClient) CloseSend() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.stopFlag {
		return nil
	}

	c.stopFlag = true
	close(c.stopChan)

	c.disconnectAll()

	bftLogger.Debugf("Exit")

	return nil
}

func (c *bftDeliveryClient) disconnectAll() {
	if c.blockReceiver != nil {
		ep := c.blockReceiver.GetEndpoint()
		c.blockReceiver.CloseSend()
		bftLogger.Debugf("[%s] closed block receiver to: %s", c.chainID, ep)
		c.blockReceiver = nil
	}

	for ep, hRcv := range c.headerReceivers {
		hRcv.CloseSend()
		bftLogger.Debugf("[%s] closed header receiver to: %s", c.chainID, ep)
		delete(c.headerReceivers, ep)
	}
}

// Disconnect just the block receiver client, so that the next Recv() will choose a new one.
func (c *bftDeliveryClient) Disconnect() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.blockReceiver != nil {
		c.blockReceiver.CloseSend()
		c.blockReceiver = nil
	}
}

// UpdateReceived allows the client to track the reception of valid blocks.
// This is needed because blocks are verified by the blockprovider, not here.
func (c *bftDeliveryClient) UpdateReceived(blockNumber uint64) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	bftLogger.Infof("[%s] received blockNumber=%d", c.chainID, blockNumber)
	c.nextBlockNumber = blockNumber + 1
	c.lastBlockTime = time.Now()
}

func (c *bftDeliveryClient) shouldStop() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.stopFlag
}

// UpdateEndpoints assigns the new endpoints for the delivery client
func (c *bftDeliveryClient) UpdateEndpoints(endpoints []*orderers.Endpoint) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.stopFlag {
		return
	}

	if equalEndpoints(c.endpoints, endpoints) {
		return
	}

	bftLogger.Debugf("[%s] updating endpoints: existing: %v, new: %v", c.chainID, c.endpoints, endpoints)
	c.endpoints = endpoints
	c.blockReceiverIndex = 0
	c.disconnectAll()
}

// GetEndpoint provides the endpoint of the ordering service server that delivers
// blocks (as opposed to headers) to this delivery client.
func (c *bftDeliveryClient) GetEndpoint() string {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.blockReceiver == nil {
		return ""
	}
	return c.blockReceiver.GetEndpoint()
}

// GetNextBlockNumTime returns the next block number and the last block time
func (c *bftDeliveryClient) GetNextBlockNumTime() (uint64, time.Time) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.nextBlockNumber, c.lastBlockTime
}

// GetHeadersBlockNumTime returns a number of header receivers and their last block times
func (c *bftDeliveryClient) GetHeadersBlockNumTime() ([]uint64, []time.Time, []error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	hNum := make([]uint64, 0, len(c.headerReceivers))
	hTime := make([]time.Time, 0, len(c.headerReceivers))
	hErr := make([]error, 0, len(c.headerReceivers))
	for _, hRcv := range c.headerReceivers {
		num, t, err := hRcv.LastBlockNum()
		hNum = append(hNum, num)
		hTime = append(hTime, t)
		hErr = append(hErr, err)
	}
	return hNum, hTime, hErr
}

// create a deliver client that delivers blocks
func (c *bftDeliveryClient) newBlockClient(endpoint *orderers.Endpoint) (*broadcastClient, error) {
	requester := &blocksRequester{
		tls:         viper.GetBool("peer.tls.enabled"),
		chainID:     c.chainID,
		signer:      c.signer,
		tlsCertHash: c.tlsCertHash,
	}

	// Update the nextBlockNumber from the ledger, and then make sure the client request the nextBlockNumber,
	// because that is what we expect in the Recv() loop.
	if height, err := c.ledgerInfoProvider.LedgerHeight(); err == nil {
		c.nextBlockNumber = height
	}

	broadcastSetup := func(deliverer blocksprovider.DeliverClient) error {
		return requester.RequestBlocks(c) // Do not ask the ledger directly, ask the bftDeliveryClient
	}

	// Let block receivers give-up early so we can replace them with a header receiver
	backoffPolicy := func(attemptNum int, elapsedTime time.Duration) (time.Duration, bool) {
		if elapsedTime >= c.reconnectBlockRcvTotalTimeThreshold {
			return 0, false
		}
		return backOffDuration(2.0, uint(attemptNum), c.minBackoffDelay, c.maxBackoffDelay), true
	}

	clFactory := func(conn *grpc.ClientConn) orderer.AtomicBroadcastClient {
		return orderer.NewAtomicBroadcastClient(conn)
	}

	// Only a single endpoint
	blockClient := NewBroadcastClient(endpoint, c.defaultConnectionProducer, clFactory, broadcastSetup, backoffPolicy)
	requester.client = blockClient
	return blockClient, nil
}

// create a deliver client that delivers headers
func (c *bftDeliveryClient) newHeaderClient(endpoint *orderers.Endpoint) (*broadcastClient, error) {
	requester := &blocksRequester{
		tls:         viper.GetBool("peer.tls.enabled"),
		chainID:     c.chainID,
		signer:      c.signer,
		tlsCertHash: c.tlsCertHash,
	}

	broadcastSetup := func(deliverer blocksprovider.DeliverClient) error {
		return requester.RequestHeaders(c) // Do not ask the ledger directly, ask the bftDeliveryClient
	}

	// Let block receivers give-up early so we can replace them with a header receiver
	backoffPolicy := func(attemptNum int, elapsedTime time.Duration) (time.Duration, bool) {
		if elapsedTime >= c.reconnectTotalTimeThreshold { // Let header receivers continue to try until we close them
			return 0, false
		}
		return backOffDuration(2.0, uint(attemptNum), c.minBackoffDelay, c.maxBackoffDelay), true
	}

	clFactory := func(conn *grpc.ClientConn) orderer.AtomicBroadcastClient {
		return orderer.NewAtomicBroadcastClient(conn)
	}

	// Only a single endpoint
	headerClient := NewBroadcastClient(endpoint, c.defaultConnectionProducer, clFactory, broadcastSetup, backoffPolicy)
	requester.client = headerClient
	return headerClient, nil
}

func shuffle(a []*orderers.Endpoint) []*orderers.Endpoint {
	n := len(a)
	returnedSlice := make([]*orderers.Endpoint, n)
	rand.Seed(time.Now().UnixNano())
	indices := rand.Perm(n)
	for i, idx := range indices {
		returnedSlice[i] = a[idx]
	}
	return returnedSlice
}

func equalEndpoints(existingEndpoints, newEndpoints []*orderers.Endpoint) bool {
	if len(newEndpoints) != len(existingEndpoints) {
		return false
	}

	// Check that endpoints were actually updated
	for _, endpoint := range newEndpoints {
		if !contains(endpoint, existingEndpoints) {
			// Found new endpoint
			return false
		}
	}
	// Endpoints are of the same length and the existing endpoints contain all the new endpoints,
	// so there are no new changes.
	return true
}

func contains(s *orderers.Endpoint, a []*orderers.Endpoint) bool {
	for _, e := range a {
		if e.Address == s.Address && reflect.DeepEqual(e.RootCerts, s.RootCerts) {
			return true
		}
	}
	return false
}

func (c *bftDeliveryClient) defaultConnectionProducer(endpoint *orderers.Endpoint) (*grpc.ClientConn, error) {
	return c.dialer.Dial(endpoint.Address, endpoint.RootCerts)
}
