// Code generated by MockGen. DO NOT EDIT.
// Source: internal/database/database.go
//
// Generated by this command:
//
//	mockgen -source=internal/database/database.go -destination=internal/database/database_mock.go
//

// Package mock_database is a generated GoMock package.
package database

import (
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
	gorm "gorm.io/gorm"
)

// MockSeeder is a mock of Seeder interface.
type MockSeeder struct {
	ctrl     *gomock.Controller
	recorder *MockSeederMockRecorder
}

// MockSeederMockRecorder is the mock recorder for MockSeeder.
type MockSeederMockRecorder struct {
	mock *MockSeeder
}

// NewMockSeeder creates a new mock instance.
func NewMockSeeder(ctrl *gomock.Controller) *MockSeeder {
	mock := &MockSeeder{ctrl: ctrl}
	mock.recorder = &MockSeederMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSeeder) EXPECT() *MockSeederMockRecorder {
	return m.recorder
}

// Count mocks base method.
func (m *MockSeeder) Count(arg0 *gorm.DB) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Count", arg0)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Count indicates an expected call of Count.
func (mr *MockSeederMockRecorder) Count(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Count", reflect.TypeOf((*MockSeeder)(nil).Count), arg0)
}

// Seed mocks base method.
func (m *MockSeeder) Seed(arg0 *gorm.DB) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Seed", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Seed indicates an expected call of Seed.
func (mr *MockSeederMockRecorder) Seed(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Seed", reflect.TypeOf((*MockSeeder)(nil).Seed), arg0)
}
