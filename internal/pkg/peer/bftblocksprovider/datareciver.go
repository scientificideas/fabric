package bftblocksprovider

import (
	"context"
	"math"
	"time"

	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric/common/flogging"
	gossipcommon "github.com/hyperledger/fabric/gossip/common"
	"github.com/hyperledger/fabric/internal/pkg/identity"
	"github.com/hyperledger/fabric/internal/pkg/peer/orderers"
	"github.com/pkg/errors"
	"go.uber.org/atomic"
)

const backoffExponentBase = 1.2

type UnitDeliver struct {
	ctx           context.Context
	cancel        context.CancelFunc
	endpoint      *orderers.Endpoint
	channelID     string
	signer        identity.SignerSerializer
	deliverStream DeliverStreamer
	dialer        Dialer
	ledger        LedgerInfo
	blockVerifier BlockVerifier
	// TLSCertHash should be nil when TLS is not enabled
	tlsCertHash       []byte // util.ComputeSHA256(b.credSupport.GetClientCertificate().Certificate[0])
	maxRetryDelay     time.Duration
	initialRetryDelay time.Duration
	logger            *flogging.FabricLogger
	workHeader        bool
	workFunc          func(ctx context.Context, block *common.Block)
	endFunc           func()
	stop              atomic.Int32
	seekInfoEnv       *common.Envelope
}

func NewUnitDeliver(
	ctx context.Context,
	channelID string,
	signer identity.SignerSerializer,
	deliverStream DeliverStreamer,
	tlsCertHash []byte,
	endpoint *orderers.Endpoint,
	dialer Dialer,
	ledger LedgerInfo,
	blockVerifier BlockVerifier,
	maxRetryDelay time.Duration,
	initialRetryDelay time.Duration,
	workHeader bool,
	workFunc func(ctx context.Context, block *common.Block),
	endFunc func(),
	seekInfoEnv *common.Envelope,
) *UnitDeliver {
	ctx, cancel := context.WithCancel(ctx)

	u := &UnitDeliver{
		ctx:               ctx,
		cancel:            cancel,
		channelID:         channelID,
		signer:            signer,
		deliverStream:     deliverStream,
		tlsCertHash:       tlsCertHash,
		endpoint:          endpoint,
		dialer:            dialer,
		maxRetryDelay:     maxRetryDelay,
		initialRetryDelay: initialRetryDelay,
		ledger:            ledger,
		blockVerifier:     blockVerifier,
		workHeader:        workHeader,
		workFunc:          workFunc,
		endFunc:           endFunc,
		logger: flogging.MustGetLogger("peer.blocksproviderbft").
			With("channel", channelID, "orderer-address", endpoint.Address),
		seekInfoEnv: seekInfoEnv,
	}

	return u
}

// DeliverBlocks used to pull out head from the ordering service to
// distributed them across peers
func (u *UnitDeliver) DeliverBlocks() {
	u.stop.CAS(0, 1)
	defer u.stop.CAS(1, 0)

	defer func() {
		if u.endFunc != nil {
			u.endFunc()
		}
	}()

	failureCounter := 0

	// InitialRetryDelay * backoffExponentBase^n > MaxRetryDelay
	// backoffExponentBase^n > MaxRetryDelay / InitialRetryDelay
	// n * log(backoffExponentBase) > log(MaxRetryDelay / InitialRetryDelay)
	// n > log(MaxRetryDelay / InitialRetryDelay) / log(backoffExponentBase)
	maxFailures := int(math.Log(float64(u.maxRetryDelay)/float64(u.initialRetryDelay)) / math.Log(backoffExponentBase))

	for {
		select {
		case <-u.ctx.Done():
			return
		default:
		}

		if failureCounter > 0 {
			var sleepDuration time.Duration
			if failureCounter-1 > maxFailures {
				sleepDuration = u.maxRetryDelay
			} else {
				sleepDuration = time.Duration(math.Pow(1.2, float64(failureCounter-1))*100) * time.Millisecond
			}

			u.sleep(sleepDuration)
		}

		deliverClient, cancel, err := u.connect(u.seekInfoEnv)
		if err != nil {
			u.logger.Warningf("Could not connect to ordering service: %v", err)
			failureCounter++
			continue
		}

		recv := make(chan *orderer.DeliverResponse)
		go func() {
			for {
				resp, err := deliverClient.Recv()
				if err != nil {
					u.logger.Warningf("Encountered an error reading from deliver stream: %v", err)
					close(recv)
					return
				}

				select {
				case recv <- resp:
				case <-u.ctx.Done():
					close(recv)
					return
				}
			}
		}()

	RecvLoop: // Loop until the endpoint is refreshed, or there is an error on the connection
		for {
			select {
			case <-u.endpoint.Refreshed:
				u.logger.Infof("Ordering endpoints have been refreshed, disconnecting from deliver to reconnect using updated endpoints")
				break RecvLoop
			case response, ok := <-recv:
				if !ok {
					u.logger.Warningf("Orderer hung up without sending status")
					failureCounter++
					break RecvLoop
				}
				err = u.processMsg(response)
				if err != nil {
					u.logger.Warningf("Got error while attempting to receive blocks: %v", err)
					failureCounter++
					break RecvLoop
				}
				failureCounter = 0
			case <-u.ctx.Done():
				break RecvLoop
			}
		}

		// cancel and wait for our spawned go routine to exit
		cancel()
		<-recv
	}
}

