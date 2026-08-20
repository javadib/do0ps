package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
)

// UseCases collects the application entry points this adapter exposes.
type UseCases struct {
	ProvisionServer    *app.ProvisionServer
	ListServers        *app.ListServers
	GetServer          *app.GetServer
	DeleteServer       *app.DeleteServer
	RegisterSSHKey     *app.RegisterSSHKey
	ListSSHKeys        *app.ListSSHKeys
	DeleteSSHKey       *app.DeleteSSHKey
	GetOperationStatus *app.GetOperationStatus

	CreateSnapshot *app.CreateSnapshot
	ListSnapshots  *app.ListSnapshots
	DeleteSnapshot *app.DeleteSnapshot
	RestoreVM      *app.RestoreVM
	CreateVPC      *app.CreateVPC
	ListVPCs       *app.ListVPCs
	GetVPC         *app.GetVPC
	DeleteVPC      *app.DeleteVPC

	CreateCDNZone        *app.CreateCDNZone
	ListCDNZones         *app.ListCDNZones
	GetCDNZone           *app.GetCDNZone
	DeleteCDNZone        *app.DeleteCDNZone
	ListCDNZonePlans     *app.ListCDNZonePlans
	GetNameserverRecords *app.GetNameserverRecords
	ListDNSRecords       *app.ListDNSRecords
	CreateDNSRecord      *app.CreateDNSRecord
	UpdateDNSRecord      *app.UpdateDNSRecord
	DeleteDNSRecord      *app.DeleteDNSRecord
	ReserveIP            *app.ReserveIP
	ReleaseIP            *app.ReleaseIP
	AssignIPToServer     *app.AssignIPToServer
	UnassignIP           *app.UnassignIP

	ListSSLProducts       *app.ListSSLProducts
	CreateSSLOrder        *app.CreateSSLOrder
	ProcessSSLOrder       *app.ProcessSSLOrder
	GetSSLChallenge       *app.GetSSLChallenge
	ReloadSSLChallenge    *app.ReloadSSLChallenge
	VerifySSLChallenge    *app.VerifySSLChallenge
	GetSSLCertificate     *app.GetSSLCertificate
	ReissueSSLCertificate *app.ReissueSSLCertificate

	CreateFirewall *app.CreateFirewall
	GetFirewall    *app.GetFirewall
	ListFirewalls  *app.ListFirewalls
	UpdateFirewall *app.UpdateFirewall
	DeleteFirewall *app.DeleteFirewall

	ProvisionLoadBalancer *app.ProvisionLoadBalancer
	GetLoadBalancer       *app.GetLoadBalancer
	ListLoadBalancers     *app.ListLoadBalancers
	UpdateLoadBalancer    *app.UpdateLoadBalancer
	DeleteLoadBalancer    *app.DeleteLoadBalancer

	// CDN capabilities beyond zone/DNS (issue #24).
	GetCDNAntivirusStatus       *app.GetCDNAntivirusStatus
	UpdateCDNAntivirusStatus    *app.UpdateCDNAntivirusStatus
	GetCDNDNSSecStatus          *app.GetCDNDNSSecStatus
	UpdateCDNDNSSecStatus       *app.UpdateCDNDNSSecStatus
	GetCDNOptimizationStatus    *app.GetCDNOptimizationStatus
	UpdateCDNOptimization       *app.UpdateCDNOptimization
	UpdateCDNDeveloperMode      *app.UpdateCDNDeveloperMode
	UpdateCDNMaintenanceMode    *app.UpdateCDNMaintenanceMode
	UpdateCDNQueryStringSetting *app.UpdateCDNQueryStringSetting
	UpdateCDNOriginOffline      *app.UpdateCDNOriginOffline

	ListCDNBulklists         *app.ListCDNBulklists
	CreateCDNBulklist        *app.CreateCDNBulklist
	GetCDNBulklist           *app.GetCDNBulklist
	UpdateCDNBulklist        *app.UpdateCDNBulklist
	DeleteCDNBulklist        *app.DeleteCDNBulklist
	ListCDNFirewallCountries *app.ListCDNFirewallCountries

	UpdateCDNCacheTTL              *app.UpdateCDNCacheTTL
	UpdateCDNCacheRule             *app.UpdateCDNCacheRule
	UpdateCDNCacheUserAgentSetting *app.UpdateCDNCacheUserAgentSetting
	GetCDNCacheSettings            *app.GetCDNCacheSettings
	ListCDNCacheEntries            *app.ListCDNCacheEntries
	PurgeCDNCache                  *app.PurgeCDNCache
	GetCDNCacheEntry               *app.GetCDNCacheEntry

	ListCDNAccessRules    *app.ListCDNAccessRules
	CreateCDNAccessRule   *app.CreateCDNAccessRule
	GetCDNAccessRule      *app.GetCDNAccessRule
	UpdateCDNAccessRule   *app.UpdateCDNAccessRule
	DeleteCDNAccessRule   *app.DeleteCDNAccessRule
	GetCDNIPReputation    *app.GetCDNIPReputation
	UpdateCDNIPReputation *app.UpdateCDNIPReputation
	GetCDNDDoSActions     *app.GetCDNDDoSActions
	UpdateCDNDDoSActions  *app.UpdateCDNDDoSActions

	ListCDNLoadBalances        *app.ListCDNLoadBalances
	CreateCDNLoadBalance       *app.CreateCDNLoadBalance
	GetCDNLoadBalance          *app.GetCDNLoadBalance
	UpdateCDNLoadBalance       *app.UpdateCDNLoadBalance
	DeleteCDNLoadBalance       *app.DeleteCDNLoadBalance
	ListCDNLoadBalanceServers  *app.ListCDNLoadBalanceServers
	CreateCDNLoadBalanceServer *app.CreateCDNLoadBalanceServer
	GetCDNLoadBalanceServer    *app.GetCDNLoadBalanceServer
	UpdateCDNLoadBalanceServer *app.UpdateCDNLoadBalanceServer
	DeleteCDNLoadBalanceServer *app.DeleteCDNLoadBalanceServer

	GetCDNModSecStatus    *app.GetCDNModSecStatus
	UpdateCDNModSecStatus *app.UpdateCDNModSecStatus
	ListCDNModSecData     *app.ListCDNModSecData
	CreateCDNModSecData   *app.CreateCDNModSecData
	GetCDNModSecData      *app.GetCDNModSecData
	UpdateCDNModSecData   *app.UpdateCDNModSecData
	DeleteCDNModSecData   *app.DeleteCDNModSecData
	ListCDNModSecRules    *app.ListCDNModSecRules
	CreateCDNModSecRule   *app.CreateCDNModSecRule
	GetCDNModSecRule      *app.GetCDNModSecRule
	UpdateCDNModSecRule   *app.UpdateCDNModSecRule
	DeleteCDNModSecRule   *app.DeleteCDNModSecRule

	GetCDNHTTPSConvertor              *app.GetCDNHTTPSConvertor
	UpdateCDNHTTPSConvertor           *app.UpdateCDNHTTPSConvertor
	GetCDNEdgeToUpstreamConnection    *app.GetCDNEdgeToUpstreamConnection
	UpdateCDNEdgeToUpstreamConnection *app.UpdateCDNEdgeToUpstreamConnection
	GetCDNWWWRedirection              *app.GetCDNWWWRedirection
	UpdateCDNWWWRedirection           *app.UpdateCDNWWWRedirection
	GetCDNWebSocket                   *app.GetCDNWebSocket
	UpdateCDNWebSocket                *app.UpdateCDNWebSocket

	ListCDNOriginRules  *app.ListCDNOriginRules
	CreateCDNOriginRule *app.CreateCDNOriginRule
	GetCDNOriginRule    *app.GetCDNOriginRule
	UpdateCDNOriginRule *app.UpdateCDNOriginRule
	DeleteCDNOriginRule *app.DeleteCDNOriginRule
	ToggleCDNOriginRule *app.ToggleCDNOriginRule

	ListCDNPageRules  *app.ListCDNPageRules
	CreateCDNPageRule *app.CreateCDNPageRule
	GetCDNPageRule    *app.GetCDNPageRule
	UpdateCDNPageRule *app.UpdateCDNPageRule
	DeleteCDNPageRule *app.DeleteCDNPageRule

	ListCDNTransformRules  *app.ListCDNTransformRules
	CreateCDNTransformRule *app.CreateCDNTransformRule
	GetCDNTransformRule    *app.GetCDNTransformRule
	UpdateCDNTransformRule *app.UpdateCDNTransformRule
	DeleteCDNTransformRule *app.DeleteCDNTransformRule
	ToggleCDNTransformRule *app.ToggleCDNTransformRule

	ListCDNRateLimitRules          *app.ListCDNRateLimitRules
	CreateCDNRateLimitRule         *app.CreateCDNRateLimitRule
	GetCDNRateLimitRule            *app.GetCDNRateLimitRule
	UpdateCDNRateLimitRule         *app.UpdateCDNRateLimitRule
	DeleteCDNRateLimitRule         *app.DeleteCDNRateLimitRule
	UpdateCDNRateLimitRulePriority *app.UpdateCDNRateLimitRulePriority
	GetCDNUpstreamErrors           *app.GetCDNUpstreamErrors
	UpdateCDNUpstreamErrors        *app.UpdateCDNUpstreamErrors

	GetCDNAccessLog           *app.GetCDNAccessLog
	GetCDNSecurityLog         *app.GetCDNSecurityLog
	GetCDNErrorLog            *app.GetCDNErrorLog
	GetCDNWAFLog              *app.GetCDNWAFLog
	GetCDNTopVisitors         *app.GetCDNTopVisitors
	GetCDNMonthlyTrafficUsage *app.GetCDNMonthlyTrafficUsage
	GetCDNMinTLSVersion       *app.GetCDNMinTLSVersion
	UpdateCDNMinTLSVersion    *app.UpdateCDNMinTLSVersion
	ListCDNCertificates       *app.ListCDNCertificates
	GetCDNHSTS                *app.GetCDNHSTS
	UpdateCDNHSTS             *app.UpdateCDNHSTS

	// ArvanCloud domain onboarding and lifecycle (issue #62).
	CreateArvanCloudDomain           *app.CreateArvanCloudDomain
	ListArvanCloudDomains            *app.ListArvanCloudDomains
	GetArvanCloudDomain              *app.GetArvanCloudDomain
	DeleteArvanCloudDomain           *app.DeleteArvanCloudDomain
	SetArvanCloudNSKeys              *app.SetArvanCloudNSKeys
	ResetArvanCloudNSKeys            *app.ResetArvanCloudNSKeys
	CheckArvanCloudNSStatus          *app.CheckArvanCloudNSStatus
	UseArvanCloudOptionalNSKeys      *app.UseArvanCloudOptionalNSKeys
	SetArvanCloudCnameTarget         *app.SetArvanCloudCnameTarget
	ResetArvanCloudCnameTarget       *app.ResetArvanCloudCnameTarget
	ConvertArvanCloudToCnameSetup    *app.ConvertArvanCloudToCnameSetup
	CheckArvanCloudCnameStatus       *app.CheckArvanCloudCnameStatus
	CloneArvanCloudDomainConfig      *app.CloneArvanCloudDomainConfig
	RegenerateArvanCloudDomainConfig *app.RegenerateArvanCloudDomainConfig
	HoldArvanCloudDomain             *app.HoldArvanCloudDomain
	UnholdArvanCloudDomain           *app.UnholdArvanCloudDomain
}

