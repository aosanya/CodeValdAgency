// Package server implements the AgencyService gRPC handler.
// It wraps a codevaldagency.AgencyManager and translates between proto messages
// and domain types.
package server

import (
	"context"
	"time"

	codevaldagency "github.com/aosanya/CodeValdAgency"
	pb "github.com/aosanya/CodeValdAgency/gen/go/codevaldagency/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements pb.AgencyServiceServer by wrapping a codevaldagency.AgencyManager.
// Construct via New; register with grpc.Server using
// pb.RegisterAgencyServiceServer.
type Server struct {
	pb.UnimplementedAgencyServiceServer
	mgr codevaldagency.AgencyManager
}

// New constructs a Server backed by the given AgencyManager.
func New(mgr codevaldagency.AgencyManager) *Server {
	return &Server{mgr: mgr}
}

// GetAgency implements pb.AgencyServiceServer.
func (s *Server) GetAgency(ctx context.Context, _ *pb.GetAgencyRequest) (*pb.Agency, error) {
	agency, err := s.mgr.GetAgency(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return agencyToProto(agency), nil
}

// SetAgencyDetails implements pb.AgencyServiceServer.
func (s *Server) SetAgencyDetails(ctx context.Context, req *pb.SetAgencyDetailsRequest) (*pb.Agency, error) {
	agency, err := s.mgr.SetAgencyDetails(ctx, req.GetJson())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return agencyToProto(agency), nil
}

// PublishAgency implements pb.AgencyServiceServer.
func (s *Server) PublishAgency(ctx context.Context, _ *pb.PublishAgencyRequest) (*pb.AgencyPublication, error) {
	pub, err := s.mgr.PublishAgency(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return publicationToProto(pub), nil
}

// GetPublication implements pb.AgencyServiceServer.
func (s *Server) GetPublication(ctx context.Context, req *pb.GetPublicationRequest) (*pb.AgencyPublication, error) {
	pub, err := s.mgr.GetPublication(ctx, int(req.GetVersion()))
	if err != nil {
		return nil, toGRPCError(err)
	}
	return publicationToProto(pub), nil
}

// ListPublications implements pb.AgencyServiceServer.
func (s *Server) ListPublications(ctx context.Context, _ *pb.ListPublicationsRequest) (*pb.ListPublicationsResponse, error) {
	pubs, err := s.mgr.ListPublications(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}
	out := make([]*pb.AgencyPublication, len(pubs))
	for i, p := range pubs {
		out[i] = publicationToProto(p)
	}
	return &pb.ListPublicationsResponse{Publications: out}, nil
}

// GetGoals implements pb.AgencyServiceServer.
func (s *Server) GetGoals(ctx context.Context, _ *pb.GetGoalsRequest) (*pb.GetGoalsResponse, error) {
	goals, err := s.mgr.GetGoals(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.GetGoalsResponse{Goals: goalsToProto(goals)}, nil
}

// GetWorkflows implements pb.AgencyServiceServer.
func (s *Server) GetWorkflows(ctx context.Context, _ *pb.GetWorkflowsRequest) (*pb.GetWorkflowsResponse, error) {
	workflows, err := s.mgr.GetWorkflows(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.GetWorkflowsResponse{Workflows: workflowsToProto(workflows)}, nil
}

// GetConfiguredRoles implements pb.AgencyServiceServer.
func (s *Server) GetConfiguredRoles(ctx context.Context, _ *pb.GetConfiguredRolesRequest) (*pb.GetConfiguredRolesResponse, error) {
	roles, err := s.mgr.GetConfiguredRoles(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.GetConfiguredRolesResponse{ConfiguredRoles: configuredRolesToProto(roles)}, nil
}

// ── Domain → Proto converters ─────────────────────────────────────────────────

func agencyToProto(a codevaldagency.Agency) *pb.Agency {
	return &pb.Agency{
		Id:        a.ID,
		Name:      a.Name,
		Mission:   a.Mission,
		Vision:    a.Vision,
		Enabled:   a.Enabled,
		CreatedAt: timeToProto(a.CreatedAt),
		UpdatedAt: timeToProto(a.UpdatedAt),
	}
}

func goalsToProto(goals []codevaldagency.Goal) []*pb.Goal {
	if len(goals) == 0 {
		return nil
	}
	pgs := make([]*pb.Goal, len(goals))
	for i, g := range goals {
		pgs[i] = &pb.Goal{
			Id:          g.ID,
			Title:       g.Title,
			Description: g.Description,
			Ordinality:  int32(g.Ordinality),
		}
	}
	return pgs
}

func workflowsToProto(workflows []codevaldagency.Workflow) []*pb.Workflow {
	if len(workflows) == 0 {
		return nil
	}
	pws := make([]*pb.Workflow, len(workflows))
	for i, w := range workflows {
		pws[i] = &pb.Workflow{
			Id:   w.ID,
			Name: w.Name,
		}
	}
	return pws
}

func actorTypeToProto(a codevaldagency.ActorType) pb.ActorType {
	switch a {
	case codevaldagency.ActorTypeHuman:
		return pb.ActorType_ACTOR_TYPE_HUMAN
	case codevaldagency.ActorTypeAIAgent:
		return pb.ActorType_ACTOR_TYPE_AI
	case codevaldagency.ActorTypeComputeAgent:
		return pb.ActorType_ACTOR_TYPE_EITHER
	default:
		return pb.ActorType_ACTOR_TYPE_UNSPECIFIED
	}
}

func configuredRolesToProto(roles []codevaldagency.ConfiguredRole) []*pb.ConfiguredRole {
	if len(roles) == 0 {
		return nil
	}
	out := make([]*pb.ConfiguredRole, len(roles))
	for i, r := range roles {
		out[i] = &pb.ConfiguredRole{
			Role:      r.Name,
			ActorType: actorTypeToProto(r.ActorType),
		}
	}
	return out
}

func timeToProto(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func publicationToProto(p codevaldagency.AgencyPublication) *pb.AgencyPublication {
	return &pb.AgencyPublication{
		Id:          p.ID,
		Version:     int32(p.Version),
		Tag:         p.Tag,
		PublishedAt: timeToProto(p.PublishedAt),
	}
}

func draftToProto(d codevaldagency.AgencyDraft) *pb.AgencyDraft {
	return &pb.AgencyDraft{
		Id:             d.ID,
		AgencyId:       d.AgencyID,
		Description:    d.Description,
		ForkedFromId:   d.ForkedFromID,
		ForkedFromType: d.ForkedFromType,
		Status:         draftStatusToProto(d.Status),
		CreatedAt:      timeToProto(d.CreatedAt),
		UpdatedAt:      timeToProto(d.UpdatedAt),
	}
}

func draftStatusToProto(s codevaldagency.AgencyDraftStatus) pb.AgencyDraftStatus {
	switch s {
	case codevaldagency.DraftStatusOpen:
		return pb.AgencyDraftStatus_AGENCY_DRAFT_STATUS_OPEN
	case codevaldagency.DraftStatusPromoted:
		return pb.AgencyDraftStatus_AGENCY_DRAFT_STATUS_PROMOTED
	case codevaldagency.DraftStatusArchived:
		return pb.AgencyDraftStatus_AGENCY_DRAFT_STATUS_ARCHIVED
	default:
		return pb.AgencyDraftStatus_AGENCY_DRAFT_STATUS_UNSPECIFIED
	}
}

// ── Draft RPC handlers ────────────────────────────────────────────────────────

// CreateDraft implements pb.AgencyServiceServer.
func (s *Server) CreateDraft(ctx context.Context, req *pb.CreateDraftRequest) (*pb.AgencyDraft, error) {
	draft, err := s.mgr.CreateDraft(ctx, req.GetDescription(), req.GetForkedFromId(), req.GetForkedFromType())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return draftToProto(draft), nil
}

// GetDraft implements pb.AgencyServiceServer.
func (s *Server) GetDraft(ctx context.Context, req *pb.GetDraftRequest) (*pb.AgencyDraft, error) {
	draft, err := s.mgr.GetDraft(ctx, req.GetDraftId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return draftToProto(draft), nil
}

// ListDrafts implements pb.AgencyServiceServer.
func (s *Server) ListDrafts(ctx context.Context, _ *pb.ListDraftsRequest) (*pb.ListDraftsResponse, error) {
	drafts, err := s.mgr.ListDrafts(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}
	out := make([]*pb.AgencyDraft, len(drafts))
	for i, d := range drafts {
		out[i] = draftToProto(d)
	}
	return &pb.ListDraftsResponse{Drafts: out}, nil
}

// UpdateDraftDescription implements pb.AgencyServiceServer.
func (s *Server) UpdateDraftDescription(ctx context.Context, req *pb.UpdateDraftDescriptionRequest) (*pb.AgencyDraft, error) {
	draft, err := s.mgr.UpdateDraftDescription(ctx, req.GetDraftId(), req.GetDescription())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return draftToProto(draft), nil
}

// PromoteDraft implements pb.AgencyServiceServer.
func (s *Server) PromoteDraft(ctx context.Context, req *pb.PromoteDraftRequest) (*pb.Agency, error) {
	agency, err := s.mgr.PromoteDraft(ctx, req.GetDraftId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return agencyToProto(agency), nil
}

// ArchiveDraft implements pb.AgencyServiceServer.
func (s *Server) ArchiveDraft(ctx context.Context, req *pb.ArchiveDraftRequest) (*pb.AgencyDraft, error) {
	draft, err := s.mgr.ArchiveDraft(ctx, req.GetDraftId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return draftToProto(draft), nil
}
