package bftblocksprovider

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/rand"
	"reflect"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/gossip"
	"github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric/common/flogging"
	gossipcommon "github.com/hyperledger/fabric/gossip/common"
	"github.com/hyperledger/fabric/gossip/discovery"
	"github.com/hyperledger/fabric/internal/pkg/identity"
	"github.com/hyperledger/fabric/internal/pkg/peer/orderers"
	"github.com/hyperledger/fabric/protoutil"
	"google.golang.org/grpc"
)

var (
	errClientClosing          = errors.New("client is closing")
	errClientReconnectTimeout = errors.New("client reconnect timeout")
	errNoBlockReceiver        = errors.New("no block receiver")
	errDuplicateBlock         = errors.New("duplicate block")
	errOutOfOrderBlock        = errors.New("out-of-order block")
	errChanBlockRecvIsClosed  = errors.New("channel block receiver is closed")
)

// LedgerInfo an adapter to provide the interface to query
// the ledger committer for current ledger height
//go:generate counterfeiter -o fake/ledger_info.go --fake-name LedgerInfo . LedgerInfo
type LedgerInfo interface {
	// LedgerHeight returns current local ledger height
	LedgerHeight() (uint64, error)
}

// GossipServiceAdapter serves to provide basic functionality
// required from gossip service by delivery service
//go:generate counterfeiter -o fake/gossip_service_adapter.go --fake-name GossipServiceAdapter . GossipServiceAdapter
type GossipServiceAdapter interface {
	// PeersOfChannel returns slice with members of specified channel
	PeersOfChannel(id gossipcommon.ChannelID) []discovery.NetworkMember
	// AddPayload adds payload to the local state sync buffer
	AddPayload(chainID string, payload *gossip.Payload) error
	// Gossip the message across the peers
	Gossip(msg *gossip.GossipMessage)
}

//go:generate counterfeiter -o fake/block_verifier.go --fake-name BlockVerifier . BlockVerifier
type BlockVerifier interface {
	VerifyBlock(channelID gossipcommon.ChannelID, blockNum uint64, block *common.Block) error
	// VerifyHeader returns nil when the header matches the metadata signature, but it does not compute the
	// block.Data.Hash() and compare it to the block.Header.DataHash, or otherwise inspect the block.Data.
	// This is used when the orderer delivers a block with header & metadata only (i.e. block.Data==nil).
	// See: gossip/api/MessageCryptoService
	VerifyHeader(chainID string, signedBlock *common.Block) error
}

//go:generate counterfeiter -o fake/dialer.go --fake-name Dialer . Dialer
type Dialer interface {
	Dial(address string, rootCerts [][]byte) (*grpc.ClientConn, error)
}

//go:generate counterfeiter -o fake/deliver_streamer.go --fake-name DeliverStreamer . DeliverStreamer
type DeliverStreamer interface {
	Deliver(context.Context, *grpc.ClientConn) (orderer.AtomicBroadcast_DeliverClient, error)
}

type header struct {
	ud       *UnitDeliver
	ch       chan *common.Block
	number   uint64
	dataHash []byte
}

// Deliverer the actual implementation for BlocksProviderBFT interface
type Deliverer struct {
	Ctx             context.Context
	Cancel          context.CancelFunc
	ChainID         string // a.k.a. Channel ID
	Gossip          GossipServiceAdapter
	Ledger          LedgerInfo                // provides access to the ledger height
	BlockVerifier   BlockVerifier             // verifies headers
	Signer          identity.SignerSerializer // signer for blocks requester
	DeliverStreamer DeliverStreamer
	TLSCertHash     []byte // tlsCertHash
	Dialer          Dialer // broadcast client dialer
	Logger          *flogging.FabricLogger
	// The minimal time between connection retries. This interval grows exponentially until maxBackoffDelay, below.
	MinBackoffDelay time.Duration
	// The maximal time between connection retries.
	MaxBackoffDelay time.Duration
	// The block censorship timeout. A block censorship suspicion is declared if more than f header receivers are
	// ahead of the block receiver for a period larger than this timeout.
	BlockCensorshipTimeout time.Duration

	UpdateEndpointsCh  chan []*orderers.Endpoint // channel receives new endpoints
	Endpoints          []*orderers.Endpoint      // a set of endpoints
	blockReceiverIndex int                       // index of the current block receiver endpoint
	blockReceiver      *UnitDeliver
	chBlockReceiver    chan *common.Block

	nextBlockNumber     uint64    // next block number
	lastBlockTime       time.Time // last block time
	BlockGossipDisabled bool
	headerReceivers     map[string]*header

	startOnce sync.Once
	mutex     sync.RWMutex
}

