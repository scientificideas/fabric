// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"

	smartbftprotos "github.com/hyperledger-labs/SmartBFT/smartbftprotos"
)

// MessageReceiver is an autogenerated mock type for the MessageReceiver type
type MessageReceiver struct {
	mock.Mock
}

// HandleMessage provides a mock function with given fields: sender, m
func (_m *MessageReceiver) HandleMessage(sender uint64, m *smartbftprotos.Message) {
	_m.Called(sender, m)
}

// HandleRequest provides a mock function with given fields: sender, req
func (_m *MessageReceiver) HandleRequest(sender uint64, req []byte) {
	_m.Called(sender, req)
}
