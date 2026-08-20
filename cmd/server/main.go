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

	// --- CDN capabilities beyond zone/DNS (issue #24) --------------------
	getCDNAntivirusStatus := app.NewGetCDNAntivirusStatus(pool, provider)
	updateCDNAntivirusStatus := app.NewUpdateCDNAntivirusStatus(pool, provider)
	getCDNDNSSecStatus := app.NewGetCDNDNSSecStatus(pool, provider)
	updateCDNDNSSecStatus := app.NewUpdateCDNDNSSecStatus(pool, provider)
	getCDNOptimizationStatus := app.NewGetCDNOptimizationStatus(pool, provider)
	updateCDNOptimization := app.NewUpdateCDNOptimization(pool, provider)
	updateCDNDeveloperMode := app.NewUpdateCDNDeveloperMode(pool, provider)
	updateCDNMaintenanceMode := app.NewUpdateCDNMaintenanceMode(pool, provider)
	updateCDNQueryStringSetting := app.NewUpdateCDNQueryStringSetting(pool, provider)
	updateCDNOriginOffline := app.NewUpdateCDNOriginOffline(pool, provider)

	listCDNBulklists := app.NewListCDNBulklists(pool, provider)
	createCDNBulklist := app.NewCreateCDNBulklist(pool, provider)
	getCDNBulklist := app.NewGetCDNBulklist(pool, provider)
	updateCDNBulklist := app.NewUpdateCDNBulklist(pool, provider)
	deleteCDNBulklist := app.NewDeleteCDNBulklist(pool, provider)
	listCDNFirewallCountries := app.NewListCDNFirewallCountries(pool, provider)

	updateCDNCacheTTL := app.NewUpdateCDNCacheTTL(pool, provider)
	updateCDNCacheRule := app.NewUpdateCDNCacheRule(pool, provider)
	updateCDNCacheUserAgentSetting := app.NewUpdateCDNCacheUserAgentSetting(pool, provider)
	getCDNCacheSettings := app.NewGetCDNCacheSettings(pool, provider)
	listCDNCacheEntries := app.NewListCDNCacheEntries(pool, provider)
	purgeCDNCache := app.NewPurgeCDNCache(pool, provider)
	getCDNCacheEntry := app.NewGetCDNCacheEntry(pool, provider)

	listCDNAccessRules := app.NewListCDNAccessRules(pool, provider)
	createCDNAccessRule := app.NewCreateCDNAccessRule(pool, provider)
	getCDNAccessRule := app.NewGetCDNAccessRule(pool, provider)
	updateCDNAccessRule := app.NewUpdateCDNAccessRule(pool, provider)
	deleteCDNAccessRule := app.NewDeleteCDNAccessRule(pool, provider)
	getCDNIPReputation := app.NewGetCDNIPReputation(pool, provider)
	updateCDNIPReputation := app.NewUpdateCDNIPReputation(pool, provider)
	getCDNDDoSActions := app.NewGetCDNDDoSActions(pool, provider)
	updateCDNDDoSActions := app.NewUpdateCDNDDoSActions(pool, provider)

	listCDNLoadBalances := app.NewListCDNLoadBalances(pool, provider)
	createCDNLoadBalance := app.NewCreateCDNLoadBalance(pool, provider)
	getCDNLoadBalance := app.NewGetCDNLoadBalance(pool, provider)
	updateCDNLoadBalance := app.NewUpdateCDNLoadBalance(pool, provider)
	deleteCDNLoadBalance := app.NewDeleteCDNLoadBalance(pool, provider)
	listCDNLoadBalanceServers := app.NewListCDNLoadBalanceServers(pool, provider)
	createCDNLoadBalanceServer := app.NewCreateCDNLoadBalanceServer(pool, provider)
	getCDNLoadBalanceServer := app.NewGetCDNLoadBalanceServer(pool, provider)
	updateCDNLoadBalanceServer := app.NewUpdateCDNLoadBalanceServer(pool, provider)
	deleteCDNLoadBalanceServer := app.NewDeleteCDNLoadBalanceServer(pool, provider)

	getCDNModSecStatus := app.NewGetCDNModSecStatus(pool, provider)
	updateCDNModSecStatus := app.NewUpdateCDNModSecStatus(pool, provider)
	listCDNModSecData := app.NewListCDNModSecData(pool, provider)
	createCDNModSecData := app.NewCreateCDNModSecData(pool, provider)
	getCDNModSecData := app.NewGetCDNModSecData(pool, provider)
	updateCDNModSecData := app.NewUpdateCDNModSecData(pool, provider)
	deleteCDNModSecData := app.NewDeleteCDNModSecData(pool, provider)
	listCDNModSecRules := app.NewListCDNModSecRules(pool, provider)
	createCDNModSecRule := app.NewCreateCDNModSecRule(pool, provider)
	getCDNModSecRule := app.NewGetCDNModSecRule(pool, provider)
	updateCDNModSecRule := app.NewUpdateCDNModSecRule(pool, provider)
	deleteCDNModSecRule := app.NewDeleteCDNModSecRule(pool, provider)

	getCDNHTTPSConvertor := app.NewGetCDNHTTPSConvertor(pool, provider)
	updateCDNHTTPSConvertor := app.NewUpdateCDNHTTPSConvertor(pool, provider)
	getCDNEdgeToUpstreamConnection := app.NewGetCDNEdgeToUpstreamConnection(pool, provider)
	updateCDNEdgeToUpstreamConnection := app.NewUpdateCDNEdgeToUpstreamConnection(pool, provider)
	getCDNWWWRedirection := app.NewGetCDNWWWRedirection(pool, provider)
	updateCDNWWWRedirection := app.NewUpdateCDNWWWRedirection(pool, provider)
	getCDNWebSocket := app.NewGetCDNWebSocket(pool, provider)
	updateCDNWebSocket := app.NewUpdateCDNWebSocket(pool, provider)

	listCDNOriginRules := app.NewListCDNOriginRules(pool, provider)
	createCDNOriginRule := app.NewCreateCDNOriginRule(pool, provider)
	getCDNOriginRule := app.NewGetCDNOriginRule(pool, provider)
	updateCDNOriginRule := app.NewUpdateCDNOriginRule(pool, provider)
	deleteCDNOriginRule := app.NewDeleteCDNOriginRule(pool, provider)
	toggleCDNOriginRule := app.NewToggleCDNOriginRule(pool, provider)
	listCDNPageRules := app.NewListCDNPageRules(pool, provider)
	createCDNPageRule := app.NewCreateCDNPageRule(pool, provider)
	getCDNPageRule := app.NewGetCDNPageRule(pool, provider)
	updateCDNPageRule := app.NewUpdateCDNPageRule(pool, provider)
	deleteCDNPageRule := app.NewDeleteCDNPageRule(pool, provider)
	listCDNTransformRules := app.NewListCDNTransformRules(pool, provider)
	createCDNTransformRule := app.NewCreateCDNTransformRule(pool, provider)
	getCDNTransformRule := app.NewGetCDNTransformRule(pool, provider)
	updateCDNTransformRule := app.NewUpdateCDNTransformRule(pool, provider)
	deleteCDNTransformRule := app.NewDeleteCDNTransformRule(pool, provider)
	toggleCDNTransformRule := app.NewToggleCDNTransformRule(pool, provider)

	listCDNRateLimitRules := app.NewListCDNRateLimitRules(pool, provider)
	createCDNRateLimitRule := app.NewCreateCDNRateLimitRule(pool, provider)
	getCDNRateLimitRule := app.NewGetCDNRateLimitRule(pool, provider)
	updateCDNRateLimitRule := app.NewUpdateCDNRateLimitRule(pool, provider)
	deleteCDNRateLimitRule := app.NewDeleteCDNRateLimitRule(pool, provider)
	updateCDNRateLimitRulePriority := app.NewUpdateCDNRateLimitRulePriority(pool, provider)
	getCDNUpstreamErrors := app.NewGetCDNUpstreamErrors(pool, provider)
	updateCDNUpstreamErrors := app.NewUpdateCDNUpstreamErrors(pool, provider)

	getCDNAccessLog := app.NewGetCDNAccessLog(pool, provider)
	getCDNSecurityLog := app.NewGetCDNSecurityLog(pool, provider)
	getCDNErrorLog := app.NewGetCDNErrorLog(pool, provider)
	getCDNWAFLog := app.NewGetCDNWAFLog(pool, provider)
	getCDNTopVisitors := app.NewGetCDNTopVisitors(pool, provider)
	getCDNMonthlyTrafficUsage := app.NewGetCDNMonthlyTrafficUsage(pool, provider)
	getCDNMinTLSVersion := app.NewGetCDNMinTLSVersion(pool, provider)
	updateCDNMinTLSVersion := app.NewUpdateCDNMinTLSVersion(pool, provider)
	listCDNCertificates := app.NewListCDNCertificates(pool, provider)
	getCDNHSTS := app.NewGetCDNHSTS(pool, provider)
	updateCDNHSTS := app.NewUpdateCDNHSTS(pool, provider)

	pool.Register(domain.JobTypeProvisionServer, provisionServer.Handle)
	pool.Register(domain.JobTypeCreateSnapshot, createSnapshot.Handle)
	pool.Register(domain.JobTypeRestoreVM, restoreVM.Handle)
	pool.Register(domain.JobTypeProvisionLoadBalancer, provisionLoadBalancer.Handle)

	// Each long operation holds the caller's credentials in memory until the
	// job can no longer be re-attempted; the pool calls these once it is done
	// with the job, whichever way it ended.
	pool.RegisterSettled(domain.JobTypeProvisionServer, provisionServer.Settled)
	pool.RegisterSettled(domain.JobTypeCreateSnapshot, createSnapshot.Settled)
	pool.RegisterSettled(domain.JobTypeRestoreVM, restoreVM.Settled)
	pool.RegisterSettled(domain.JobTypeProvisionLoadBalancer, provisionLoadBalancer.Settled)
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

		GetCDNAntivirusStatus:       getCDNAntivirusStatus,
		UpdateCDNAntivirusStatus:    updateCDNAntivirusStatus,
		GetCDNDNSSecStatus:          getCDNDNSSecStatus,
		UpdateCDNDNSSecStatus:       updateCDNDNSSecStatus,
		GetCDNOptimizationStatus:    getCDNOptimizationStatus,
		UpdateCDNOptimization:       updateCDNOptimization,
		UpdateCDNDeveloperMode:      updateCDNDeveloperMode,
		UpdateCDNMaintenanceMode:    updateCDNMaintenanceMode,
		UpdateCDNQueryStringSetting: updateCDNQueryStringSetting,
		UpdateCDNOriginOffline:      updateCDNOriginOffline,

		ListCDNBulklists:         listCDNBulklists,
		CreateCDNBulklist:        createCDNBulklist,
		GetCDNBulklist:           getCDNBulklist,
		UpdateCDNBulklist:        updateCDNBulklist,
		DeleteCDNBulklist:        deleteCDNBulklist,
		ListCDNFirewallCountries: listCDNFirewallCountries,

		UpdateCDNCacheTTL:              updateCDNCacheTTL,
		UpdateCDNCacheRule:             updateCDNCacheRule,
		UpdateCDNCacheUserAgentSetting: updateCDNCacheUserAgentSetting,
		GetCDNCacheSettings:            getCDNCacheSettings,
		ListCDNCacheEntries:            listCDNCacheEntries,
		PurgeCDNCache:                  purgeCDNCache,
		GetCDNCacheEntry:               getCDNCacheEntry,

		ListCDNAccessRules:    listCDNAccessRules,
		CreateCDNAccessRule:   createCDNAccessRule,
		GetCDNAccessRule:      getCDNAccessRule,
		UpdateCDNAccessRule:   updateCDNAccessRule,
		DeleteCDNAccessRule:   deleteCDNAccessRule,
		GetCDNIPReputation:    getCDNIPReputation,
		UpdateCDNIPReputation: updateCDNIPReputation,
		GetCDNDDoSActions:     getCDNDDoSActions,
		UpdateCDNDDoSActions:  updateCDNDDoSActions,

		ListCDNLoadBalances:        listCDNLoadBalances,
		CreateCDNLoadBalance:       createCDNLoadBalance,
		GetCDNLoadBalance:          getCDNLoadBalance,
		UpdateCDNLoadBalance:       updateCDNLoadBalance,
		DeleteCDNLoadBalance:       deleteCDNLoadBalance,
		ListCDNLoadBalanceServers:  listCDNLoadBalanceServers,
		CreateCDNLoadBalanceServer: createCDNLoadBalanceServer,
		GetCDNLoadBalanceServer:    getCDNLoadBalanceServer,
		UpdateCDNLoadBalanceServer: updateCDNLoadBalanceServer,
		DeleteCDNLoadBalanceServer: deleteCDNLoadBalanceServer,

		GetCDNModSecStatus:    getCDNModSecStatus,
		UpdateCDNModSecStatus: updateCDNModSecStatus,
		ListCDNModSecData:     listCDNModSecData,
		CreateCDNModSecData:   createCDNModSecData,
		GetCDNModSecData:      getCDNModSecData,
		UpdateCDNModSecData:   updateCDNModSecData,
		DeleteCDNModSecData:   deleteCDNModSecData,
		ListCDNModSecRules:    listCDNModSecRules,
		CreateCDNModSecRule:   createCDNModSecRule,
		GetCDNModSecRule:      getCDNModSecRule,
		UpdateCDNModSecRule:   updateCDNModSecRule,
		DeleteCDNModSecRule:   deleteCDNModSecRule,

		GetCDNHTTPSConvertor:              getCDNHTTPSConvertor,
		UpdateCDNHTTPSConvertor:           updateCDNHTTPSConvertor,
		GetCDNEdgeToUpstreamConnection:    getCDNEdgeToUpstreamConnection,
		UpdateCDNEdgeToUpstreamConnection: updateCDNEdgeToUpstreamConnection,
		GetCDNWWWRedirection:              getCDNWWWRedirection,
		UpdateCDNWWWRedirection:           updateCDNWWWRedirection,
		GetCDNWebSocket:                   getCDNWebSocket,
		UpdateCDNWebSocket:                updateCDNWebSocket,

		ListCDNOriginRules:  listCDNOriginRules,
		CreateCDNOriginRule: createCDNOriginRule,
		GetCDNOriginRule:    getCDNOriginRule,
		UpdateCDNOriginRule: updateCDNOriginRule,
		DeleteCDNOriginRule: deleteCDNOriginRule,
		ToggleCDNOriginRule: toggleCDNOriginRule,

		ListCDNPageRules:  listCDNPageRules,
		CreateCDNPageRule: createCDNPageRule,
		GetCDNPageRule:    getCDNPageRule,
		UpdateCDNPageRule: updateCDNPageRule,
		DeleteCDNPageRule: deleteCDNPageRule,

		ListCDNTransformRules:  listCDNTransformRules,
		CreateCDNTransformRule: createCDNTransformRule,
		GetCDNTransformRule:    getCDNTransformRule,
		UpdateCDNTransformRule: updateCDNTransformRule,
		DeleteCDNTransformRule: deleteCDNTransformRule,
		ToggleCDNTransformRule: toggleCDNTransformRule,

		ListCDNRateLimitRules:          listCDNRateLimitRules,
		CreateCDNRateLimitRule:         createCDNRateLimitRule,
		GetCDNRateLimitRule:            getCDNRateLimitRule,
		UpdateCDNRateLimitRule:         updateCDNRateLimitRule,
		DeleteCDNRateLimitRule:         deleteCDNRateLimitRule,
		UpdateCDNRateLimitRulePriority: updateCDNRateLimitRulePriority,
		GetCDNUpstreamErrors:           getCDNUpstreamErrors,
		UpdateCDNUpstreamErrors:        updateCDNUpstreamErrors,

		GetCDNAccessLog:           getCDNAccessLog,
		GetCDNSecurityLog:         getCDNSecurityLog,
		GetCDNErrorLog:            getCDNErrorLog,
		GetCDNWAFLog:              getCDNWAFLog,
		GetCDNTopVisitors:         getCDNTopVisitors,
		GetCDNMonthlyTrafficUsage: getCDNMonthlyTrafficUsage,
		GetCDNMinTLSVersion:       getCDNMinTLSVersion,
		UpdateCDNMinTLSVersion:    updateCDNMinTLSVersion,
		ListCDNCertificates:       listCDNCertificates,
		GetCDNHSTS:                getCDNHSTS,
		UpdateCDNHSTS:             updateCDNHSTS,
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