// DeliverBlocks used to pull out blocks from the ordering service to
// distributed them across peers
func (d *Deliverer) DeliverBlocks() {
	if d.BlockGossipDisabled {
		d.Logger.Infof("Will pull blocks without forwarding them to remote peers via gossip")
	}

	d.headerReceivers = make(map[string]*header)

	var (
		verErrCounter uint64
		delay         time.Duration
	)

	defer d.Cancel()

	d.lastBlockTime = time.Now()

	for {
		select {
		case <-d.Ctx.Done():
			return
		default:
		}

		block, err := d.recv()
		if err != nil {
			d.Logger.Warningf("receive error: %v", err)
			delay, verErrCounter = d.computeBackOffDelay(verErrCounter)
			d.sleep(delay)
			continue
		}

		blockNum := block.Header.Number

		marshaledBlock, err := proto.Marshal(block)
		if err != nil {
			d.Logger.Errorf("error serializing block with sequence number %d, due to %v; disconnecting client from orderer.", blockNum, err)
			delay, verErrCounter = d.computeBackOffDelay(verErrCounter)
			d.sleep(delay)

			d.stopAndWaitReciever(d.blockReceiver, d.chBlockReceiver)
			d.blockReceiver = nil
			continue
		}

		verErrCounter = 0 // On a good block

		numberOfPeers := len(d.Gossip.PeersOfChannel(gossipcommon.ChannelID(d.ChainID)))
		// Create payload with a block received
		payload := createPayload(blockNum, marshaledBlock)
		// Use payload to create gossip message
		gossipMsg := createGossipMsg(d.ChainID, payload)

		d.Logger.Debugf("adding payload to local buffer, blockNum = [%d]", blockNum)
		// Add payload to local state payloads buffer
		if err = d.Gossip.AddPayload(d.ChainID, payload); err != nil {
			d.Logger.Warningf("block [%d] received from ordering service wasn't added to payload buffer: %v", blockNum, err)
			return
		}

		d.Logger.Infof("received blockNumber=%d", blockNum)
		d.nextBlockNumber = blockNum + 1
		d.lastBlockTime = time.Now()

		if d.BlockGossipDisabled {
			continue
		}

		// Gossip messages with other nodes
		d.Logger.Debugf("gossiping block [%d], peers number [%d]", blockNum, numberOfPeers)
		select {
		case <-d.Ctx.Done():
			return
		default:
			d.Gossip.Gossip(gossipMsg)
		}
	}
}

// Stop stops blocks delivery provider
func (d *Deliverer) Stop() {
	d.Cancel()
}

func (d *Deliverer) stopAndWaitReciever(recv *UnitDeliver, chClose chan *common.Block) {
	recv.Stop()
	for range chClose {
	}
}

func (d *Deliverer) sleep(dt time.Duration) {
	select {
	case <-d.Ctx.Done():
	case <-time.After(dt):
	}
}

func (d *Deliverer) recv() (block *common.Block, err error) {
	d.Logger.Debugf("entry")

	d.startOnce.Do(func() {
		d.Logger.Debugf("starting update endpoints routine; current num of endpoints: %d", len(d.Endpoints))
		go d.updateEndpoints()
	})

	var (
		verErrCounter uint64
		delay         time.Duration
	)

OuterLoop:
	for {
		select {
		case <-d.Ctx.Done():
			break OuterLoop
		default:
		}

		if _, err = d.assignReceivers(); err != nil {
			d.Logger.Debugf("exit: error=%v", err)
			return nil, err
		}

		d.launchHeaderReceivers()

		block, err = d.receiveBlock()
		if err == nil {
			d.Logger.Debugf("exit: response=%v", block)
			return block, nil // the normal return path
		}

		if errors.Is(err, errClientReconnectTimeout) {
			d.closeBlockReceiver(true)
		} else {
			d.closeBlockReceiver(false)
		}

		delay, verErrCounter = d.computeBackOffDelay(verErrCounter)
		d.sleep(delay)
	}

	d.Logger.Debugf("exit: %v", errClientClosing)
	return nil, errClientClosing
}

