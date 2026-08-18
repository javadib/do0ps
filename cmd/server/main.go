// Command server starts the do0ps MCP server.
//
// This file is the composition root: the only place that knows about every
// package at once. It builds the adapters, injects them into the core use
// cases through ports, and starts Fiber. Nothing below this file reaches for a
// concrete implementation of a port.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v3"

	"github.com/javadib/do0ps/internal/adapters/mcp"
	"github.com/javadib/do0ps/internal/adapters/providers/parspack"
	"github.com/javadib/do0ps/internal/adapters/queue"
	"github.com/javadib/do0ps/internal/adapters/sqlite"
	"github.com/javadib/do0ps/internal/adapters/system"
	"github.com/javadib/do0ps/internal/auth"
	"github.com/javadib/do0ps/internal/config"
	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// version is the do0ps build version. It defaults to "dev" for local builds
// and is overridden at build time via `-ldflags "-X main.version=$APP_VERSION"`
// -- see Dockerfile and .github/workflows/docker-publish.yml, which pass this
// on tagged releases.
var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	logger, err := config.NewLogger(cfg.LogLevel)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	slog.SetDefault(logger)

	// Signals bound the whole process: the same context stops the workers and
	// triggers Fiber's graceful shutdown. Kept here, not inside run, so run
	// can be driven by an arbitrary context (e.g. a test's own cancellation)
	// without touching process-wide signal handling.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg, logger, nil); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