// credentialProperties are repeated on every provider-touching tool: the
// chatbot session holds the user's provider credentials and passes them on
// each call, since this server stores none (AGENTS.md 4.2).
func credentialProperties() map[string]any {
	return map[string]any{
		"api_key": map[string]any{
			"type":        "string",
			"description": "Provider API key from the user's provider account. Never stored by this server; supply it on every call.",
		},
		"secret_key": map[string]any{
			"type":        "string",
			"description": "Provider secret key, if the provider issues a key pair. Leave empty when the provider uses a single API key.",
		},
	}
}

type credentialArgs struct {
	APIKey    string `json:"api_key"`
	SecretKey string `json:"secret_key"`
}

func (a credentialArgs) domain() domain.ProviderCredentials {
	return domain.ProviderCredentials{APIKey: a.APIKey, SecretKey: a.SecretKey}
}

// Tools builds the tool set backed by the given use cases.
func Tools(uc UseCases) []Tool {
	return []Tool{
		PingTool(),
		createServerTool(uc.ProvisionServer),
		listServersTool(uc.ListServers),
		getServerTool(uc.GetServer),
		deleteServerTool(uc.DeleteServer),
		registerSSHKeyTool(uc.RegisterSSHKey),
		listSSHKeysTool(uc.ListSSHKeys),
		deleteSSHKeyTool(uc.DeleteSSHKey),
		getOperationStatusTool(uc.GetOperationStatus),
		createSnapshotTool(uc.CreateSnapshot),
		listSnapshotsTool(uc.ListSnapshots),
		deleteSnapshotTool(uc.DeleteSnapshot),
		restoreVMTool(uc.RestoreVM),
		createVPCTool(uc.CreateVPC),
		listVPCsTool(uc.ListVPCs),
		getVPCTool(uc.GetVPC),
		deleteVPCTool(uc.DeleteVPC),
		createCDNZoneTool(uc.CreateCDNZone),
		listCDNZonesTool(uc.ListCDNZones),
		getCDNZoneTool(uc.GetCDNZone),
		deleteCDNZoneTool(uc.DeleteCDNZone),
		listCDNZonePlansTool(uc.ListCDNZonePlans),
		getNameserverRecordsTool(uc.GetNameserverRecords),
		listDNSRecordsTool(uc.ListDNSRecords),
		createDNSRecordTool(uc.CreateDNSRecord),
		updateDNSRecordTool(uc.UpdateDNSRecord),
		deleteDNSRecordTool(uc.DeleteDNSRecord),
		reserveIPTool(uc.ReserveIP),
		releaseIPTool(uc.ReleaseIP),
		assignIPToServerTool(uc.AssignIPToServer),
		unassignIPTool(uc.UnassignIP),
		listSSLProductsTool(uc.ListSSLProducts),
		createSSLOrderTool(uc.CreateSSLOrder),
		processSSLOrderTool(uc.ProcessSSLOrder),
		getSSLChallengeTool(uc.GetSSLChallenge),
		reloadSSLChallengeTool(uc.ReloadSSLChallenge),
		verifySSLChallengeTool(uc.VerifySSLChallenge),
		getSSLCertificateTool(uc.GetSSLCertificate),
		reissueSSLCertificateTool(uc.ReissueSSLCertificate),
		createFirewallTool(uc.CreateFirewall),
		getFirewallTool(uc.GetFirewall),
		listFirewallsTool(uc.ListFirewalls),
		updateFirewallTool(uc.UpdateFirewall),
		deleteFirewallTool(uc.DeleteFirewall),
		createLoadBalancerTool(uc.ProvisionLoadBalancer),
		getLoadBalancerTool(uc.GetLoadBalancer),
		listLoadBalancersTool(uc.ListLoadBalancers),
		updateLoadBalancerTool(uc.UpdateLoadBalancer),
		deleteLoadBalancerTool(uc.DeleteLoadBalancer),

		// CDN capabilities beyond zone/DNS (issue #24).
		getCDNAntivirusStatusTool(uc.GetCDNAntivirusStatus),
		updateCDNAntivirusStatusTool(uc.UpdateCDNAntivirusStatus),
		getCDNDNSSecStatusTool(uc.GetCDNDNSSecStatus),
		updateCDNDNSSecStatusTool(uc.UpdateCDNDNSSecStatus),
		getCDNOptimizationStatusTool(uc.GetCDNOptimizationStatus),
		updateCDNOptimizationTool(uc.UpdateCDNOptimization),
		updateCDNDeveloperModeTool(uc.UpdateCDNDeveloperMode),
		updateCDNMaintenanceModeTool(uc.UpdateCDNMaintenanceMode),
		updateCDNQueryStringSettingTool(uc.UpdateCDNQueryStringSetting),
		updateCDNOriginOfflineTool(uc.UpdateCDNOriginOffline),

		listCDNBulklistsTool(uc.ListCDNBulklists),
		createCDNBulklistTool(uc.CreateCDNBulklist),
		getCDNBulklistTool(uc.GetCDNBulklist),
		updateCDNBulklistTool(uc.UpdateCDNBulklist),
		deleteCDNBulklistTool(uc.DeleteCDNBulklist),
		listCDNFirewallCountriesTool(uc.ListCDNFirewallCountries),

		updateCDNCacheTTLTool(uc.UpdateCDNCacheTTL),
		updateCDNCacheRuleTool(uc.UpdateCDNCacheRule),
		updateCDNCacheUserAgentTool(uc.UpdateCDNCacheUserAgentSetting),
		getCDNCacheSettingsTool(uc.GetCDNCacheSettings),
		listCDNCacheEntriesTool(uc.ListCDNCacheEntries),
		purgeCDNCacheTool(uc.PurgeCDNCache),
		getCDNCacheEntryTool(uc.GetCDNCacheEntry),

		listCDNAccessRulesTool(uc.ListCDNAccessRules),
		createCDNAccessRuleTool(uc.CreateCDNAccessRule),
		getCDNAccessRuleTool(uc.GetCDNAccessRule),
		updateCDNAccessRuleTool(uc.UpdateCDNAccessRule),
		deleteCDNAccessRuleTool(uc.DeleteCDNAccessRule),
		getCDNIPReputationTool(uc.GetCDNIPReputation),
		updateCDNIPReputationTool(uc.UpdateCDNIPReputation),
		getCDNDDoSActionsTool(uc.GetCDNDDoSActions),
		updateCDNDDoSActionsTool(uc.UpdateCDNDDoSActions),

		listCDNLoadBalancesTool(uc.ListCDNLoadBalances),
		createCDNLoadBalanceTool(uc.CreateCDNLoadBalance),
		getCDNLoadBalanceTool(uc.GetCDNLoadBalance),
		updateCDNLoadBalanceTool(uc.UpdateCDNLoadBalance),
		deleteCDNLoadBalanceTool(uc.DeleteCDNLoadBalance),
		listCDNLoadBalanceServersTool(uc.ListCDNLoadBalanceServers),
		createCDNLoadBalanceServerTool(uc.CreateCDNLoadBalanceServer),
		getCDNLoadBalanceServerTool(uc.GetCDNLoadBalanceServer),
		updateCDNLoadBalanceServerTool(uc.UpdateCDNLoadBalanceServer),
		deleteCDNLoadBalanceServerTool(uc.DeleteCDNLoadBalanceServer),

		getCDNModSecStatusTool(uc.GetCDNModSecStatus),
		updateCDNModSecStatusTool(uc.UpdateCDNModSecStatus),
		listCDNModSecDataTool(uc.ListCDNModSecData),
		createCDNModSecDataTool(uc.CreateCDNModSecData),
		getCDNModSecDataTool(uc.GetCDNModSecData),
		updateCDNModSecDataTool(uc.UpdateCDNModSecData),
		deleteCDNModSecDataTool(uc.DeleteCDNModSecData),
		listCDNModSecRulesTool(uc.ListCDNModSecRules),
		createCDNModSecRuleTool(uc.CreateCDNModSecRule),
		getCDNModSecRuleTool(uc.GetCDNModSecRule),
		updateCDNModSecRuleTool(uc.UpdateCDNModSecRule),
		deleteCDNModSecRuleTool(uc.DeleteCDNModSecRule),

		getCDNHTTPSConvertorTool(uc.GetCDNHTTPSConvertor),
		updateCDNHTTPSConvertorTool(uc.UpdateCDNHTTPSConvertor),
		getCDNEdgeToUpstreamConnectionTool(uc.GetCDNEdgeToUpstreamConnection),
		updateCDNEdgeToUpstreamConnectionTool(uc.UpdateCDNEdgeToUpstreamConnection),
		getCDNWWWRedirectionTool(uc.GetCDNWWWRedirection),
		updateCDNWWWRedirectionTool(uc.UpdateCDNWWWRedirection),
		getCDNWebSocketTool(uc.GetCDNWebSocket),
		updateCDNWebSocketTool(uc.UpdateCDNWebSocket),

		listCDNOriginRulesTool(uc.ListCDNOriginRules),
		createCDNOriginRuleTool(uc.CreateCDNOriginRule),
		getCDNOriginRuleTool(uc.GetCDNOriginRule),
		updateCDNOriginRuleTool(uc.UpdateCDNOriginRule),
		deleteCDNOriginRuleTool(uc.DeleteCDNOriginRule),
		toggleCDNOriginRuleTool(uc.ToggleCDNOriginRule),
		listCDNPageRulesTool(uc.ListCDNPageRules),
		createCDNPageRuleTool(uc.CreateCDNPageRule),
		getCDNPageRuleTool(uc.GetCDNPageRule),
		updateCDNPageRuleTool(uc.UpdateCDNPageRule),
		deleteCDNPageRuleTool(uc.DeleteCDNPageRule),
		listCDNTransformRulesTool(uc.ListCDNTransformRules),
		createCDNTransformRuleTool(uc.CreateCDNTransformRule),
		getCDNTransformRuleTool(uc.GetCDNTransformRule),
		updateCDNTransformRuleTool(uc.UpdateCDNTransformRule),
		deleteCDNTransformRuleTool(uc.DeleteCDNTransformRule),
		toggleCDNTransformRuleTool(uc.ToggleCDNTransformRule),

		listCDNRateLimitRulesTool(uc.ListCDNRateLimitRules),
		createCDNRateLimitRuleTool(uc.CreateCDNRateLimitRule),
		getCDNRateLimitRuleTool(uc.GetCDNRateLimitRule),
		updateCDNRateLimitRuleTool(uc.UpdateCDNRateLimitRule),
		deleteCDNRateLimitRuleTool(uc.DeleteCDNRateLimitRule),
		updateCDNRateLimitRulePriorityTool(uc.UpdateCDNRateLimitRulePriority),
		getCDNUpstreamErrorsTool(uc.GetCDNUpstreamErrors),
		updateCDNUpstreamErrorsTool(uc.UpdateCDNUpstreamErrors),

		getCDNAccessLogTool(uc.GetCDNAccessLog),
		getCDNSecurityLogTool(uc.GetCDNSecurityLog),
		getCDNErrorLogTool(uc.GetCDNErrorLog),
		getCDNWAFLogTool(uc.GetCDNWAFLog),
		getCDNTopVisitorsTool(uc.GetCDNTopVisitors),
		getCDNMonthlyTrafficUsageTool(uc.GetCDNMonthlyTrafficUsage),
		getCDNMinTLSVersionTool(uc.GetCDNMinTLSVersion),
		updateCDNMinTLSVersionTool(uc.UpdateCDNMinTLSVersion),
		listCDNCertificatesTool(uc.ListCDNCertificates),
		getCDNHSTSTool(uc.GetCDNHSTS),
		updateCDNHSTSTool(uc.UpdateCDNHSTS),

		createArvanCloudDomainTool(uc.CreateArvanCloudDomain),
		listArvanCloudDomainsTool(uc.ListArvanCloudDomains),
		getArvanCloudDomainTool(uc.GetArvanCloudDomain),
		deleteArvanCloudDomainTool(uc.DeleteArvanCloudDomain),
		setArvanCloudNSKeysTool(uc.SetArvanCloudNSKeys),
		resetArvanCloudNSKeysTool(uc.ResetArvanCloudNSKeys),
		checkArvanCloudNSStatusTool(uc.CheckArvanCloudNSStatus),
		useArvanCloudOptionalNSKeysTool(uc.UseArvanCloudOptionalNSKeys),
		setArvanCloudCnameTargetTool(uc.SetArvanCloudCnameTarget),
		resetArvanCloudCnameTargetTool(uc.ResetArvanCloudCnameTarget),
		convertArvanCloudToCnameSetupTool(uc.ConvertArvanCloudToCnameSetup),
		checkArvanCloudCnameStatusTool(uc.CheckArvanCloudCnameStatus),
		cloneArvanCloudDomainConfigTool(uc.CloneArvanCloudDomainConfig),
		regenerateArvanCloudDomainConfigTool(uc.RegenerateArvanCloudDomainConfig),
		holdArvanCloudDomainTool(uc.HoldArvanCloudDomain),
		unholdArvanCloudDomainTool(uc.UnholdArvanCloudDomain),
	}
}

