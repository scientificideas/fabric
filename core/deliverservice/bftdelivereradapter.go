package deliverservice

import (
	"context"

	"github.com/hyperledger/fabric-protos-go/orderer"
	"google.golang.org/grpc"
)

type bftDeliverAdapter struct{}

// Deliver initialize deliver client
func (bftDeliverAdapter) Deliver(ctx context.Context, clientConn *grpc.ClientConn) (orderer.AtomicBroadcast_DeliverClient, error) {
	return orderer.NewAtomicBroadcastClient(clientConn).Deliver(ctx)
}
