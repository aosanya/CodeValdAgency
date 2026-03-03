package codevaldagency

import "errors"

// ErrAgencyNotFound is returned when an agency does not exist for the given ID.
var ErrAgencyNotFound = errors.New("agency not found")

// ErrInvalidLifecycleTransition is returned by [AgencyManager.UpdateAgency]
// when the requested lifecycle change is not a valid forward transition from
// the current state. See [AgencyLifecycle.CanTransitionTo] for the allowed
// transition table.
var ErrInvalidLifecycleTransition = errors.New("invalid agency lifecycle transition")

// ErrInvalidAgency is returned when an agency is missing required fields.
var ErrInvalidAgency = errors.New("invalid agency: missing required fields")

// ErrInvalidJSON is returned by [AgencyManager.SetAgencyDetails] when the
// supplied JSON payload cannot be parsed into a valid Agency document,
// or when the required "id" field is absent.
var ErrInvalidJSON = errors.New("invalid agency: malformed JSON payload")
