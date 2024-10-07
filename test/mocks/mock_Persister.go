// Code generated by mockery v2.45.0. DO NOT EDIT.

package mocks

import (
	context "context"

	backend "github.com/argoproj-labs/ephemeral-access/internal/backend"

	mock "github.com/stretchr/testify/mock"

	v1alpha1 "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
)

// MockPersister is an autogenerated mock type for the Persister type
type MockPersister struct {
	mock.Mock
}

type MockPersister_Expecter struct {
	mock *mock.Mock
}

func (_m *MockPersister) EXPECT() *MockPersister_Expecter {
	return &MockPersister_Expecter{mock: &_m.Mock}
}

// CreateAccessRequest provides a mock function with given fields: ctx, ar
func (_m *MockPersister) CreateAccessRequest(ctx context.Context, ar *v1alpha1.AccessRequest) (*v1alpha1.AccessRequest, error) {
	ret := _m.Called(ctx, ar)

	if len(ret) == 0 {
		panic("no return value specified for CreateAccessRequest")
	}

	var r0 *v1alpha1.AccessRequest
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.AccessRequest) (*v1alpha1.AccessRequest, error)); ok {
		return rf(ctx, ar)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.AccessRequest) *v1alpha1.AccessRequest); ok {
		r0 = rf(ctx, ar)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.AccessRequest)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1alpha1.AccessRequest) error); ok {
		r1 = rf(ctx, ar)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockPersister_CreateAccessRequest_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreateAccessRequest'
type MockPersister_CreateAccessRequest_Call struct {
	*mock.Call
}

// CreateAccessRequest is a helper method to define mock.On call
//   - ctx context.Context
//   - ar *v1alpha1.AccessRequest
func (_e *MockPersister_Expecter) CreateAccessRequest(ctx interface{}, ar interface{}) *MockPersister_CreateAccessRequest_Call {
	return &MockPersister_CreateAccessRequest_Call{Call: _e.mock.On("CreateAccessRequest", ctx, ar)}
}

func (_c *MockPersister_CreateAccessRequest_Call) Run(run func(ctx context.Context, ar *v1alpha1.AccessRequest)) *MockPersister_CreateAccessRequest_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*v1alpha1.AccessRequest))
	})
	return _c
}

func (_c *MockPersister_CreateAccessRequest_Call) Return(_a0 *v1alpha1.AccessRequest, _a1 error) *MockPersister_CreateAccessRequest_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockPersister_CreateAccessRequest_Call) RunAndReturn(run func(context.Context, *v1alpha1.AccessRequest) (*v1alpha1.AccessRequest, error)) *MockPersister_CreateAccessRequest_Call {
	_c.Call.Return(run)
	return _c
}

// ListAccessBindings provides a mock function with given fields: ctx, roleName, namespace
func (_m *MockPersister) ListAccessBindings(ctx context.Context, roleName string, namespace string) (*v1alpha1.AccessBindingList, error) {
	ret := _m.Called(ctx, roleName, namespace)

	if len(ret) == 0 {
		panic("no return value specified for ListAccessBindings")
	}

	var r0 *v1alpha1.AccessBindingList
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*v1alpha1.AccessBindingList, error)); ok {
		return rf(ctx, roleName, namespace)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *v1alpha1.AccessBindingList); ok {
		r0 = rf(ctx, roleName, namespace)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.AccessBindingList)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, roleName, namespace)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockPersister_ListAccessBindings_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ListAccessBindings'
type MockPersister_ListAccessBindings_Call struct {
	*mock.Call
}

// ListAccessBindings is a helper method to define mock.On call
//   - ctx context.Context
//   - roleName string
//   - namespace string
func (_e *MockPersister_Expecter) ListAccessBindings(ctx interface{}, roleName interface{}, namespace interface{}) *MockPersister_ListAccessBindings_Call {
	return &MockPersister_ListAccessBindings_Call{Call: _e.mock.On("ListAccessBindings", ctx, roleName, namespace)}
}

func (_c *MockPersister_ListAccessBindings_Call) Run(run func(ctx context.Context, roleName string, namespace string)) *MockPersister_ListAccessBindings_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *MockPersister_ListAccessBindings_Call) Return(_a0 *v1alpha1.AccessBindingList, _a1 error) *MockPersister_ListAccessBindings_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockPersister_ListAccessBindings_Call) RunAndReturn(run func(context.Context, string, string) (*v1alpha1.AccessBindingList, error)) *MockPersister_ListAccessBindings_Call {
	_c.Call.Return(run)
	return _c
}

// ListAccessRequests provides a mock function with given fields: ctx, key
func (_m *MockPersister) ListAccessRequests(ctx context.Context, key *backend.AccessRequestKey) (*v1alpha1.AccessRequestList, error) {
	ret := _m.Called(ctx, key)

	if len(ret) == 0 {
		panic("no return value specified for ListAccessRequests")
	}

	var r0 *v1alpha1.AccessRequestList
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *backend.AccessRequestKey) (*v1alpha1.AccessRequestList, error)); ok {
		return rf(ctx, key)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *backend.AccessRequestKey) *v1alpha1.AccessRequestList); ok {
		r0 = rf(ctx, key)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.AccessRequestList)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *backend.AccessRequestKey) error); ok {
		r1 = rf(ctx, key)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockPersister_ListAccessRequests_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ListAccessRequests'
type MockPersister_ListAccessRequests_Call struct {
	*mock.Call
}

// ListAccessRequests is a helper method to define mock.On call
//   - ctx context.Context
//   - key *backend.AccessRequestKey
func (_e *MockPersister_Expecter) ListAccessRequests(ctx interface{}, key interface{}) *MockPersister_ListAccessRequests_Call {
	return &MockPersister_ListAccessRequests_Call{Call: _e.mock.On("ListAccessRequests", ctx, key)}
}

func (_c *MockPersister_ListAccessRequests_Call) Run(run func(ctx context.Context, key *backend.AccessRequestKey)) *MockPersister_ListAccessRequests_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*backend.AccessRequestKey))
	})
	return _c
}

func (_c *MockPersister_ListAccessRequests_Call) Return(_a0 *v1alpha1.AccessRequestList, _a1 error) *MockPersister_ListAccessRequests_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockPersister_ListAccessRequests_Call) RunAndReturn(run func(context.Context, *backend.AccessRequestKey) (*v1alpha1.AccessRequestList, error)) *MockPersister_ListAccessRequests_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockPersister creates a new instance of MockPersister. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockPersister(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockPersister {
	mock := &MockPersister{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
