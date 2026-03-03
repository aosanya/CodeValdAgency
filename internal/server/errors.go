// Package server implements the AgencyService gRPC handler.
package server

import (
	"errors"

	codevaldagency "github.com/aosanya/CodeValdAgency"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// toGRPCError maps CodeValdAgency domain errors to the appropriate gRPC status.
// Unknown errors are wrapped as codes.Internal.
func toGRPCError(err error) error {
	switch {
	case errors.Is(err, codevaldagency.ErrAgencyNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, codevaldagency.ErrInvalidJSON):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, codevaldagency.ErrInvalidLifecycleTransition):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, codevaldagency.ErrInvalidAgency):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Errorf(codes.Internal, "internal error: %v", err)
	}
}
