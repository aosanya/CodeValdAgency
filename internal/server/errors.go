// Package server implements the AgencyService gRPC handler.
package server

import (
	"errors"

	codevaldagency "github.com/aosanya/CodeValdAgency"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// toGRPCError maps CodeValdAgency domain errors to the appropriate gRPC status.
// Unknown errors are wrapped as codes.Internal.
func toGRPCError(err error) error {
	switch {
	case errors.Is(err, codevaldagency.ErrAgencyNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, codevaldagency.ErrPublicationNotFound):
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

// toEntityGRPCError maps entitygraph errors to the appropriate gRPC status.
// Unknown errors are wrapped as codes.Internal.
func toEntityGRPCError(err error) error {
	switch {
	case errors.Is(err, entitygraph.ErrEntityNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, entitygraph.ErrEntityAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, entitygraph.ErrRelationshipNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, entitygraph.ErrImmutableType):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, entitygraph.ErrInvalidRelationship):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, entitygraph.ErrRelationshipCardinalityViolation):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, entitygraph.ErrRequiredRelationshipViolation):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Errorf(codes.Internal, "internal error: %v", err)
	}
}
