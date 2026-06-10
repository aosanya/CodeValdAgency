package codevaldagency

import "errors"

// ErrAgencyNotFound is returned when an agency does not exist for the given ID.
var ErrAgencyNotFound = errors.New("agency not found")

// ErrAgencyNotPublished is returned when an operation requires a live (promoted)
// agency state but no draft has ever been promoted for this agency.
var ErrAgencyNotPublished = errors.New("agency not published: promote a draft first")

// ErrAgencyReadOnly is returned by [AgencyManager.SetAgencyDetails] when a
// live agency already exists. The live agency is immutable; create a draft
// from it and promote the draft to make changes.
var ErrAgencyReadOnly = errors.New("agency is read-only: create and promote a draft to modify the live state")

// ErrInvalidAgency is returned when an agency is missing required fields.
var ErrInvalidAgency = errors.New("invalid agency: missing required fields")

// ErrInvalidJSON is returned by [AgencyManager.SetAgencyDetails] when the
// supplied JSON payload cannot be parsed into a valid Agency document,
// or when the required "id" field is absent.
var ErrInvalidJSON = errors.New("invalid agency: malformed JSON payload")

// ErrDraftNotFound is returned when an AgencyDraft does not exist for the
// given draft ID.
var ErrDraftNotFound = errors.New("agency draft not found")

// ErrDraftNotOpen is returned when an operation requires the draft to be in
// the "open" state but it is already promoted or archived.
var ErrDraftNotOpen = errors.New("agency draft is not open")

// ErrPublicationNotFound is returned by [AgencyManager.GetPublication] when
// no publication with the requested version number exists for this agency.
var ErrPublicationNotFound = errors.New("agency publication not found")

// ErrInvalidPublicationStatus is returned by [AgencyManager.UpdatePublicationStatus]
// when the requested status is not a valid transition from the publication's
// current status. Allowed transitions: draft → active, active → archived.
var ErrInvalidPublicationStatus = errors.New("invalid publication status transition")

// ErrNoChangesDetected is returned by [AgencyManager.PublishAgency] when the
// content hash of the supplied draft matches an existing publication, meaning
// the draft content is identical to a previously published version and there
// is nothing new to release.
var ErrNoChangesDetected = errors.New("no changes detected: draft content is identical to an existing publication")

// ErrWorkPlanNotFound is returned by work plan methods when no WorkPlan entity
// exists for the given work plan ID.
var ErrWorkPlanNotFound = errors.New("work plan not found")

// ErrContextSourceNotFound is returned when no ContextSource entity exists for
// the given source ID.
var ErrContextSourceNotFound = errors.New("context source not found")

// ErrInvalidRegex is returned by [AgencyManager.CreateWorkPlan] and
// [AgencyManager.UpdateWorkPlan] when trigger_topic or payload_condition is not
// a valid Go regular expression.
var ErrInvalidRegex = errors.New("invalid regex: trigger_topic or payload_condition is not a valid Go regexp")

// ErrEventFlowStepNotFound is returned by [AgencyManager.LookupFlowStep] when no
// live EventFlowStep entity in the active publication matches the lookup criteria.
var ErrEventFlowStepNotFound = errors.New("event flow step not found")

// ErrInvalidLookup is returned by [AgencyManager.LookupFlowStep] when none of the
// lookup keys (handler_code, trigger_topic+consumer, code) are populated.
var ErrInvalidLookup = errors.New("invalid lookup: at least one of handler_code, (trigger_topic + consumer), or code must be set")