// (re)-assign a block delivery client and header delivery clients
func (d *Deliverer) assignReceivers() (int, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	numEP := len(d.Endpoints)
	if numEP <= 0 {
		d.Cancel()
		return numEP, errors.New("no endpoints")
	}

	ledgerHeight, err := d.Ledger.LedgerHeight()
	if err != nil {
		d.Logger.Error("Did not return ledger height, something is critically wrong", err)
		d.Cancel()
		return numEP, err
	}

	if ledgerHeight > d.nextBlockNumber {
		d.nextBlockNumber = ledgerHeight
	}

	if d.blockReceiver == nil {
		seekInfoEnv, err := d.createSeekInfo(d.nextBlockNumber, false)
		if err != nil {
			d.Logger.Error("Could not create a signed Deliver SeekInfo message, something is critically wrong", err)
			d.Cancel()

			return numEP, err
		}

		d.blockReceiverIndex = (d.blockReceiverIndex + 1) % numEP
		ep := d.Endpoints[d.blockReceiverIndex]
		if headerReceiver, exists := d.headerReceivers[ep.Address]; exists {
			d.stopAndWaitReciever(headerReceiver.ud, headerReceiver.ch)
			delete(d.headerReceivers, ep.Address)
			d.Logger.Debugf("closed header receiver to: %s", ep.Address)
		}

		d.chBlockReceiver = make(chan *common.Block)

		d.blockReceiver = NewUnitDeliver(
			d.Ctx,
			d.ChainID,
			d.Signer,
			d.DeliverStreamer,
			d.TLSCertHash,
			ep,
			d.Dialer,
			d.Ledger,
			d.BlockVerifier,
			d.MaxBackoffDelay,
			d.MinBackoffDelay,
			false,
			d.workBlockReceiver(d.chBlockReceiver),
			d.endBlockReceiver(d.chBlockReceiver),
			seekInfoEnv,
		)

		go d.blockReceiver.DeliverBlocks()

		d.Logger.Debugf("created block receiver to: %s", ep.Address)
	}

	hRcvToCreate := make([]*orderers.Endpoint, 0)
	for i, ep := range d.Endpoints {
		if i == d.blockReceiverIndex {
			continue
		}

		if hRcv, exists := d.headerReceivers[ep.Address]; exists {
			if hRcv.ud.IsStopped() {
				delete(d.headerReceivers, ep.Address)
			} else {
				continue
			}
		}

		hRcvToCreate = append(hRcvToCreate, ep)
	}

	seekInfoEnv, err := d.createSeekInfo(d.nextBlockNumber, true)
	if err != nil {
		d.Logger.Error("Could not create a signed Deliver SeekInfo message, something is critically wrong", err)
		d.Cancel()

		return numEP, err
	}

	for _, ep := range hRcvToCreate {
		ch := make(chan *common.Block, 10)

		headerReceiver := NewUnitDeliver(
			d.Ctx,
			d.ChainID,
			d.Signer,
			d.DeliverStreamer,
			d.TLSCertHash,
			ep,
			d.Dialer,
			d.Ledger,
			d.BlockVerifier,
			d.MaxBackoffDelay,
			d.MinBackoffDelay,
			true,
			d.workHeadReceiver(ch),
			d.endBlockReceiver(ch),
			seekInfoEnv,
		)

		d.headerReceivers[ep.Address] = &header{
			ud: headerReceiver,
			ch: ch,
		}

		d.Logger.Debugf("created header receiver to: %s", ep.Address)
	}

	d.Logger.Debugf("exit: number of endpoints: %d", numEP)
	return numEP, nil
}

func (d *Deliverer) createSeekInfo(ledgerHeight uint64, workHeader bool) (*common.Envelope, error) {
	ct := orderer.SeekInfo_BLOCK

	if workHeader {
		ct = orderer.SeekInfo_HEADER_WITH_SIG
	}

	return protoutil.CreateSignedEnvelopeWithTLSBinding(
		common.HeaderType_DELIVER_SEEK_INFO,
		d.ChainID,
		d.Signer,
		&orderer.SeekInfo{
			Start: &orderer.SeekPosition{
				Type: &orderer.SeekPosition_Specified{
					Specified: &orderer.SeekSpecified{
						Number: ledgerHeight,
					},
				},
			},
			Stop: &orderer.SeekPosition{
				Type: &orderer.SeekPosition_Specified{
					Specified: &orderer.SeekSpecified{
						Number: math.MaxUint64,
					},
				},
			},
			Behavior:    orderer.SeekInfo_BLOCK_UNTIL_READY,
			ContentType: ct,
		},
		int32(0),
		uint64(0),
		d.TLSCertHash,
	)
}

