// Command server starts the CodeValdAgency gRPC microservice.
//
// Configuration is via environment variables:
//
//	CODEVALDAGENCY_GRPC_PORT       gRPC listener port (required, set in .env)
//	CROSS_GRPC_ADDR                CodeValdCross gRPC address for service
//	                               registration heartbeats and event publishing
//	                               (optional; omit to disable)
//	AGENCY_GRPC_ADVERTISE_ADDR     address CodeValdCross dials back (default ":PORT")
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
	"github.com/aosanya/CodeValdAgency/storage/arangodb"
	"github.com/aosanya/CodeValdSharedLib/serverutil"
)

func main() {
	cfg := config.Load()

	backend, err := initBackend(cfg)
	if err != nil {
		log.Fatalf("codevaldagency: failed to initialise backend: %v", err)
	}

	// Build AgencyManager options — attach a CrossPublisher if Cross is configured.
	var mgrOpts []codevaldagency.AgencyManagerOption

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.CrossGRPCAddr != "" {
		reg, err := registrar.New(
			cfg.CrossGRPCAddr,
			cfg.AdvertiseAddr,
			cfg.PingInterval,
			cfg.PingTimeout,
		)
		if err != nil {
			log.Printf("codevaldagency: registrar: failed to create: %v — continuing without registration", err)
		} else {
			defer reg.Close()
			go reg.Run(ctx)
			mgrOpts = append(mgrOpts, codevaldagency.WithPublisher(reg))
		}
	} else {
		log.Println("codevaldagency: CROSS_GRPC_ADDR not set — skipping CodeValdCross registration")
	}

	mgr, err := codevaldagency.NewAgencyManager(backend, mgrOpts...)
	if err != nil {
		log.Fatalf("codevaldagency: failed to create AgencyManager: %v", err)
	}

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatalf("codevaldagency: failed to listen on :%s: %v", cfg.GRPCPort, err)
	}

	grpcServer, _ := serverutil.NewGRPCServer()
	pb.RegisterAgencyServiceServer(grpcServer, server.New(mgr))

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

// initBackend constructs the ArangoDB storage backend from config.
func initBackend(cfg config.Config) (codevaldagency.Backend, error) {
	return arangodb.NewBackend(arangodb.Config{
		Endpoint: cfg.ArangoEndpoint,
		Username: cfg.ArangoUser,
		Password: cfg.ArangoPassword,
		Database: cfg.ArangoDatabase,
	})
}

