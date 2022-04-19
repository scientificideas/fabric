// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import (
	client "github.com/hyperledger/fabric/discovery/client"

	discovery "github.com/hyperledger/fabric-protos-go/discovery"

	mock "github.com/stretchr/testify/mock"

	peer "github.com/hyperledger/fabric-protos-go/peer"
)

// ChannelResponse is an autogenerated mock type for the ChannelResponse type
type ChannelResponse struct {
	mock.Mock
}

// Config provides a mock function with given fields:
func (_m *ChannelResponse) Config() (*discovery.ConfigResult, error) {
	ret := _m.Called()

	var r0 *discovery.ConfigResult
	if rf, ok := ret.Get(0).(func() *discovery.ConfigResult); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*discovery.ConfigResult)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Endorsers provides a mock function with given fields: invocationChain, f
func (_m *ChannelResponse) Endorsers(invocationChain client.InvocationChain, f client.Filter) (client.Endorsers, error) {
	ret := _m.Called(invocationChain, f)

	var r0 client.Endorsers
	if rf, ok := ret.Get(0).(func(client.InvocationChain, client.Filter) client.Endorsers); ok {
		r0 = rf(invocationChain, f)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(client.Endorsers)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(client.InvocationChain, client.Filter) error); ok {
		r1 = rf(invocationChain, f)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Peers provides a mock function with given fields: invocationChain
func (_m *ChannelResponse) Peers(invocationChain ...*peer.ChaincodeCall) ([]*client.Peer, error) {
	_va := make([]interface{}, len(invocationChain))
	for _i := range invocationChain {
		_va[_i] = invocationChain[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 []*client.Peer
	if rf, ok := ret.Get(0).(func(...*peer.ChaincodeCall) []*client.Peer); ok {
		r0 = rf(invocationChain...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*client.Peer)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(...*peer.ChaincodeCall) error); ok {
		r1 = rf(invocationChain...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