func (d *Deliverer) launchHeaderReceivers() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	var launched int
	for ep, hRcv := range d.headerReceivers {
		if hRcv.ud.IsStopped() {
			d.Logger.Debugf("launching a header receiver to endpoint: %s", ep)
			launched++
			go hRcv.ud.DeliverBlocks()
		}
	}

	d.Logger.Debugf("header receivers: launched=%d, total running=%d ", launched, len(d.headerReceivers))
}

func (d *Deliverer) receiveBlock() (*common.Block, error) {
	d.mutex.Lock()
	receiver := d.blockReceiver
	d.mutex.Unlock()

	if receiver == nil {
		return nil, errNoBlockReceiver
	}

	addr := receiver.GetEndpoint()

	t := time.Until(d.lastBlockTime.Add(d.BlockCensorshipTimeout))
	if t < d.BlockCensorshipTimeout/100 {
		t = d.BlockCensorshipTimeout / 100
	}

	select {
	case <-d.Ctx.Done():
		return nil, errClientClosing
	case <-time.After(t):
		if d.collectDataFromHeaders() {
			return nil, errClientReconnectTimeout
		}
		return nil, errNoBlockReceiver
	case block, ok := <-d.chBlockReceiver:
		if !ok {
			d.Logger.Warnf("channel block receiver is closed: %s", addr)
			return nil, errChanBlockRecvIsClosed
		}
		if block.Header.Number > d.nextBlockNumber {
			d.Logger.Warnf("ignoring out-of-order block from orderer: %s; received block number: %d, expected: %d",
				addr, block.Header.Number, d.nextBlockNumber)
			return nil, errOutOfOrderBlock
		}
		if block.Header.Number < d.nextBlockNumber {
			d.Logger.Warnf("ignoring duplicate block from orderer: %s; received block number: %d, expected: %d",
				addr, block.Header.Number, d.nextBlockNumber)
			return nil, errDuplicateBlock
		}
		d.Logger.Debugf("received block from orderer: %s; received block number: %d",
			addr, block.Header.Number)

		ticker := time.NewTicker(d.BlockCensorshipTimeout / 100)
		defer ticker.Stop()

		for range ticker.C {
			select {
			case <-d.Ctx.Done():
				ticker.Stop()
				return nil, errClientClosing
			default:
			}

			if d.collectDataFromHeaders() && d.checkDataHashFrom(block.Header.DataHash) {
				break
			}

			if d.lastBlockTime.Add(d.BlockCensorshipTimeout).Before(time.Now()) {
				return nil, errClientReconnectTimeout
			}
		}

		return block, nil
	}
}

func (d *Deliverer) collectDataFromHeaders() bool {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	var numAhead int
	for _, hRcv := range d.headerReceivers {
		if hRcv.number == d.nextBlockNumber {
			numAhead++
		}

		if hRcv.number >= d.nextBlockNumber {
			continue
		}

	OuterLoop:
		for {
			select {
			case h := <-hRcv.ch:
				hRcv.number = h.Header.Number
				hRcv.dataHash = h.Header.DataHash

				if hRcv.number == d.nextBlockNumber {
					numAhead++
				}

				if hRcv.number >= d.nextBlockNumber {
					break OuterLoop
				}
			default:
				break OuterLoop
			}
		}
	}

	numEP := uint64(len(d.Endpoints))
	_, f := computeQuorum(numEP)
	if numAhead > f {
		d.Logger.Warnf("suspected block censorship: %d header receivers are ahead of block receiver, out of %d endpoints",
			numAhead, numEP)
		return true
	}

	return false
}

func (d *Deliverer) checkDataHashFrom(hash []byte) bool {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	var numAhead int

	numEP := uint64(len(d.Endpoints))
	_, f := computeQuorum(numEP)

	for _, hRcv := range d.headerReceivers {
		if hRcv.number != d.nextBlockNumber {
			continue
		}

		if !bytes.Equal(hRcv.dataHash, hash) {
			continue
		}

		numAhead++

		if numAhead > f {
			d.Logger.Warnf("suspected block censorship: %d header receivers are ahead of block receiver, out of %d endpoints",
				numAhead, numEP)
			return true
		}
	}

	return false
}

func (d *Deliverer) closeBlockReceiver(updateLastBlockTime bool) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if updateLastBlockTime {
		d.lastBlockTime = time.Now()
	}

	if d.blockReceiver != nil {
		d.stopAndWaitReciever(d.blockReceiver, d.chBlockReceiver)
		d.blockReceiver = nil
	}
}