func (u *UnitDeliver) processMsg(msg *orderer.DeliverResponse) error {
	switch t := msg.Type.(type) {
	case *orderer.DeliverResponse_Status:
		if t.Status == common.Status_SUCCESS {
			return errors.Errorf("received success for a seek that should never complete")
		}

		return errors.Errorf("received bad status %v from orderer", t.Status)
	case *orderer.DeliverResponse_Block:
		blockNum := t.Block.Header.Number

		if u.workHeader {
			if err := u.blockVerifier.VerifyHeader(
				u.channelID,
				t.Block,
			); err != nil {
				return errors.WithMessage(err, "block header from orderer could not be verified")
			}
		} else {
			if err := u.blockVerifier.VerifyBlock(
				gossipcommon.ChannelID(u.channelID),
				t.Block.Header.Number,
				t.Block,
			); err != nil {
				return errors.WithMessage(err, "block from orderer could not be verified")
			}
		}

		// do not verify, just save for later, in case the block-receiver is suspected of censorship
		u.logger.Debugf("saving block with header and metadata, blockNum = [%d], block = [%v]", blockNum, t.Block)

		u.workFunc(u.ctx, t.Block)

		return nil
	default:
		u.logger.Warningf("Received unknown: %v", t)
		return errors.Errorf("unknown message type '%T'", msg.Type)
	}
}

// Stop stops blocks delivery provider
func (u *UnitDeliver) Stop() {
	u.cancel()
}

func (u *UnitDeliver) IsStopped() bool {
	return u.stop.Load() == 0
}

func (u *UnitDeliver) GetEndpoint() string {
	return u.endpoint.Address
}

func (u *UnitDeliver) connect(seekInfoEnv *common.Envelope) (orderer.AtomicBroadcast_DeliverClient, func(), error) {
	conn, err := u.dialer.Dial(u.endpoint.Address, u.endpoint.RootCerts)
	if err != nil {
		return nil, nil, errors.WithMessagef(err, "could not dial endpoint '%s'", u.endpoint.Address)
	}

	ctx, ctxCancel := context.WithCancel(u.ctx)

	deliverClient, err := u.deliverStream.Deliver(ctx, conn)
	if err != nil {
		_ = conn.Close()
		ctxCancel()
		return nil, nil, errors.WithMessagef(err, "could not create deliver client to endpoints '%s'", u.endpoint.Address)
	}

	err = deliverClient.Send(seekInfoEnv)
	if err != nil {
		_ = deliverClient.CloseSend()
		_ = conn.Close()
		ctxCancel()
		return nil, nil, errors.WithMessagef(err, "could not send deliver seek info handshake to '%s'", u.endpoint.Address)
	}

	return deliverClient, func() {
		_ = deliverClient.CloseSend()
		ctxCancel()
		_ = conn.Close()
	}, nil
}

func (u *UnitDeliver) sleep(dt time.Duration) {
	select {
	case <-u.ctx.Done():
	case <-time.After(dt):
	}
}
