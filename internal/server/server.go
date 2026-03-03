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
func (s *Server) GetAgency(ctx context.Context, req *pb.GetAgencyRequest) (*pb.Agency, error) {
	agency, err := s.mgr.GetAgency(ctx, req.GetAgencyId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return agencyToProto(agency), nil
}

// UpdateAgency implements pb.AgencyServiceServer.
func (s *Server) UpdateAgency(ctx context.Context, req *pb.UpdateAgencyRequest) (*pb.Agency, error) {
	agency, err := s.mgr.UpdateAgency(ctx, req.GetAgencyId(), protoToUpdateRequest(req))
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

// ── Proto → Domain converters ─────────────────────────────────────────────────

func protoToUpdateRequest(req *pb.UpdateAgencyRequest) codevaldagency.UpdateAgencyRequest {
	return codevaldagency.UpdateAgencyRequest{
		Name:            req.GetName(),
		Mission:         req.GetMission(),
		Vision:          req.GetVision(),
		Status:          protoToLifecycle(req.GetStatus()),
		Goals:           protoToGoals(req.GetGoals()),
		Workflows:       protoToWorkflows(req.GetWorkflows()),
		ConfiguredRoles: req.GetConfiguredRoles(),
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
			ID:        pw.GetId(),
			Name:      pw.GetName(),
			WorkItems: protoToWorkItems(pw.GetWorkItems()),
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
			Order:       int(pwi.GetOrder()),
			Parallel:    pwi.GetParallel(),
			GoalIDs:     pwi.GetGoalIds(),
			Assignments: protoToAssignments(pwi.GetAssignments()),
		}
	}
	return items
}

func protoToAssignments(pas []*pb.RoleAssignment) []codevaldagency.RoleAssignment {
	if len(pas) == 0 {
		return nil
	}
	assignments := make([]codevaldagency.RoleAssignment, len(pas))
	for i, pa := range pas {
		assignments[i] = codevaldagency.RoleAssignment{
			Role: codevaldagency.AgencyRole(pa.GetRole()),
			RACI: protoToRACILabel(pa.GetRaci()),
		}
	}
	return assignments
}

func protoToRACILabel(r pb.RACILabel) codevaldagency.RACILabel {
	switch r {
	case pb.RACILabel_RACI_LABEL_RESPONSIBLE:
		return codevaldagency.RACIResponsible
	case pb.RACILabel_RACI_LABEL_ACCOUNTABLE:
		return codevaldagency.RACIAccountable
	case pb.RACILabel_RACI_LABEL_CONSULTED:
		return codevaldagency.RACIConsulted
	case pb.RACILabel_RACI_LABEL_INFORMED:
		return codevaldagency.RACIInformed
	default:
		return ""
	}
}

// ── Domain → Proto converters ─────────────────────────────────────────────────

func agencyToProto(a codevaldagency.Agency) *pb.Agency {
	return &pb.Agency{
		Id:              a.ID,
		Name:            a.Name,
		Mission:         a.Mission,
		Vision:          a.Vision,
		Status:          lifecycleToProto(a.Status),
		Goals:           goalsToProto(a.Goals),
		Workflows:       workflowsToProto(a.Workflows),
		ConfiguredRoles: a.ConfiguredRoles,
		CreatedAt:       timeToProto(a.CreatedAt),
		UpdatedAt:       timeToProto(a.UpdatedAt),
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
			Id:        w.ID,
			Name:      w.Name,
			WorkItems: workItemsToProto(w.WorkItems),
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
			Order:       int32(wi.Order),
			Parallel:    wi.Parallel,
			GoalIds:     wi.GoalIDs,
			Assignments: assignmentsToProto(wi.Assignments),
		}
	}
	return pwis
}

func assignmentsToProto(assignments []codevaldagency.RoleAssignment) []*pb.RoleAssignment {
	if len(assignments) == 0 {
		return nil
	}
	pas := make([]*pb.RoleAssignment, len(assignments))
	for i, a := range assignments {
		pas[i] = &pb.RoleAssignment{
			Role: string(a.Role),
			Raci: raciLabelToProto(a.RACI),
		}
	}
	return pas
}

func raciLabelToProto(r codevaldagency.RACILabel) pb.RACILabel {
	switch r {
	case codevaldagency.RACIResponsible:
		return pb.RACILabel_RACI_LABEL_RESPONSIBLE
	case codevaldagency.RACIAccountable:
		return pb.RACILabel_RACI_LABEL_ACCOUNTABLE
	case codevaldagency.RACIConsulted:
		return pb.RACILabel_RACI_LABEL_CONSULTED
	case codevaldagency.RACIInformed:
		return pb.RACILabel_RACI_LABEL_INFORMED
	default:
		return pb.RACILabel_RACI_LABEL_UNSPECIFIED
	}
}

func timeToProto(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}
