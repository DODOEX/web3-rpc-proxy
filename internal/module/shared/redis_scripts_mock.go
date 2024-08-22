// Code generated by MockGen. DO NOT EDIT.
// Source: ../../shared/redis_scripts.go
//
// Generated by this command:
//
//	mockgen -source=../../shared/redis_scripts.go -destination=../../shared/redis_scripts_mock.go -package=shared
//

// Package shared is a generated GoMock package.
package shared

import (
	context "context"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockScripts is a mock of Scripts interface.
type MockScripts struct {
	ctrl     *gomock.Controller
	recorder *MockScriptsMockRecorder
}

// MockScriptsMockRecorder is the mock recorder for MockScripts.
type MockScriptsMockRecorder struct {
	mock *MockScripts
}

// NewMockScripts creates a new mock instance.
func NewMockScripts(ctrl *gomock.Controller) *MockScripts {
	mock := &MockScripts{ctrl: ctrl}
	mock.recorder = &MockScriptsMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockScripts) EXPECT() *MockScriptsMockRecorder {
	return m.recorder
}

// Balance mocks base method.
func (m *MockScripts) Balance(ctx context.Context, key string, capacity, rate int64) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Balance", ctx, key, capacity, rate)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Balance indicates an expected call of Balance.
func (mr *MockScriptsMockRecorder) Balance(ctx, key, capacity, rate any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Balance", reflect.TypeOf((*MockScripts)(nil).Balance), ctx, key, capacity, rate)
}