// run builds every adapter, wires the use cases behind their ports, and
// serves the MCP endpoint until ctx is canceled. onListen, if non-nil, is
// invoked with the address Fiber actually bound to — nil in production,
// where cfg.Addr is already a fixed host:port; tests use it to discover an
// ephemeral port.
func run(ctx context.Context, cfg config.Config, logger *slog.Logger, onListen func(net.Addr)) error {
	// --- secondary adapters ---------------------------------------------
	db, err := sqlite.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer db.Close()

	jobs := sqlite.NewJobStore(db)
	clock := system.Clock{}
	ids := system.IDGenerator{}

	provider, err := parspack.New()
	if err != nil {
		return err
	}

	pool, err := queue.New(jobs, clock,
		queue.WithWorkers(cfg.Workers),
		queue.WithCapacity(cfg.QueueDepth),
		queue.WithLogger(logger),
	)
	if err != nil {
		return err
	}

	// --- core use cases --------------------------------------------------
	provisionServer, err := app.NewProvisionServer(jobs, pool, provider, clock, ids,
		app.WithPollInterval(cfg.PollInterval),
		app.WithPollTimeout(cfg.PollTimeout),
	)
	if err != nil {
		return err
	}
	listServers := app.NewListServers(pool, provider)
	getServer := app.NewGetServer(pool, provider)
	deleteServer := app.NewDeleteServer(pool, provider)
	createVPC := app.NewCreateVPC(pool, provider)
	listVPCs := app.NewListVPCs(pool, provider)
	getVPC := app.NewGetVPC(pool, provider)
	deleteVPC := app.NewDeleteVPC(pool, provider)
	reserveIP := app.NewReserveIP(pool, provider)
	releaseIP := app.NewReleaseIP(pool, provider)
	assignIPToServer := app.NewAssignIPToServer(pool, provider)
	unassignIP := app.NewUnassignIP(pool, provider)
	registerSSHKey := app.NewRegisterSSHKey(pool, provider)
	listSSHKeys := app.NewListSSHKeys(pool, provider)
	deleteSSHKey := app.NewDeleteSSHKey(pool, provider)
	operationStatus := app.NewGetOperationStatus(jobs, provider, clock)
	createSnapshot, err := app.NewCreateSnapshot(jobs, pool, provider, clock, ids,
		app.WithActionPollInterval(cfg.PollInterval),
		app.WithActionPollTimeout(cfg.PollTimeout),
	)
	if err != nil {
		return err
	}
	listSnapshots := app.NewListSnapshots(pool, provider)
	deleteSnapshot := app.NewDeleteSnapshot(pool, provider)
	restoreVM, err := app.NewRestoreVM(jobs, pool, provider, clock, ids,
		app.WithActionPollInterval(cfg.PollInterval),
		app.WithActionPollTimeout(cfg.PollTimeout),
	)
	if err != nil {
		return err
	}

	listSSLProducts := app.NewListSSLProducts(pool, provider)
	createSSLOrder := app.NewCreateSSLOrder(pool, provider)
	processSSLOrder := app.NewProcessSSLOrder(pool, provider)
	getSSLChallenge := app.NewGetSSLChallenge(pool, provider)
	reloadSSLChallenge := app.NewReloadSSLChallenge(pool, provider)
	verifySSLChallenge := app.NewVerifySSLChallenge(pool, provider)
	getSSLCertificate := app.NewGetSSLCertificate(pool, provider)
	reissueSSLCertificate := app.NewReissueSSLCertificate(pool, provider)
	recovery := app.NewRecovery(jobs, clock)

	createCDNZone := app.NewCreateCDNZone(pool, provider)
	listCDNZones := app.NewListCDNZones(pool, provider)
	getCDNZone := app.NewGetCDNZone(pool, provider)
	deleteCDNZone := app.NewDeleteCDNZone(pool, provider)
	listCDNZonePlans := app.NewListCDNZonePlans(pool, provider)
	getNameserverRecords := app.NewGetNameserverRecords(pool, provider)
	listDNSRecords := app.NewListDNSRecords(pool, provider)
	createDNSRecord := app.NewCreateDNSRecord(pool, provider)
	updateDNSRecord := app.NewUpdateDNSRecord(pool, provider)
	deleteDNSRecord := app.NewDeleteDNSRecord(pool, provider)

	createFirewall := app.NewCreateFirewall(pool, provider)
	getFirewall := app.NewGetFirewall(pool, provider)
	listFirewalls := app.NewListFirewalls(pool, provider)
	updateFirewall := app.NewUpdateFirewall(pool, provider)
	deleteFirewall := app.NewDeleteFirewall(pool, provider)

	provisionLoadBalancer, err := app.NewProvisionLoadBalancer(jobs, pool, provider, clock, ids,
		app.WithLoadBalancerPollInterval(cfg.PollInterval),
		app.WithLoadBalancerPollTimeout(cfg.PollTimeout),
	)
	if err != nil {
		return err
	}
	getLoadBalancer := app.NewGetLoadBalancer(pool, provider)
	listLoadBalancers := app.NewListLoadBalancers(pool, provider)
	updateLoadBalancer := app.NewUpdateLoadBalancer(pool, provider)
	deleteLoadBalancer := app.NewDeleteLoadBalancer(pool, provider)

	pool.Register(domain.JobTypeProvisionServer, provisionServer.Handle)
	pool.Register(domain.JobTypeCreateSnapshot, createSnapshot.Handle)
	pool.Register(domain.JobTypeRestoreVM, restoreVM.Handle)
	pool.Register(domain.JobTypeProvisionLoadBalancer, provisionLoadBalancer.Handle)
	pool.Start(ctx)

	flagged, err := recovery.Run(ctx)
	if err != nil {
		return err
	}
	if flagged > 0 {
		logger.Warn("jobs interrupted by a previous shutdown; they will be reconciled on the next status call",
			"count", flagged)
	}

	// --- primary adapter -------------------------------------------------
	mcpServer, err := mcp.NewServer(mcp.Tools(mcp.UseCases{
		ProvisionServer: provisionServer,
		ListServers:     listServers,
		GetServer:       getServer,
		DeleteServer:    deleteServer,
		CreateVPC:       createVPC,
		ListVPCs:        listVPCs,
		GetVPC:          getVPC,
		DeleteVPC:       deleteVPC,

		GetOperationStatus: operationStatus,
		CreateSnapshot:     createSnapshot,
		ListSnapshots:      listSnapshots,
		DeleteSnapshot:     deleteSnapshot,
		RestoreVM:          restoreVM,

		CreateCDNZone:        createCDNZone,
		ListCDNZones:         listCDNZones,
		GetCDNZone:           getCDNZone,
		DeleteCDNZone:        deleteCDNZone,
		ListCDNZonePlans:     listCDNZonePlans,
		GetNameserverRecords: getNameserverRecords,
		ListDNSRecords:       listDNSRecords,
		CreateDNSRecord:      createDNSRecord,
		UpdateDNSRecord:      updateDNSRecord,
		DeleteDNSRecord:      deleteDNSRecord,

		RegisterSSHKey: registerSSHKey,
		ListSSHKeys:    listSSHKeys,
		DeleteSSHKey:   deleteSSHKey,

		CreateFirewall: createFirewall,
		GetFirewall:    getFirewall,
		ListFirewalls:  listFirewalls,
		UpdateFirewall: updateFirewall,
		DeleteFirewall: deleteFirewall,

		ReserveIP:        reserveIP,
		ReleaseIP:        releaseIP,
		AssignIPToServer: assignIPToServer,
		UnassignIP:       unassignIP,

		ListSSLProducts:       listSSLProducts,
		CreateSSLOrder:        createSSLOrder,
		ProcessSSLOrder:       processSSLOrder,
		GetSSLChallenge:       getSSLChallenge,
		ReloadSSLChallenge:    reloadSSLChallenge,
		VerifySSLChallenge:    verifySSLChallenge,
		GetSSLCertificate:     getSSLCertificate,
		ReissueSSLCertificate: reissueSSLCertificate,
		ProvisionLoadBalancer: provisionLoadBalancer,
		GetLoadBalancer:       getLoadBalancer,
		ListLoadBalancers:     listLoadBalancers,
		UpdateLoadBalancer:    updateLoadBalancer,
		DeleteLoadBalancer:    deleteLoadBalancer,
	}), mcp.WithLogger(logger))
	if err != nil {
		return err
	}

	tokens, err := auth.ParseTokens(cfg.AuthTokens)
	if err != nil {
		return err
	}
	tokenStore, err := auth.NewStaticStore(tokens)
	if err != nil {
		return err
	}

	fiberApp := fiber.New(fiber.Config{
		AppName:      "do0ps",
		ServerHeader: "do0ps",
	})

	// Liveness stays outside the allow-list so orchestrators can probe it.
	fiberApp.Get("/healthz", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Everything MCP sits behind the bearer allow-list.
	protected := fiberApp.Group("", auth.Middleware(tokenStore))
	mcpServer.Register(protected)

	logger.Info("listening", "addr", cfg.Addr, "tools", len(mcpServer.Tools()), "version", version)

	listenErr := fiberApp.Listen(cfg.Addr, fiber.ListenConfig{
		DisableStartupMessage: true,
		GracefulContext:       ctx,
		ListenerAddrFunc:      onListen,
	})

	// Fiber has stopped accepting requests; drain the workers before the
	// deferred db.Close runs.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownWait)
	defer cancel()

	if err := pool.Shutdown(shutdownCtx); err != nil {
		logger.Error("draining worker pool", "error", err)
	}

	if listenErr != nil && !errors.Is(listenErr, context.Canceled) {
		return listenErr
	}
	logger.Info("shutdown complete")
	return nil
}
