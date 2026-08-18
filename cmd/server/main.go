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

	if err := run(cfg, logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, logger *slog.Logger) error {

	// Signals bound the whole process: the same context stops the workers and
	// triggers Fiber's graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
	reserveIP := app.NewReserveIP(pool, provider)
	releaseIP := app.NewReleaseIP(pool, provider)
	assignIPToServer := app.NewAssignIPToServer(pool, provider)
	unassignIP := app.NewUnassignIP(pool, provider)
	setupDNS := app.NewSetupDNS(pool, provider)
	operationStatus := app.NewGetOperationStatus(jobs, provider, clock)
	recovery := app.NewRecovery(jobs, clock)

	pool.Register(domain.JobTypeProvisionServer, provisionServer.Handle)
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
		ProvisionServer:    provisionServer,
		ListServers:        listServers,
		GetServer:          getServer,
		DeleteServer:       deleteServer,
		ReserveIP:          reserveIP,
		ReleaseIP:          releaseIP,
		AssignIPToServer:   assignIPToServer,
		UnassignIP:         unassignIP,
		SetupDNS:           setupDNS,
		GetOperationStatus: operationStatus,
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

	logger.Info("listening", "addr", cfg.Addr, "tools", len(mcpServer.Tools()))

	listenErr := fiberApp.Listen(cfg.Addr, fiber.ListenConfig{
		DisableStartupMessage: true,
		GracefulContext:       ctx,
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
