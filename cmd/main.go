// Command server starts the CodeValdAgency gRPC microservice.
//
// Configuration is via environment variables:
//
//	CODEVALDAGENCY_GRPC_PORT       gRPC listener port (default "50053")
//	CROSS_GRPC_ADDR                CodeValdCross gRPC address for service
//	                               registration heartbeats (optional; omit to
//	                               disable registration)
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
	"github.com/aosanya/CodeValdAgency/internal/server"
	"github.com/aosanya/CodeValdAgency/storage/arangodb"
	crossv1 "github.com/aosanya/CodeValdSharedLib/gen/go/codevaldcross/v1"
	sharedregistrar "github.com/aosanya/CodeValdSharedLib/registrar"
	"github.com/aosanya/CodeValdSharedLib/serverutil"
)

func main() {
	cfg := config.Load()

	backend, err := initBackend(cfg)
	if err != nil {
		log.Fatalf("codevaldagency: failed to initialise backend: %v", err)
	}

	mgr, err := codevaldagency.NewAgencyManager(backend)
	if err != nil {
		log.Fatalf("codevaldagency: failed to create AgencyManager: %v", err)
	}

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatalf("codevaldagency: failed to listen on :%s: %v", cfg.GRPCPort, err)
	}

	grpcServer, _ := serverutil.NewGRPCServer()
	pb.RegisterAgencyServiceServer(grpcServer, server.New(mgr))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.CrossGRPCAddr != "" {
		reg, err := sharedregistrar.New(
			cfg.CrossGRPCAddr,
			cfg.AdvertiseAddr,
			"", // agency-scoped ID — empty because this service manages all agencies
			"codevaldagency",
			[]string{"cross.agency.created"},
			[]string{},
			agencyRoutes(),
			cfg.PingInterval,
			cfg.PingTimeout,
		)
		if err != nil {
			log.Printf("codevaldagency: registrar: failed to create: %v — continuing without registration", err)
		} else {
			defer reg.Close()
			go reg.Run(ctx)
		}
	} else {
		log.Println("codevaldagency: CROSS_GRPC_ADDR not set — skipping CodeValdCross registration")
	}

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

// agencyRoutes returns the HTTP routes that CodeValdAgency declares to CodeValdCross.
func agencyRoutes() []*crossv1.RouteDeclaration {
	return []*crossv1.RouteDeclaration{
		{
			Method:     "POST",
			Pattern:    "/agencies",
			Capability: "create_agency",
			GrpcMethod: "/codevaldagency.v1.AgencyService/CreateAgency",
		},
		{
			Method:     "GET",
			Pattern:    "/agencies",
			Capability: "list_agencies",
			GrpcMethod: "/codevaldagency.v1.AgencyService/ListAgencies",
		},
		{
			Method:     "GET",
			Pattern:    "/agencies/{agencyId}",
			Capability: "get_agency",
			GrpcMethod: "/codevaldagency.v1.AgencyService/GetAgency",
			PathBindings: []*crossv1.PathBinding{
				{UrlParam: "agencyId", Field: "agency_id"},
			},
		},
		{
			Method:     "PUT",
			Pattern:    "/agencies/{agencyId}",
			Capability: "update_agency",
			GrpcMethod: "/codevaldagency.v1.AgencyService/UpdateAgency",
			PathBindings: []*crossv1.PathBinding{
				{UrlParam: "agencyId", Field: "agency_id"},
			},
		},
		{
			Method:     "DELETE",
			Pattern:    "/agencies/{agencyId}",
			Capability: "delete_agency",
			GrpcMethod: "/codevaldagency.v1.AgencyService/DeleteAgency",
			PathBindings: []*crossv1.PathBinding{
				{UrlParam: "agencyId", Field: "agency_id"},
			},
		},
	}
}