func computeQuorum(n uint64) (q int, f int) {
	f = (int(n) - 1) / 3
	q = int(math.Ceil((float64(n) + float64(f) + 1) / 2.0))
	return
}

func (d *Deliverer) workBlockReceiver(ch chan *common.Block) func(ctx context.Context, block *common.Block) {
	return func(ctx context.Context, block *common.Block) {
		select {
		case <-ctx.Done():
		case ch <- block:
		}
	}
}

func (d *Deliverer) endBlockReceiver(ch chan *common.Block) func() {
	return func() {
		close(ch)
	}
}

func (d *Deliverer) workHeadReceiver(ch chan *common.Block) func(ctx context.Context, block *common.Block) {
	return func(ctx context.Context, block *common.Block) {
		select {
		case <-ctx.Done():
		case ch <- block:
		}
	}
}

// computeBackOffDelay computes an exponential back-off delay and increments the counter,
// as long as the computed delay is below the maximal delay.
func (d *Deliverer) computeBackOffDelay(count uint64) (time.Duration, uint64) {
	delay := computeBackoff(int(count), d.MinBackoffDelay, d.MaxBackoffDelay)

	return delay, count + 1
}

func createPayload(seqNum uint64, marshaledBlock []byte) *gossip.Payload {
	return &gossip.Payload{
		Data:   marshaledBlock,
		SeqNum: seqNum,
	}
}

func createGossipMsg(chainID string, payload *gossip.Payload) *gossip.GossipMessage {
	gossipMsg := &gossip.GossipMessage{
		Nonce:   0,
		Tag:     gossip.GossipMessage_CHAN_AND_ORG,
		Channel: []byte(chainID),
		Content: &gossip.GossipMessage_DataMsg{
			DataMsg: &gossip.DataMessage{
				Payload: payload,
			},
		},
	}
	return gossipMsg
}

// Check endpoints changes.
func (d *Deliverer) updateEndpoints() {
	d.Logger.Debugf("entry")

OuterLoop:
	for {
		select {
		case endpoints := <-d.UpdateEndpointsCh:
			d.Logger.Debugf("received endpoints: %d", len(endpoints))
			d.UpdateEndpoints(endpoints)
		case <-d.Ctx.Done():
			break OuterLoop
		}
	}

	d.Logger.Debugf("exit")
}

// UpdateEndpoints assigns the new endpoints for the delivery client
func (d *Deliverer) UpdateEndpoints(endpoints []*orderers.Endpoint) {
	select {
	case <-d.Ctx.Done():
		return
	default:
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()

	if equalEndpoints(d.Endpoints, endpoints) {
		return
	}

	d.Logger.Debugf("updating endpoints: existing: %v, new: %v", d.Endpoints, endpoints)
	d.Endpoints = endpoints
	d.blockReceiverIndex = 0
	d.disconnectAll()
}

func (d *Deliverer) disconnectAll() {
	if d.blockReceiver != nil {
		d.stopAndWaitReciever(d.blockReceiver, d.chBlockReceiver)
		d.blockReceiver = nil
		d.Logger.Debug("closed block receiver")
	}

	for ep, hRcv := range d.headerReceivers {
		d.stopAndWaitReciever(hRcv.ud, hRcv.ch)
		d.Logger.Debugf("closed header receiver to: %s", ep)
		delete(d.headerReceivers, ep)
	}
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

func Shuffle(a []*orderers.Endpoint) []*orderers.Endpoint {
	n := len(a)
	returnedSlice := make([]*orderers.Endpoint, n)
	rand.Seed(time.Now().UnixNano())
	indices := rand.Perm(n)
	for i, idx := range indices {
		returnedSlice[i] = a[idx]
	}
	return returnedSlice
}

func computeBackoff(
	retries int,
	baseDelay time.Duration,
	maxDelay time.Duration,
) time.Duration {
	if retries == 0 {
		return baseDelay
	}

	jitter := 0.2
	multiplier := 1.6

	backoff, max := float64(baseDelay), float64(maxDelay)
	for backoff < max && retries > 0 {
		backoff *= multiplier
		retries--
	}
	if backoff > max {
		backoff = max
	}
	// Randomize backoff delays so that if a cluster of requests start at
	// the same time, they won't operate in lockstep.
	backoff *= 1 + jitter*(rand.New(rand.NewSource(time.Now().UnixNano())).Float64()*2-1)
	if backoff < 0 {
		return 0
	}
	return time.Duration(backoff)
}
