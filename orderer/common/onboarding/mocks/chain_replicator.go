// Code generated by mockery v2.10.4. DO NOT EDIT.

package mocks

import (
	common "github.com/hyperledger/fabric-protos-go/common"
	mock "github.com/stretchr/testify/mock"
)

// ChainReplicator is an autogenerated mock type for the ChainReplicator type
type ChainReplicator struct {
	mock.Mock
}

// ReplicateChains provides a mock function with given fields: lastConfigBlock, chains
func (_m *ChainReplicator) ReplicateChains(lastConfigBlock *common.Block, chains []string) []string {
	ret := _m.Called(lastConfigBlock, chains)

	var r0 []string
	if rf, ok := ret.Get(0).(func(*common.Block, []string) []string); ok {
		r0 = rf(lastConfigBlock, chains)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	return r0
}
