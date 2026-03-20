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

// UpdateAgency implements pb.AgencyServiceServer.
func (s *Server) UpdateAgency(ctx context.Context, req *pb.UpdateAgencyRequest) (*pb.Agency, error) {
	agency, err := s.mgr.UpdateAgency(ctx, protoToUpdateRequest(req))
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

// ── Proto → Domain converters ─────────────────────────────────────────────────

func protoToUpdateRequest(req *pb.UpdateAgencyRequest) codevaldagency.UpdateAgencyRequest {
	return codevaldagency.UpdateAgencyRequest{
		Name:    req.GetName(),
		Mission: req.GetMission(),
		Vision:  req.GetVision(),
		Status:  protoToLifecycle(req.GetStatus()),
	}
}

func protoToLifecycle(l pb.AgencyLifecycle) codevaldagency.AgencyLifecycle {
	switch l {
	case pb.AgencyLifecycle_AGENCY_LIFECYCLE_DRAFT:
		return codevaldagency.LifecycleDraft
	case pb.AgencyLifecycle_AGENCY_LIFECYCLE_ACTIVE:
		return codevaldagency.LifecycleActive
	case pb.AgencyLifecycle_AGENCY_LIFECYCLE_ACHIEVED:
		return codevaldagency.LifecycleAchieved
	default:
		return ""
	}
}

func protoToGoals(pgs []*pb.Goal) []codevaldagency.Goal {
	if len(pgs) == 0 {
		return nil
	}
	goals := make([]codevaldagency.Goal, len(pgs))
	for i, pg := range pgs {
		goals[i] = codevaldagency.Goal{
			ID:          pg.GetId(),
			Title:       pg.GetTitle(),
			Description: pg.GetDescription(),
			Ordinality:  int(pg.GetOrdinality()),
		}
	}
	return goals
}

func protoToWorkflows(pws []*pb.Workflow) []codevaldagency.Workflow {
	if len(pws) == 0 {
		return nil
	}
	wfs := make([]codevaldagency.Workflow, len(pws))
	for i, pw := range pws {
		wfs[i] = codevaldagency.Workflow{
			ID:   pw.GetId(),
			Name: pw.GetName(),
		}
	}
	return wfs
}

func protoToWorkItems(pwis []*pb.WorkItem) []codevaldagency.WorkItem {
	if len(pwis) == 0 {
		return nil
	}
	items := make([]codevaldagency.WorkItem, len(pwis))
	for i, pwi := range pwis {
		items[i] = codevaldagency.WorkItem{
			ID:          pwi.GetId(),
			Title:       pwi.GetTitle(),
			Description: pwi.GetDescription(),
		}
	}
	return items
}

// ── Domain → Proto converters ─────────────────────────────────────────────────

func agencyToProto(a codevaldagency.Agency) *pb.Agency {
	return &pb.Agency{
		Id:        a.ID,
		Name:      a.Name,
		Mission:   a.Mission,
		Vision:    a.Vision,
		Status:    lifecycleToProto(a.Status),
		CreatedAt: timeToProto(a.CreatedAt),
		UpdatedAt: timeToProto(a.UpdatedAt),
	}
}

func lifecycleToProto(l codevaldagency.AgencyLifecycle) pb.AgencyLifecycle {
	switch l {
	case codevaldagency.LifecycleDraft:
		return pb.AgencyLifecycle_AGENCY_LIFECYCLE_DRAFT
	case codevaldagency.LifecycleActive:
		return pb.AgencyLifecycle_AGENCY_LIFECYCLE_ACTIVE
	case codevaldagency.LifecycleAchieved:
		return pb.AgencyLifecycle_AGENCY_LIFECYCLE_ACHIEVED
	default:
		return pb.AgencyLifecycle_AGENCY_LIFECYCLE_UNSPECIFIED
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

func workItemsToProto(items []codevaldagency.WorkItem) []*pb.WorkItem {
	if len(items) == 0 {
		return nil
	}
	pwis := make([]*pb.WorkItem, len(items))
	for i, wi := range items {
		pwis[i] = &pb.WorkItem{
			Id:          wi.ID,
			Title:       wi.Title,
			Description: wi.Description,
		}
	}
	return pwis
}

func protoToActorType(a pb.ActorType) codevaldagency.ActorType {
	switch a {
	case pb.ActorType_ACTOR_TYPE_HUMAN:
		return codevaldagency.ActorTypeHuman
	case pb.ActorType_ACTOR_TYPE_AI:
		return codevaldagency.ActorTypeAIAgent
	case pb.ActorType_ACTOR_TYPE_EITHER:
		return codevaldagency.ActorTypeComputeAgent
	default:
		return ""
	}
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

func protoToConfiguredRoles(prs []*pb.ConfiguredRole) []codevaldagency.ConfiguredRole {
	if len(prs) == 0 {
		return nil
	}
	out := make([]codevaldagency.ConfiguredRole, len(prs))
	for i, pr := range prs {
		out[i] = codevaldagency.ConfiguredRole{
			Name:      pr.GetRole(),
			ActorType: protoToActorType(pr.GetActorType()),
		}
	}
	return out
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
