// Code generated by mockery v2.44.1. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"

	time "time"
)

// DetectorIface is an autogenerated mock type for the detectorIface type
type DetectorIface struct {
	mock.Mock
}

// Detect provides a mock function with given fields: reason, timeout
func (_m *DetectorIface) Detect(reason string, timeout time.Duration) (bool, error) {
	ret := _m.Called(reason, timeout)

	if len(ret) == 0 {
		panic("no return value specified for Detect")
	}

	var r0 bool
	var r1 error
	if rf, ok := ret.Get(0).(func(string, time.Duration) (bool, error)); ok {
		return rf(reason, timeout)
	}
	if rf, ok := ret.Get(0).(func(string, time.Duration) bool); ok {
		r0 = rf(reason, timeout)
	} else {
		r0 = ret.Get(0).(bool)
	}

	if rf, ok := ret.Get(1).(func(string, time.Duration) error); ok {
		r1 = rf(reason, timeout)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewDetectorIface creates a new instance of DetectorIface. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewDetectorIface(t interface {
	mock.TestingT
	Cleanup(func())
}) *DetectorIface {
	mock := &DetectorIface{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
