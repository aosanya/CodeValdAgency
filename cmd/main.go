// Command server starts the CodeValdAgency gRPC microservice.
//
// Configuration is via environment variables:
//
//	CODEVALDAGENCY_GRPC_PORT       gRPC listener port (required, set in .env)
//	CROSS_GRPC_ADDR                CodeValdCross gRPC address for service
//	                               registration heartbeats and event publishing
//	                               (optional; omit to disable)
//	AGENCY_GRPC_ADVERTISE_ADDR     address CodeValdCross dials back (default ":PORT")
//	CODEVALDAGENCY_AGENCY_ID       agency ID sent in every Register heartbeat
//	                               (required when CROSS_GRPC_ADDR is set)
//	CROSS_PING_INTERVAL            heartbeat cadence (default "20s")
//	CROSS_PING_TIMEOUT             per-RPC timeout for each Register call (default "5s")
//
// ArangoDB backend:
//
//	AGENCY_ARANGO_ENDPOINT         ArangoDB endpoint URL (default "http://localhost:8529")
//	AGENCY_ARANGO_USER             ArangoDB username (default "root")
//	AGENCY_ARANGO_PASSWORD         ArangoDB password
//	AGENCY_ARANGO_DATABASE         ArangoDB database name (default "codevaldagency")
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	codevaldagency "github.com/aosanya/CodeValdAgency"
	pb "github.com/aosanya/CodeValdAgency/gen/go/codevaldagency/v1"
	"github.com/aosanya/CodeValdAgency/internal/config"
	"github.com/aosanya/CodeValdAgency/internal/registrar"
	"github.com/aosanya/CodeValdAgency/internal/server"
	arangodb "github.com/aosanya/CodeValdAgency/storage/arangodb"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	healthpb "github.com/aosanya/CodeValdSharedLib/gen/go/codevaldhealth/v1"
	"github.com/aosanya/CodeValdSharedLib/health"
	"github.com/aosanya/CodeValdSharedLib/serverutil"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var pub codevaldagency.CrossPublisher
	if cfg.CrossGRPCAddr != "" {
		reg, err := registrar.New(
			cfg.CrossGRPCAddr,
			cfg.AdvertiseAddr,
			cfg.AgencyID,
			cfg.PingInterval,
			cfg.PingTimeout,
		)
		if err != nil {
			log.Printf("codevaldagency: registrar: failed to create: %v — continuing without registration", err)
		} else {
			defer reg.Close()
			go reg.Run(ctx)
			pub = reg
		}
	} else {
		log.Println("codevaldagency: CROSS_GRPC_ADDR not set — skipping CodeValdCross registration")
	}

	// Connect to ArangoDB and construct the DataManager + SchemaManager.
	backend, err := arangodb.NewBackend(arangodb.Config{
		Endpoint: cfg.ArangoEndpoint,
		Username: cfg.ArangoUser,
		Password: cfg.ArangoPassword,
		Database: cfg.ArangoDatabase,
		Schema:   codevaldagency.DefaultAgencySchema(),
	})
	if err != nil {
		log.Fatalf("codevaldagency: ArangoDB backend: %v", err)
	}

	// Seed the pre-delivered schema idempotently on startup.
	if cfg.AgencyID != "" {
		seedCtx, seedCancel := context.WithTimeout(ctx, 10*time.Second)
		if err := seedSchemaIfNeeded(seedCtx, backend, cfg.AgencyID); err != nil {
			log.Printf("codevaldagency: schema seed: %v", err)
		}
		seedCancel()
	} else {
		log.Println("codevaldagency: CODEVALDAGENCY_AGENCY_ID not set — skipping schema seed")
	}

	mgr := codevaldagency.NewAgencyManager(backend, backend, pub, cfg.AgencyID)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatalf("codevaldagency: failed to listen on :%s: %v", cfg.GRPCPort, err)
	}

	grpcServer, _ := serverutil.NewGRPCServer()
	pb.RegisterAgencyServiceServer(grpcServer, server.New(mgr))
	pb.RegisterEntityServiceServer(grpcServer, server.NewEntityServer(backend))
	healthpb.RegisterHealthServiceServer(grpcServer, health.New("codevaldagency"))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-quit
		log.Println("codevaldagency: shutdown signal received")
		cancel()
	}()

	log.Printf("CodeValdAgency gRPC server listening on :%s", cfg.GRPCPort)
	serverutil.RunWithGracefulShutdown(ctx, grpcServer, lis, 30*time.Second)
}

// seedSchemaIfNeeded seeds the pre-delivered agency schema idempotently on
// startup. It is a no-op if an active schema version already exists.
// On first run it calls SetSchema, Publish, then Activate(1) to make the
// default schema live.
func seedSchemaIfNeeded(ctx context.Context, sm codevaldagency.AgencySchemaManager, agencyID string) error {
	_, err := sm.GetActive(ctx, agencyID)
	if err == nil {
		return nil // already active — idempotent
	}
	if !errors.Is(err, entitygraph.ErrSchemaNotFound) {
		return fmt.Errorf("seedSchemaIfNeeded %s: check active: %w", agencyID, err)
	}
	schema := codevaldagency.DefaultAgencySchema()
	schema.AgencyID = agencyID
	if err := sm.SetSchema(ctx, schema); err != nil {
		return fmt.Errorf("seedSchemaIfNeeded %s: set schema: %w", agencyID, err)
	}
	if err := sm.Publish(ctx, agencyID); err != nil {
		return fmt.Errorf("seedSchemaIfNeeded %s: publish: %w", agencyID, err)
	}
	if err := sm.Activate(ctx, agencyID, 1); err != nil {
		return fmt.Errorf("seedSchemaIfNeeded %s: activate: %w", agencyID, err)
	}
	log.Printf("codevaldagency: schema seeded for agency %s", agencyID)
	return nil
}
