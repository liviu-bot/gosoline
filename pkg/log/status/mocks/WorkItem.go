// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"

// WorkItem is an autogenerated mock type for the WorkItem type
type WorkItem struct {
	mock.Mock
}

// Monitor provides a mock function with given fields: f
func (_m *WorkItem) Monitor(f func() error) func() error {
	ret := _m.Called(f)

	var r0 func() error
	if rf, ok := ret.Get(0).(func(func() error) func() error); ok {
		r0 = rf(f)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(func() error)
		}
	}

	return r0
}

// ReportDone provides a mock function with given fields:
func (_m *WorkItem) ReportDone() {
	_m.Called()
}

// ReportError provides a mock function with given fields: err
func (_m *WorkItem) ReportError(err error) {
	_m.Called(err)
}

// ReportProgress provides a mock function with given fields: step, progress
func (_m *WorkItem) ReportProgress(step int, progress float64) {
	_m.Called(step, progress)
}