// PingTool is a trivial built-in tool with no use case or provider behind it.
// It proves the full MCP transport round-trip end-to-end — a client can
// connect, list tools, and call one successfully — before any real business
// tool exists (AGENTS.md 5).
func PingTool() Tool {
	return Tool{
		Name:        "ping",
		Description: "Health-check tool with no side effects. Returns \"pong\" to confirm the MCP server is reachable.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(_ context.Context, _ json.RawMessage) (any, error) {
			return map[string]any{"message": "pong"}, nil
		},
	}
}

type createServerArgs struct {
	credentialArgs
	Name     string   `json:"name"`
	Region   string   `json:"region"`
	Image    string   `json:"image"`
	PlanID   string   `json:"plan_id"`
	CPUCores int      `json:"cpu_cores"`
	RAMMB    int      `json:"ram_mb"`
	DiskGB   int      `json:"disk_gb"`
	SSHKeys  []string `json:"ssh_keys"`
}

func createServerTool(uc *app.ProvisionServer) Tool {
	props := credentialProperties()
	props["name"] = map[string]any{
		"type":        "string",
		"description": "Hostname for the new server, e.g. \"web-01\". Must be unique within the account; it is also how a retry recognizes an already-created server.",
	}
	props["region"] = map[string]any{
		"type":        "string",
		"description": "Provider datacenter region, e.g. \"tehran\". Omit to use the provider default.",
	}
	props["image"] = map[string]any{
		"type":        "string",
		"description": "Operating system image, e.g. \"ubuntu-24.04\".",
	}
	props["plan_id"] = map[string]any{
		"type":        "string",
		"description": "Provider plan/flavor identifier. Supply this when the user names a plan; otherwise give cpu_cores, ram_mb and disk_gb instead.",
	}
	props["cpu_cores"] = map[string]any{
		"type":        "integer",
		"description": "Number of virtual CPU cores, e.g. 2.",
		"minimum":     1,
	}
	props["ram_mb"] = map[string]any{
		"type":        "integer",
		"description": "RAM in megabytes, e.g. 2048 for 2GB.",
		"minimum":     512,
	}
	props["disk_gb"] = map[string]any{
		"type":        "integer",
		"description": "Disk size in gigabytes, e.g. 40.",
		"minimum":     10,
	}
	props["ssh_keys"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "IDs or fingerprints of SSH keys already registered with the provider, to be installed on the new server.",
	}

	return Tool{
		Name: "create_server",
		Description: "Provision a new server (VPS) at Parspack. This is a long operation: it returns immediately with " +
			"an operation_id and status \"pending\". Poll get_operation_status with that id to learn when the server is ready.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "name"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args createServerArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			out, err := uc.Execute(ctx, app.ProvisionServerInput{
				Credentials: args.domain(),
				Spec: domain.ServerSpec{
					Name:     args.Name,
					Region:   args.Region,
					Image:    args.Image,
					PlanID:   args.PlanID,
					CPUCores: args.CPUCores,
					RAMMB:    args.RAMMB,
					DiskGB:   args.DiskGB,
					SSHKeys:  args.SSHKeys,
				},
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"operation_id": out.OperationID,
				"status":       out.Status.String(),
				"note":         "Server provisioning runs in the background. Call get_operation_status with this operation_id to check progress.",
			}, nil
		},
	}
}

