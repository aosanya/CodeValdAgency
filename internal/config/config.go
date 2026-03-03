// Package config loads CodeValdAgency runtime configuration from environment variables.
package config

import (
	"time"

	"github.com/aosanya/CodeValdSharedLib/serverutil"
)

// Config holds all runtime configuration for the CodeValdAgency service.
type Config struct {
	// GRPCPort is the port the gRPC server listens on (default "50053").
	GRPCPort string

	// ArangoEndpoint is the ArangoDB HTTP endpoint (default "http://localhost:8529").
	ArangoEndpoint string

	// ArangoUser is the ArangoDB username (default "root").
	ArangoUser string

	// ArangoPassword is the ArangoDB password.
	ArangoPassword string

	// ArangoDatabase is the ArangoDB database name (default "codevaldagency").
	ArangoDatabase string

	// CrossGRPCAddr is the CodeValdCross gRPC address for registration heartbeats.
	// Empty string disables registration.
	CrossGRPCAddr string

	// AdvertiseAddr is the address CodeValdCross dials back on (default ":GRPCPort").
	AdvertiseAddr string

	// PingInterval is the heartbeat cadence sent to CodeValdCross (default 20s).
	PingInterval time.Duration

	// PingTimeout is the per-RPC timeout for each Register call (default 5s).
	PingTimeout time.Duration
}

// Load reads configuration from environment variables, falling back to defaults
// for any variable that is unset or empty.
func Load() Config {
	port := serverutil.EnvOrDefault("CODEVALDAGENCY_GRPC_PORT", "50054")
	return Config{
		GRPCPort:       port,
		ArangoEndpoint: serverutil.EnvOrDefault("AGENCY_ARANGO_ENDPOINT", "http://localhost:8529"),
		ArangoUser:     serverutil.EnvOrDefault("AGENCY_ARANGO_USER", "root"),
		ArangoPassword: serverutil.EnvOrDefault("AGENCY_ARANGO_PASSWORD", ""),
		ArangoDatabase: serverutil.EnvOrDefault("AGENCY_ARANGO_DATABASE", "codevaldagency"),
		CrossGRPCAddr:  serverutil.EnvOrDefault("CROSS_GRPC_ADDR", ""),
		AdvertiseAddr:  serverutil.EnvOrDefault("AGENCY_GRPC_ADVERTISE_ADDR", ":"+port),
		PingInterval:   serverutil.ParseDurationString("CROSS_PING_INTERVAL", 20*time.Second),
		PingTimeout:    serverutil.ParseDurationString("CROSS_PING_TIMEOUT", 5*time.Second),
	}
}
