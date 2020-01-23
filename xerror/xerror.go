package xerror

import (
	"context"
	"fmt"
	"reflect"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/EMayej/pkg/xlog"
)

// Common errors could be used by other package, if any error is derived from
// below errors, then there is no need to provide them in errorMapping of
// ErrorToCode
var (
	ErrArgs     = fmt.Errorf("invalid arguments")
	ErrInternal = fmt.Errorf("internal error")
)

var errorMappingBuiltin = map[error]int32{
	ErrArgs:     1,
	ErrInternal: 2,
}

type causer interface {
	Cause() error
}

// ErrorToCode tries to unwrap error from errors.Wrap and returns the
// corresponding code pre-defined in errorMapping
func ErrorToCode(errorMapping map[error]int32, err error) int32 {
	for {
		if code, ok := errorMapping[err]; ok {
			return code
		}

		if code, ok := errorMappingBuiltin[err]; ok {
			return code
		}

		// next level
		cause, ok := err.(causer)
		if !ok {
			break
		}
		err = cause.Cause()
	}

	return 2
}

// NewErrorInterceptor tries to do two things
// 1. Recover panic and returns error
// 2. Prevent other error from leaking to client and set return code accordingly
func NewErrorInterceptor(errorMapping map[error]int32) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		request interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (response interface{}, err error) {

		// Recover panic
		defer func() {
			if r := recover(); r != nil {
				xlog.Logger(ctx).Error(
					"panic",
					zap.Any("recover", r),
					zap.String("full_method", info.FullMethod),
					zap.Any("request", request),
					zap.Stack("stack"),
				)

				// We can't call setErrorCode on response cause
				// it is a nil interface when we get here. So we
				// return a non-nil error and hope our client
				// will properly handle it.
				//
				// To understand this error, one client could
				// use below code:
				//
				// import "google.golang.org/grpc/codes"
				// import "google.golang.org/grpc/status"
				// errStatus, _ := status.FromError(err)
				// if errStatus.Code() == codes.Internal {
				// 	fmt.Println(errStatus.Code())
				// 	fmt.Println(errStatus.Message())
				// }
				err = status.Error(codes.Internal, "panic")
			}
		}()
		response, err = handler(ctx, request)

		// set code of response instead of returning error
		if err != nil {
			xlog.Logger(ctx).Error(
				"handler fails",
				zap.Error(err),
				zap.String("full_method", info.FullMethod),
				zap.Any("request", request),
			)
			if setErrorCode(response, ErrorToCode(errorMapping, err)) {
				err = nil
			}
		}

		return
	}
}

// setErrorCode tries to set pbhttpproxy.CodeEnum in response. It returns true
// if the code is been correctly set. Otherwise it returns false.
func setErrorCode(response interface{}, code int32) bool {
	if response == nil {
		return false
	}

	s := reflect.ValueOf(response).Elem()
	if s.Kind() != reflect.Struct {
		return false
	}

	f := s.FieldByName("Code")
	if !f.IsValid() || !f.CanSet() {
		return false
	}

	if f.Kind() != reflect.Int32 {
		return false
	}

	f.SetInt(int64(code))
	return true
}