// serverToMap renders a domain.Server the way every server-returning tool
// reports it back to the caller.
func serverToMap(srv domain.Server) map[string]any {
	return map[string]any{
		"id":           srv.ID,
		"name":         srv.Name,
		"status":       srv.Status.String(),
		"region":       srv.Region,
		"image":        srv.Image,
		"plan_id":      srv.PlanID,
		"cpu_cores":    srv.CPUCores,
		"ram_mb":       srv.RAMMB,
		"disk_gb":      srv.DiskGB,
		"ipv4":         srv.IPv4,
		"ipv4_private": srv.IPv4Private,
		"ipv6":         srv.IPv6,
		"vpc_uuid":     srv.VPCUUID,
		"created_at":   srv.CreatedAt,
	}
}

func listServersTool(uc *app.ListServers) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_servers",
		Description: "List every server (VPS) at Parspack visible to the given credentials. This is a fast operation: " +
			"the list is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args credentialArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			servers, err := uc.Execute(ctx, app.ListServersInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(servers))
			for i, srv := range servers {
				out[i] = serverToMap(srv)
			}
			return map[string]any{"servers": out}, nil
		},
	}
}

type serverIDArgs struct {
	credentialArgs
	ServerID string `json:"server_id"`
}

func getServerTool(uc *app.GetServer) Tool {
	props := credentialProperties()
	props["server_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID of the server to look up, as returned by create_server or list_servers.",
	}

	return Tool{
		Name: "get_server",
		Description: "Get the current state of one server (VPS) at Parspack by its provider ID. This is a fast " +
			"operation: the result is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "server_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args serverIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			srv, err := uc.Execute(ctx, app.GetServerInput{
				Credentials: args.domain(),
				ServerID:    args.ServerID,
			})
			if err != nil {
				return nil, err
			}
			return serverToMap(*srv), nil
		},
	}
}

