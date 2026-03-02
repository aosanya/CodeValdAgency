package codevaldagency

import "errors"

// ErrAgencyNotFound is returned when an agency does not exist for the given ID.
var ErrAgencyNotFound = errors.New("agency not found")

// ErrAgencyAlreadyExists is returned by [AgencyManager.CreateAgency] when an
// agency with the same ID already exists.
var ErrAgencyAlreadyExists = errors.New("agency already exists")

// ErrInvalidLifecycleTransition is returned by [AgencyManager.UpdateAgency]
// when the requested lifecycle change is not a valid forward transition from
// the current state. See [AgencyLifecycle.CanTransitionTo] for the allowed
// transition table.
var ErrInvalidLifecycleTransition = errors.New("invalid agency lifecycle transition")

// ErrInvalidAgency is returned when an agency is missing required fields
// (e.g. empty Name on creation).
var ErrInvalidAgency = errors.New("invalid agency: missing required fields")
