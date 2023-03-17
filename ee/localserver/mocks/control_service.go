// Code generated by mockery v2.20.0. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"

// ControlService is an autogenerated mock type for the controlService type
type ControlService struct {
	mock.Mock
}

// Fetch provides a mock function with given fields:
func (_m *ControlService) Fetch() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewControlService interface {
	mock.TestingT
	Cleanup(func())
}

// NewControlService creates a new instance of ControlService. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewControlService(t mockConstructorTestingTNewControlService) *ControlService {
	mock := &ControlService{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}