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
//
// TODO(MVP-AGENCY-008-D): replace stub DataManager/SchemaManager with
// the real arangodb.New(db) constructor once storage split is complete.
package main

import (
	"context"
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

	// TODO(MVP-AGENCY-008-D): construct real DataManager and SchemaManager
	// from arangodb.New(db) once the storage split is complete.
	// For now the service panics at startup if called — wiring is a placeholder.
	log.Println("codevaldagency: WARNING — DataManager not wired (pending MVP-AGENCY-008-D)")
	mgr := codevaldagency.NewAgencyManager(nil, nil, pub, cfg.AgencyID)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatalf("codevaldagency: failed to listen on :%s: %v", cfg.GRPCPort, err)
	}

	grpcServer, _ := serverutil.NewGRPCServer()
	pb.RegisterAgencyServiceServer(grpcServer, server.New(mgr))
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