func deleteServerTool(uc *app.DeleteServer) Tool {
	props := credentialProperties()
	props["server_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID of the server to delete, as returned by create_server or list_servers.",
	}

	return Tool{
		Name: "delete_server",
		Description: "Permanently delete a server (VPS) at Parspack by its provider ID. This is a fast operation " +
			"and cannot be undone. Deleting a server that no longer exists is treated as already done rather than " +
			"an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "server_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args serverIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteServerInput{
				Credentials: args.domain(),
				ServerID:    args.ServerID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "server_id": args.ServerID}, nil
		},
	}
}

type registerSSHKeyArgs struct {
	credentialArgs
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
}

func registerSSHKeyTool(uc *app.RegisterSSHKey) Tool {
	props := credentialProperties()
	props["name"] = map[string]any{
		"type":        "string",
		"description": "Human-readable label for the key, e.g. \"laptop\" or \"ci-runner\". Must be unique within the account.",
	}
	props["public_key"] = map[string]any{
		"type":        "string",
		"description": "The public key contents, e.g. \"ssh-ed25519 AAAAC3... user@host\". Sent to the provider as-is.",
	}

	return Tool{
		Name: "register_ssh_key",
		Description: "Register an SSH public key with the provider so it can be installed on new servers via " +
			"create_server's ssh_keys parameter. This is a fast operation: the created key (with its provider id " +
			"and fingerprint) is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "name", "public_key"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args registerSSHKeyArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			key, err := uc.Execute(ctx, app.RegisterSSHKeyInput{
				Credentials: args.domain(),
				Key:         domain.SSHKey{Name: args.Name, PublicKey: args.PublicKey},
			})
			if err != nil {
				return nil, err
			}
			return sshKeyToMap(*key), nil
		},
	}
}

func listSSHKeysTool(uc *app.ListSSHKeys) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_ssh_keys",
		Description: "List every SSH key registered with the provider for the given credentials. This is a fast " +
			"operation: the list is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args credentialArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			keys, err := uc.Execute(ctx, app.ListSSHKeysInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(keys))
			for i, key := range keys {
				out[i] = sshKeyToMap(key)
			}
			return map[string]any{"ssh_keys": out}, nil
		},
	}
}

type sshKeyIDArgs struct {
	credentialArgs
	KeyID string `json:"key_id"`
}

func deleteSSHKeyTool(uc *app.DeleteSSHKey) Tool {
	props := credentialProperties()
	props["key_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID (or fingerprint) of the key to delete, as returned by register_ssh_key or list_ssh_keys.",
	}

	return Tool{
		Name: "delete_ssh_key",
		Description: "Permanently delete a registered SSH key by its provider ID or fingerprint. This is a fast " +
			"operation and cannot be undone. Deleting a key that no longer exists is treated as already done rather " +
			"than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "key_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args sshKeyIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteSSHKeyInput{
				Credentials: args.domain(),
				KeyID:       args.KeyID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "key_id": args.KeyID}, nil
		},
	}
}

// sshKeyToMap renders a domain.SSHKey the way every key-returning tool reports
// it back to the caller.
func sshKeyToMap(key domain.SSHKey) map[string]any {
	return map[string]any{
		"id":          key.ID,
		"name":        key.Name,
		"fingerprint": key.Fingerprint,
		"public_key":  key.PublicKey,
	}
}

type firewallRuleArgs struct {
	Protocol  string   `json:"protocol"`
	PortRange string   `json:"port_range"`
	Addresses []string `json:"addresses"`
}

type firewallArgs struct {
	credentialArgs
	Name          string             `json:"name"`
	ServerIDs     []string           `json:"server_ids"`
	InboundRules  []firewallRuleArgs `json:"inbound_rules"`
	OutboundRules []firewallRuleArgs `json:"outbound_rules"`
}

type firewallIDArgs struct {
	credentialArgs
	FirewallID string `json:"firewall_id"`
}

type updateFirewallArgs struct {
	firewallArgs
	FirewallID string `json:"firewall_id"`
}

func (a firewallArgs) firewall() domain.Firewall {
	fw := domain.Firewall{
		Name:      a.Name,
		ServerIDs: a.ServerIDs,
	}
	for _, r := range a.InboundRules {
		fw.InboundRules = append(fw.InboundRules, firewallRuleArgsToDomain(r))
	}
	for _, r := range a.OutboundRules {
		fw.OutboundRules = append(fw.OutboundRules, firewallRuleArgsToDomain(r))
	}
	return fw
}

func firewallRuleArgsToDomain(r firewallRuleArgs) domain.FirewallRule {
	return domain.FirewallRule{Protocol: r.Protocol, PortRange: r.PortRange, Addresses: r.Addresses}
}

