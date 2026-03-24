// Package server implements the AgencyService and EntityService gRPC handlers.
package server

import (
	"context"
	"errors"

	pb "github.com/aosanya/CodeValdAgency/gen/go/codevaldagency/v1"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// EntityServer implements pb.EntityServiceServer by delegating to an
// entitygraph.DataManager. Construct via NewEntityServer; register with
// pb.RegisterEntityServiceServer.
type EntityServer struct {
	pb.UnimplementedEntityServiceServer
	dm entitygraph.DataManager
}

// NewEntityServer constructs an EntityServer backed by the given DataManager.
func NewEntityServer(dm entitygraph.DataManager) *EntityServer {
	return &EntityServer{dm: dm}
}

// ListEntities implements pb.EntityServiceServer.
// type_id is injected by CodeValdCross via ConstantBinding at dispatch time.
func (s *EntityServer) ListEntities(ctx context.Context, req *pb.ListEntitiesRequest) (*pb.ListEntitiesResponse, error) {
	entities, err := s.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   req.GetAgencyId(),
		TypeID:     req.GetTypeId(),
		Properties: structToMap(req.GetProperties()),
	})
	if err != nil {
		return nil, toEntityGRPCError(err)
	}
	items := make([]*pb.EntityItem, 0, len(entities))
	for _, e := range entities {
		item, convErr := entityToProto(e)
		if convErr != nil {
			return nil, toEntityGRPCError(convErr)
		}
		items = append(items, item)
	}
	return &pb.ListEntitiesResponse{Entities: items}, nil
}

// CreateEntity implements pb.EntityServiceServer.
// type_id is injected by CodeValdCross via ConstantBinding at dispatch time.
// When the entity type declares a UniqueKey, UpsertEntity is called so that
// a POST with a matching code value merges onto the existing entity instead
// of inserting a duplicate. Types without a UniqueKey fall back to a plain
// CreateEntity (immutable types, Agency, etc.).
func (s *EntityServer) CreateEntity(ctx context.Context, req *pb.CreateEntityRequest) (*pb.EntityItem, error) {
	props := structToMap(req.GetProperties())
	createReq := entitygraph.CreateEntityRequest{
		AgencyID:   req.GetAgencyId(),
		TypeID:     req.GetTypeId(),
		Properties: props,
	}
	entity, err := s.dm.UpsertEntity(ctx, createReq)
	if err != nil {
		if !errors.Is(err, entitygraph.ErrUniqueKeyNotDefined) {
			return nil, toEntityGRPCError(err)
		}
		// Type has no UniqueKey — fall back to a plain insert.
		entity, err = s.dm.CreateEntity(ctx, createReq)
		if err != nil {
			return nil, toEntityGRPCError(err)
		}
	}
	return entityToProto(entity)
}

// GetEntity implements pb.EntityServiceServer.
func (s *EntityServer) GetEntity(ctx context.Context, req *pb.GetEntityRequest) (*pb.EntityItem, error) {
	entity, err := s.dm.GetEntity(ctx, req.GetAgencyId(), req.GetEntityId())
	if err != nil {
		return nil, toEntityGRPCError(err)
	}
	return entityToProto(entity)
}

// UpdateEntity implements pb.EntityServiceServer.
// type_id is injected by CodeValdCross via ConstantBinding but not used by the
// DataManager (entity is located by ID).
func (s *EntityServer) UpdateEntity(ctx context.Context, req *pb.UpdateEntityRequest) (*pb.EntityItem, error) {
	props := structToMap(req.GetProperties())
	entity, err := s.dm.UpdateEntity(ctx, req.GetAgencyId(), req.GetEntityId(), entitygraph.UpdateEntityRequest{
		Properties: props,
	})
	if err != nil {
		return nil, toEntityGRPCError(err)
	}
	return entityToProto(entity)
}

