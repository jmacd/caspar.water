// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/jmacd/caspar.water/measure/ph/atlas/internal/device (interfaces: I2C)
//
// Generated by this command:
//
//	mockgen -package mock . I2C
//

// Package mock is a generated GoMock package.
package mock

import (
	reflect "reflect"
	time "time"

	gomock "go.uber.org/mock/gomock"
)

// MockI2C is a mock of I2C interface.
type MockI2C struct {
	ctrl     *gomock.Controller
	recorder *MockI2CMockRecorder
}

// MockI2CMockRecorder is the mock recorder for MockI2C.
type MockI2CMockRecorder struct {
	mock *MockI2C
}

// NewMockI2C creates a new mock instance.
func NewMockI2C(ctrl *gomock.Controller) *MockI2C {
	mock := &MockI2C{ctrl: ctrl}
	mock.recorder = &MockI2CMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockI2C) EXPECT() *MockI2CMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockI2C) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockI2CMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockI2C)(nil).Close))
}

// Read mocks base method.
func (m *MockI2C) Read(arg0 []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Read", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Read indicates an expected call of Read.
func (mr *MockI2CMockRecorder) Read(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Read", reflect.TypeOf((*MockI2C)(nil).Read), arg0)
}

// Sleep mocks base method.
func (m *MockI2C) Sleep(arg0 time.Duration) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Sleep", arg0)
}

// Sleep indicates an expected call of Sleep.
func (mr *MockI2CMockRecorder) Sleep(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Sleep", reflect.TypeOf((*MockI2C)(nil).Sleep), arg0)
}

// Write mocks base method.
func (m *MockI2C) Write(arg0 []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Write", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Write indicates an expected call of Write.
func (mr *MockI2CMockRecorder) Write(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Write", reflect.TypeOf((*MockI2C)(nil).Write), arg0)
}