// firewallRuleProperties is the JSON Schema for one inbound/outbound rule
// block, shared by create_firewall and update_firewall.
func firewallRuleProperties() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"protocol": map[string]any{
				"type":        "string",
				"enum":        []string{"tcp", "udp", "icmp"},
				"description": "The type of traffic the rule allows: \"tcp\", \"udp\", or \"icmp\".",
			},
			"port_range": map[string]any{
				"type":        "string",
				"description": "A single port, a range like \"8000-9000\", or \"1-65535\" for all ports. Required for tcp and udp rules, ignored for icmp.",
			},
			"addresses": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Source addresses (CIDRs or single IPs) for an inbound rule, or destination addresses for an outbound rule. Omit to mean all addresses.",
			},
		},
		"required": []string{"protocol"},
	}
}

func createFirewallTool(uc *app.CreateFirewall) Tool {
	props := credentialProperties()
	props["name"] = map[string]any{
		"type":        "string",
		"description": "Human-readable firewall name, e.g. \"only-22-80-and-443\".",
	}
	props["server_ids"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "IDs of the servers (VMs) the firewall is applied to, as returned by create_server or list_servers. Omit to create the firewall without attaching any server yet.",
	}
	props["inbound_rules"] = map[string]any{
		"type":        "array",
		"items":       firewallRuleProperties(),
		"description": "Inbound rules: traffic allowed INTO the attached servers. Each rule lists its source addresses under \"addresses\".",
	}
	props["outbound_rules"] = map[string]any{
		"type":        "array",
		"items":       firewallRuleProperties(),
		"description": "Outbound rules: traffic allowed OUT of the attached servers. Each rule lists its destination addresses under \"addresses\".",
	}

	return Tool{
		Name: "create_firewall",
		Description: "Create a new rules-based network firewall at Parspack and optionally attach it to servers. This is a " +
			"fast operation: the created firewall is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "name"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args firewallArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			fw, err := uc.Execute(ctx, app.CreateFirewallInput{
				Credentials: args.domain(),
				Firewall:    args.firewall(),
			})
			if err != nil {
				return nil, err
			}
			return firewallToMap(*fw), nil
		},
	}
}

func getFirewallTool(uc *app.GetFirewall) Tool {
	props := credentialProperties()
	props["firewall_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID of the firewall to look up, as returned by create_firewall or list_firewalls.",
	}

	return Tool{
		Name: "get_firewall",
		Description: "Get the current state of one firewall at Parspack by its provider ID. This is a fast " +
			"operation: the result is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "firewall_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args firewallIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			fw, err := uc.Execute(ctx, app.GetFirewallInput{
				Credentials: args.domain(),
				FirewallID:  args.FirewallID,
			})
			if err != nil {
				return nil, err
			}
			return firewallToMap(*fw), nil
		},
	}
}

func listFirewallsTool(uc *app.ListFirewalls) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_firewalls",
		Description: "List every firewall at Parspack visible to the given credentials. This is a fast operation: " +
			"the list is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args credentialArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			firewalls, err := uc.Execute(ctx, app.ListFirewallsInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(firewalls))
			for i, fw := range firewalls {
				out[i] = firewallToMap(fw)
			}
			return map[string]any{"firewalls": out}, nil
		},
	}
}

func updateFirewallTool(uc *app.UpdateFirewall) Tool {
	props := credentialProperties()
	props["firewall_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID of the firewall to update, as returned by create_firewall or list_firewalls.",
	}
	props["name"] = map[string]any{
		"type":        "string",
		"description": "New human-readable firewall name, e.g. \"only-22-80-and-443\".",
	}
	props["server_ids"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": "IDs of the servers (VMs) the firewall should be applied to, replacing the previous set.",
	}
	props["inbound_rules"] = map[string]any{
		"type":        "array",
		"items":       firewallRuleProperties(),
		"description": "Inbound rules: traffic allowed INTO the attached servers. Replaces the previous inbound rules. Each rule lists its source addresses under \"addresses\".",
	}
	props["outbound_rules"] = map[string]any{
		"type":        "array",
		"items":       firewallRuleProperties(),
		"description": "Outbound rules: traffic allowed OUT of the attached servers. Replaces the previous outbound rules. Each rule lists its destination addresses under \"addresses\".",
	}

	return Tool{
		Name: "update_firewall",
		Description: "Replace the configuration of an existing firewall at Parspack (rules, server attachments, and " +
			"name). This is a fast operation: the updated firewall is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "firewall_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateFirewallArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			fw, err := uc.Execute(ctx, app.UpdateFirewallInput{
				Credentials: args.domain(),
				FirewallID:  args.FirewallID,
				Firewall:    args.firewall(),
			})
			if err != nil {
				return nil, err
			}
			return firewallToMap(*fw), nil
		},
	}
}

func deleteFirewallTool(uc *app.DeleteFirewall) Tool {
	props := credentialProperties()
	props["firewall_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID of the firewall to delete, as returned by create_firewall or list_firewalls.",
	}

	return Tool{
		Name: "delete_firewall",
		Description: "Permanently delete a firewall at Parspack by its provider ID. This is a fast operation and " +
			"cannot be undone. Deleting a firewall that no longer exists is treated as already done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "firewall_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args firewallIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteFirewallInput{
				Credentials: args.domain(),
				FirewallID:  args.FirewallID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "firewall_id": args.FirewallID}, nil
		},
	}
}

// firewallToMap renders a domain.Firewall the way every firewall-returning
// tool reports it back to the caller.
func firewallToMap(fw domain.Firewall) map[string]any {
	inbound := make([]map[string]any, len(fw.InboundRules))
	for i, r := range fw.InboundRules {
		inbound[i] = firewallRuleToMap(r)
	}
	outbound := make([]map[string]any, len(fw.OutboundRules))
	for i, r := range fw.OutboundRules {
		outbound[i] = firewallRuleToMap(r)
	}
	return map[string]any{
		"id":             fw.ID,
		"name":           fw.Name,
		"status":         fw.Status,
		"server_ids":     fw.ServerIDs,
		"inbound_rules":  inbound,
		"outbound_rules": outbound,
		"created_at":     fw.CreatedAt,
	}
}

func firewallRuleToMap(r domain.FirewallRule) map[string]any {
	return map[string]any{
		"protocol":   r.Protocol,
		"port_range": r.PortRange,
		"addresses":  r.Addresses,
	}
}

type getOperationStatusArgs struct {
	credentialArgs
	OperationID string `json:"operation_id"`
}