// DeleteEntity implements pb.EntityServiceServer.
func (s *EntityServer) DeleteEntity(ctx context.Context, req *pb.DeleteEntityRequest) (*pb.DeleteEntityResponse, error) {
	if err := s.dm.DeleteEntity(ctx, req.GetAgencyId(), req.GetEntityId()); err != nil {
		return nil, toEntityGRPCError(err)
	}
	return &pb.DeleteEntityResponse{}, nil
}

// ListRelationships implements pb.EntityServiceServer.
// name is injected by CodeValdCross via ConstantBinding at dispatch time.
func (s *EntityServer) ListRelationships(ctx context.Context, req *pb.ListRelationshipsRequest) (*pb.ListRelationshipsResponse, error) {
	rels, err := s.dm.ListRelationships(ctx, entitygraph.RelationshipFilter{
		AgencyID: req.GetAgencyId(),
		FromID:   req.GetEntityId(),
		Name:     req.GetName(),
	})
	if err != nil {
		return nil, toEntityGRPCError(err)
	}
	items := make([]*pb.RelationshipItem, 0, len(rels))
	for _, r := range rels {
		item, convErr := relationshipToProto(r)
		if convErr != nil {
			return nil, toEntityGRPCError(convErr)
		}
		items = append(items, item)
	}
	return &pb.ListRelationshipsResponse{Relationships: items}, nil
}

// CreateRelationship implements pb.EntityServiceServer.
// name is injected by CodeValdCross via ConstantBinding at dispatch time.
func (s *EntityServer) CreateRelationship(ctx context.Context, req *pb.CreateRelationshipRequest) (*pb.RelationshipItem, error) {
	props := structToMap(req.GetProperties())
	rel, err := s.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
		AgencyID:   req.GetAgencyId(),
		Name:       req.GetName(),
		FromID:     req.GetEntityId(),
		ToID:       req.GetToId(),
		Properties: props,
	})
	if err != nil {
		return nil, toEntityGRPCError(err)
	}
	return relationshipToProto(rel)
}

// DeleteRelationship implements pb.EntityServiceServer.
func (s *EntityServer) DeleteRelationship(ctx context.Context, req *pb.DeleteRelationshipRequest) (*pb.DeleteRelationshipResponse, error) {
	if err := s.dm.DeleteRelationship(ctx, req.GetAgencyId(), req.GetRelationshipId()); err != nil {
		return nil, toEntityGRPCError(err)
	}
	return &pb.DeleteRelationshipResponse{}, nil
}

// ── Conversion helpers ────────────────────────────────────────────────────────

// entityToProto converts an entitygraph.Entity to its proto representation.
func entityToProto(e entitygraph.Entity) (*pb.EntityItem, error) {
	protoProps, err := structpb.NewStruct(e.Properties)
	if err != nil {
		return nil, err
	}
	return &pb.EntityItem{
		Id:         e.ID,
		AgencyId:   e.AgencyID,
		TypeId:     e.TypeID,
		Properties: protoProps,
		CreatedAt:  timestamppb.New(e.CreatedAt),
		UpdatedAt:  timestamppb.New(e.UpdatedAt),
	}, nil
}

// relationshipToProto converts an entitygraph.Relationship to its proto
// representation.
func relationshipToProto(r entitygraph.Relationship) (*pb.RelationshipItem, error) {
	protoProps, err := structpb.NewStruct(r.Properties)
	if err != nil {
		return nil, err
	}
	return &pb.RelationshipItem{
		Id:         r.ID,
		AgencyId:   r.AgencyID,
		Name:       r.Name,
		FromId:     r.FromID,
		ToId:       r.ToID,
		Properties: protoProps,
		CreatedAt:  timestamppb.New(r.CreatedAt),
	}, nil
}

// structToMap converts a proto Struct to map[string]any.
// A nil Struct is returned as a nil map.
func structToMap(s *structpb.Struct) map[string]any {
	if s == nil {
		return nil
	}
	return s.AsMap()
}
