// Code generated by mockery v2.45.0. DO NOT EDIT.

package mocks

import (
	context "context"

	client "sigs.k8s.io/controller-runtime/pkg/client"

	mock "github.com/stretchr/testify/mock"

	types "k8s.io/apimachinery/pkg/types"
)

// MockK8sClient is an autogenerated mock type for the K8sClient type
type MockK8sClient struct {
	mock.Mock
}

type MockK8sClient_Expecter struct {
	mock *mock.Mock
}

func (_m *MockK8sClient) EXPECT() *MockK8sClient_Expecter {
	return &MockK8sClient_Expecter{mock: &_m.Mock}
}

// Get provides a mock function with given fields: ctx, key, obj, opts
func (_m *MockK8sClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, key, obj)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for Get")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, types.NamespacedName, client.Object, ...client.GetOption) error); ok {
		r0 = rf(ctx, key, obj, opts...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockK8sClient_Get_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Get'
type MockK8sClient_Get_Call struct {
	*mock.Call
}

// Get is a helper method to define mock.On call
//   - ctx context.Context
//   - key types.NamespacedName
//   - obj client.Object
//   - opts ...client.GetOption
func (_e *MockK8sClient_Expecter) Get(ctx interface{}, key interface{}, obj interface{}, opts ...interface{}) *MockK8sClient_Get_Call {
	return &MockK8sClient_Get_Call{Call: _e.mock.On("Get",
		append([]interface{}{ctx, key, obj}, opts...)...)}
}

func (_c *MockK8sClient_Get_Call) Run(run func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption)) *MockK8sClient_Get_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]client.GetOption, len(args)-3)
		for i, a := range args[3:] {
			if a != nil {
				variadicArgs[i] = a.(client.GetOption)
			}
		}
		run(args[0].(context.Context), args[1].(types.NamespacedName), args[2].(client.Object), variadicArgs...)
	})
	return _c
}

func (_c *MockK8sClient_Get_Call) Return(_a0 error) *MockK8sClient_Get_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockK8sClient_Get_Call) RunAndReturn(run func(context.Context, types.NamespacedName, client.Object, ...client.GetOption) error) *MockK8sClient_Get_Call {
	_c.Call.Return(run)
	return _c
}

// Patch provides a mock function with given fields: ctx, obj, patch, opts
func (_m *MockK8sClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, obj, patch)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for Patch")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, client.Object, client.Patch, ...client.PatchOption) error); ok {
		r0 = rf(ctx, obj, patch, opts...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockK8sClient_Patch_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Patch'
type MockK8sClient_Patch_Call struct {
	*mock.Call
}

// Patch is a helper method to define mock.On call
//   - ctx context.Context
//   - obj client.Object
//   - patch client.Patch
//   - opts ...client.PatchOption
func (_e *MockK8sClient_Expecter) Patch(ctx interface{}, obj interface{}, patch interface{}, opts ...interface{}) *MockK8sClient_Patch_Call {
	return &MockK8sClient_Patch_Call{Call: _e.mock.On("Patch",
		append([]interface{}{ctx, obj, patch}, opts...)...)}
}

func (_c *MockK8sClient_Patch_Call) Run(run func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption)) *MockK8sClient_Patch_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]client.PatchOption, len(args)-3)
		for i, a := range args[3:] {
			if a != nil {
				variadicArgs[i] = a.(client.PatchOption)
			}
		}
		run(args[0].(context.Context), args[1].(client.Object), args[2].(client.Patch), variadicArgs...)
	})
	return _c
}

func (_c *MockK8sClient_Patch_Call) Return(_a0 error) *MockK8sClient_Patch_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockK8sClient_Patch_Call) RunAndReturn(run func(context.Context, client.Object, client.Patch, ...client.PatchOption) error) *MockK8sClient_Patch_Call {
	_c.Call.Return(run)
	return _c
}

// Status provides a mock function with given fields:
func (_m *MockK8sClient) Status() client.SubResourceWriter {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Status")
	}

	var r0 client.SubResourceWriter
	if rf, ok := ret.Get(0).(func() client.SubResourceWriter); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(client.SubResourceWriter)
		}
	}

	return r0
}

// MockK8sClient_Status_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Status'
type MockK8sClient_Status_Call struct {
	*mock.Call
}

// Status is a helper method to define mock.On call
func (_e *MockK8sClient_Expecter) Status() *MockK8sClient_Status_Call {
	return &MockK8sClient_Status_Call{Call: _e.mock.On("Status")}
}

func (_c *MockK8sClient_Status_Call) Run(run func()) *MockK8sClient_Status_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockK8sClient_Status_Call) Return(_a0 client.SubResourceWriter) *MockK8sClient_Status_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockK8sClient_Status_Call) RunAndReturn(run func() client.SubResourceWriter) *MockK8sClient_Status_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockK8sClient creates a new instance of MockK8sClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockK8sClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockK8sClient {
	mock := &MockK8sClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