func getOperationStatusTool(uc *app.GetOperationStatus) Tool {
	props := credentialProperties()
	props["operation_id"] = map[string]any{
		"type":        "string",
		"description": "The operation_id returned by a long operation such as create_server.",
	}

	return Tool{
		Name: "get_operation_status",
		Description: "Check the progress of a long operation started earlier, such as create_server. Returns status " +
			"pending, running, succeeded or failed, plus the result once it succeeded. Passing the provider credentials " +
			"is recommended: if the server restarted while the operation was in flight, they let this call confirm with " +
			"the provider whether the resource was actually created.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"operation_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args getOperationStatusArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			op, err := uc.Execute(ctx, app.GetOperationStatusInput{
				OperationID: args.OperationID,
				Credentials: args.domain(),
			})
			if err != nil {
				return nil, err
			}

			out := map[string]any{
				"operation_id": op.ID,
				"status":       op.Status.String(),
				"updated_at":   op.UpdatedAt,
			}
			if len(op.Result) > 0 {
				out["result"] = json.RawMessage(op.Result)
			}
			if op.Error != "" {
				out["error"] = op.Error
			}
			return out, nil
		},
	}
}

// loadBalancerConfigArgs carries the mutable configuration shared by
// create_load_balancer and update_load_balancer.
type loadBalancerConfigArgs struct {
	Name              string               `json:"name"`
	Algorithm         string               `json:"algorithm"`
	Region            string               `json:"region"`
	ForwardingRules   []forwardingRuleArgs `json:"forwarding_rules"`
	HealthCheck       *healthCheckArgs     `json:"health_check"`
	ServerIDs         []string             `json:"server_ids"`
	RedirectHTTPToTLS bool                 `json:"redirect_http_to_tls"`
	VPCUUID           string               `json:"vpc_uuid"`
}

func (a loadBalancerConfigArgs) loadBalancer() domain.LoadBalancer {
	lb := domain.LoadBalancer{
		Name:              a.Name,
		Algorithm:         a.Algorithm,
		Region:            a.Region,
		ServerIDs:         a.ServerIDs,
		RedirectHTTPToTLS: a.RedirectHTTPToTLS,
		VPCUUID:           a.VPCUUID,
	}
	for _, r := range a.ForwardingRules {
		lb.ForwardingRules = append(lb.ForwardingRules, r.domain())
	}
	if a.HealthCheck != nil {
		lb.HealthCheck = a.HealthCheck.domain()
	}
	return lb
}

type forwardingRuleArgs struct {
	EntryProtocol  string `json:"entry_protocol"`
	EntryPort      int    `json:"entry_port"`
	TargetProtocol string `json:"target_protocol"`
	TargetPort     int    `json:"target_port"`
}

func (a forwardingRuleArgs) domain() domain.ForwardingRule {
	return domain.ForwardingRule{
		EntryProtocol:  a.EntryProtocol,
		EntryPort:      a.EntryPort,
		TargetProtocol: a.TargetProtocol,
		TargetPort:     a.TargetPort,
	}
}

type healthCheckArgs struct {
	Protocol               string `json:"protocol"`
	Port                   int    `json:"port"`
	Path                   string `json:"path"`
	CheckIntervalSeconds   int    `json:"check_interval_seconds"`
	ResponseTimeoutSeconds int    `json:"response_timeout_seconds"`
	UnhealthyThreshold     int    `json:"unhealthy_threshold"`
	HealthyThreshold       int    `json:"healthy_threshold"`
}

func (a healthCheckArgs) domain() *domain.LoadBalancerHealthCheck {
	return &domain.LoadBalancerHealthCheck{
		Protocol:               a.Protocol,
		Port:                   a.Port,
		Path:                   a.Path,
		CheckIntervalSeconds:   a.CheckIntervalSeconds,
		ResponseTimeoutSeconds: a.ResponseTimeoutSeconds,
		UnhealthyThreshold:     a.UnhealthyThreshold,
		HealthyThreshold:       a.HealthyThreshold,
	}
}

type createLoadBalancerArgs struct {
	credentialArgs
	loadBalancerConfigArgs
}

type updateLoadBalancerArgs struct {
	credentialArgs
	LoadBalancerID string `json:"load_balancer_id"`
	loadBalancerConfigArgs
}

type loadBalancerIDArgs struct {
	credentialArgs
	LoadBalancerID string `json:"load_balancer_id"`
}

// loadBalancerConfigProperties is the JSON Schema for the mutable
// configuration shared by create_load_balancer and update_load_balancer.
func loadBalancerConfigProperties() map[string]any {
	return map[string]any{
		"name": map[string]any{
			"type":        "string",
			"description": "Human-readable load balancer name, e.g. \"api-lb\". Must be unique within the account; it is also how a retry recognizes an already-created balancer.",
		},
		"algorithm": map[string]any{
			"type":        "string",
			"enum":        []string{"round_robin", "least_connections"},
			"description": "Balancing algorithm. Omit to use the provider default (round_robin).",
		},
		"region": map[string]any{
			"type":        "string",
			"description": "Provider datacenter region, e.g. \"tehran\".",
		},
		"forwarding_rules": map[string]any{
			"type":        "array",
			"items":       forwardingRuleProperties(),
			"description": "Rules mapping the balancer's public protocol/port to the backend servers' protocol/port. At least one is required.",
		},
		"health_check": map[string]any{
			"type":        "object",
			"properties":  healthCheckProperties(),
			"description": "Health check that probes the backend servers. Omit to use the provider default.",
		},
		"server_ids": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"description": "IDs of the backend servers (VMs) traffic is balanced across, as returned by create_server or list_servers.",
		},
		"redirect_http_to_tls": map[string]any{
			"type":        "boolean",
			"description": "Redirect HTTP traffic to HTTPS (TLS). Defaults to false.",
		},
		"vpc_uuid": map[string]any{
			"type":        "string",
			"description": "ID of the VPC the load balancer is placed into. Omit for the default networking.",
		},
	}
}

func forwardingRuleProperties() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"entry_protocol": map[string]any{
				"type":        "string",
				"enum":        []string{"http", "https", "http2", "http3", "tcp", "udp"},
				"description": "Protocol the load balancer listens on, e.g. \"http\".",
			},
			"entry_port": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"maximum":     65535,
				"description": "Public-facing port the load balancer listens on, e.g. 80.",
			},
			"target_protocol": map[string]any{
				"type":        "string",
				"enum":        []string{"http", "https", "http2", "http3", "tcp", "udp"},
				"description": "Protocol the backend servers receive on, e.g. \"http\".",
			},
			"target_port": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"maximum":     65535,
				"description": "Port on the backend servers the traffic is forwarded to, e.g. 8080.",
			},
		},
		"required": []string{"entry_protocol", "entry_port", "target_protocol", "target_port"},
	}
}

func healthCheckProperties() map[string]any {
	return map[string]any{
		"protocol": map[string]any{
			"type":        "string",
			"enum":        []string{"http", "https", "tcp"},
			"description": "Health check protocol, e.g. \"http\".",
		},
		"port": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"maximum":     65535,
			"description": "Port the health check probes on each backend server, e.g. 80.",
		},
		"path": map[string]any{
			"type":        "string",
			"description": "Path probed for http/https checks, e.g. \"/health\". Required for http and https protocols.",
		},
		"check_interval_seconds": map[string]any{
			"type":        "integer",
			"minimum":     3,
			"maximum":     300,
			"description": "Seconds between checks, e.g. 10.",
		},
		"response_timeout_seconds": map[string]any{
			"type":        "integer",
			"minimum":     3,
			"maximum":     300,
			"description": "Seconds before a check is considered failed, e.g. 5.",
		},
		"unhealthy_threshold": map[string]any{
			"type":        "integer",
			"minimum":     2,
			"maximum":     10,
			"description": "Failed checks before a backend server is marked unhealthy, e.g. 3.",
		},
		"healthy_threshold": map[string]any{
			"type":        "integer",
			"minimum":     2,
			"maximum":     10,
			"description": "Successful checks before a backend server is marked healthy again, e.g. 5.",
		},
	}
}

func createLoadBalancerTool(uc *app.ProvisionLoadBalancer) Tool {
	props := credentialProperties()
	for k, v := range loadBalancerConfigProperties() {
		props[k] = v
	}

	return Tool{
		Name: "create_load_balancer",
		Description: "Provision a new cloud-server-level load balancer at Parspack and attach backend servers to it. " +
			"This is a long operation: it returns immediately with an operation_id and status \"pending\". Poll " +
			"get_operation_status with that id to learn when the balancer is active.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "name", "forwarding_rules"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args createLoadBalancerArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			out, err := uc.Execute(ctx, app.ProvisionLoadBalancerInput{
				Credentials:  args.domain(),
				LoadBalancer: args.loadBalancer(),
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"operation_id": out.OperationID,
				"status":       out.Status.String(),
				"note":         "Load balancer provisioning runs in the background. Call get_operation_status with this operation_id to check progress.",
			}, nil
		},
	}
}

func getLoadBalancerTool(uc *app.GetLoadBalancer) Tool {
	props := credentialProperties()
	props["load_balancer_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID of the load balancer to look up, as returned by create_load_balancer or list_load_balancers.",
	}

	return Tool{
		Name: "get_load_balancer",
		Description: "Get the current state of one load balancer at Parspack by its provider ID. This is a fast " +
			"operation: the result is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "load_balancer_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args loadBalancerIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			lb, err := uc.Execute(ctx, app.GetLoadBalancerInput{
				Credentials:    args.domain(),
				LoadBalancerID: args.LoadBalancerID,
			})
			if err != nil {
				return nil, err
			}
			return loadBalancerToMap(*lb), nil
		},
	}
}

func listLoadBalancersTool(uc *app.ListLoadBalancers) Tool {
	props := credentialProperties()

	return Tool{
		Name: "list_load_balancers",
		Description: "List every cloud-server-level load balancer at Parspack visible to the given credentials. " +
			"This is a fast operation: the list is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args credentialArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			balancers, err := uc.Execute(ctx, app.ListLoadBalancersInput{Credentials: args.domain()})
			if err != nil {
				return nil, err
			}

			out := make([]map[string]any, len(balancers))
			for i, lb := range balancers {
				out[i] = loadBalancerToMap(lb)
			}
			return map[string]any{"load_balancers": out}, nil
		},
	}
}

func updateLoadBalancerTool(uc *app.UpdateLoadBalancer) Tool {
	props := credentialProperties()
	props["load_balancer_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID of the load balancer to reconfigure, as returned by create_load_balancer or list_load_balancers.",
	}
	for k, v := range loadBalancerConfigProperties() {
		props[k] = v
	}

	return Tool{
		Name: "update_load_balancer",
		Description: "Replace the configuration of an existing cloud-server-level load balancer at Parspack by its " +
			"provider ID. This is a fast operation: the updated balancer is returned within this call.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "load_balancer_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args updateLoadBalancerArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			lb, err := uc.Execute(ctx, app.UpdateLoadBalancerInput{
				Credentials:    args.domain(),
				LoadBalancerID: args.LoadBalancerID,
				LoadBalancer:   args.loadBalancer(),
			})
			if err != nil {
				return nil, err
			}
			return loadBalancerToMap(*lb), nil
		},
	}
}

func deleteLoadBalancerTool(uc *app.DeleteLoadBalancer) Tool {
	props := credentialProperties()
	props["load_balancer_id"] = map[string]any{
		"type":        "string",
		"description": "The provider ID of the load balancer to delete, as returned by create_load_balancer or list_load_balancers.",
	}

	return Tool{
		Name: "delete_load_balancer",
		Description: "Permanently delete a cloud-server-level load balancer at Parspack by its provider ID. This is a " +
			"fast operation and cannot be undone. Deleting a balancer that no longer exists is treated as already " +
			"done rather than an error.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"api_key", "load_balancer_id"},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args loadBalancerIDArgs
			if err := decodeArgs(raw, &args); err != nil {
				return nil, err
			}

			if err := uc.Execute(ctx, app.DeleteLoadBalancerInput{
				Credentials:    args.domain(),
				LoadBalancerID: args.LoadBalancerID,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "load_balancer_id": args.LoadBalancerID}, nil
		},
	}
}

// loadBalancerToMap renders a domain.LoadBalancer the way every
// load-balancer-returning tool reports it back to the caller.
func loadBalancerToMap(lb domain.LoadBalancer) map[string]any {
	var hc map[string]any
	if lb.HealthCheck != nil {
		hc = map[string]any{
			"protocol":                 lb.HealthCheck.Protocol,
			"port":                     lb.HealthCheck.Port,
			"path":                     lb.HealthCheck.Path,
			"check_interval_seconds":   lb.HealthCheck.CheckIntervalSeconds,
			"response_timeout_seconds": lb.HealthCheck.ResponseTimeoutSeconds,
			"unhealthy_threshold":      lb.HealthCheck.UnhealthyThreshold,
			"healthy_threshold":        lb.HealthCheck.HealthyThreshold,
		}
	}

	rules := make([]map[string]any, len(lb.ForwardingRules))
	for i, r := range lb.ForwardingRules {
		rules[i] = map[string]any{
			"entry_protocol":  r.EntryProtocol,
			"entry_port":      r.EntryPort,
			"target_protocol": r.TargetProtocol,
			"target_port":     r.TargetPort,
		}
	}

	return map[string]any{
		"id":                   lb.ID,
		"name":                 lb.Name,
		"algorithm":            lb.Algorithm,
		"region":               lb.Region,
		"ip":                   lb.IP,
		"status":               lb.Status,
		"forwarding_rules":     rules,
		"health_check":         hc,
		"server_ids":           lb.ServerIDs,
		"redirect_http_to_tls": lb.RedirectHTTPToTLS,
		"vpc_uuid":             lb.VPCUUID,
		"created_at":           lb.CreatedAt,
	}
}

// decodeArgs rejects malformed tool arguments at the boundary, so use cases
// can trust what they receive.
func decodeArgs(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		return fmt.Errorf("tool arguments are required: %w", domain.ErrInvalidInput)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("decoding tool arguments: %w: %v", domain.ErrInvalidInput, err)
	}
	return nil
}
