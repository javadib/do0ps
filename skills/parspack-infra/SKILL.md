---
name: parspack-infra
description: "How to use the do0ps MCP server to manage Parspack infrastructure on the user's behalf: cloud servers (VPS) and their snapshots, SSH keys, firewalls, load balancers, reserved IPs, private networks (VPCs), CDN zones with their DNS records and the CDN edge configuration behind them (caching, edge firewall and WAF, rate limiting, rule engines, logs), and SSL certificates. Use whenever the user asks in natural language to create, check on, change, or delete any of those — and whenever this tool set is available but the user has not said exactly which tool to call."
license: MIT
---

# Parspack infrastructure via the do0ps MCP server

You are talking to a non-technical user. The user's provider (Parspack) account is
managed through a set of MCP tools. Your job is to translate the user's plain-language
requests into tool calls, and to translate tool responses back into plain language.
Never dump raw tool output at the user; summarize it.

The tools you have available are:

| Tool | What it does | Speed |
| --- | --- | --- |
| `create_server` | Provisions a new VPS | **long** (minutes) |
| `list_servers` | Lists all servers in the account | fast |
| `get_server` | Shows one server by its id | fast |
| `delete_server` | Permanently deletes a server | fast |
| `create_snapshot` | Takes a point-in-time snapshot of a server's disk | **long** |
| `list_snapshots` | Lists VM snapshots | fast |
| `delete_snapshot` | Permanently deletes a snapshot | fast |
| `restore_vm` | Wipes a server's disk and restores a snapshot onto it | **long** |
| `register_ssh_key` | Registers an SSH public key | fast |
| `list_ssh_keys` | Lists registered SSH keys | fast |
| `delete_ssh_key` | Deletes a registered SSH key | fast |
| `create_firewall` | Creates a network firewall, optionally attached to servers | fast |
| `get_firewall` | Shows one firewall by its id | fast |
| `list_firewalls` | Lists firewalls | fast |
| `update_firewall` | Replaces a firewall's rules, name and attachments | fast |
| `delete_firewall` | Permanently deletes a firewall | fast |
| `create_load_balancer` | Provisions a load balancer over backend servers | **long** |
| `get_load_balancer` | Shows one load balancer by its id | fast |
| `list_load_balancers` | Lists load balancers | fast |
| `update_load_balancer` | Replaces a load balancer's configuration | fast |
| `delete_load_balancer` | Permanently deletes a load balancer | fast |
| `reserve_ip` | Reserves a static public IPv4 address | fast |
| `assign_ip_to_server` | Attaches a reserved IP to a server | fast |
| `unassign_ip` | Detaches a reserved IP from its server | fast |
| `release_ip` | Releases a reserved IP back to the pool | fast |
| `create_vpc` | Creates an isolated private network | fast |
| `get_vpc` | Shows one VPC by its id | fast |
| `list_vpcs` | Lists VPCs | fast |
| `delete_vpc` | Permanently deletes a VPC | fast |
| `list_cdn_plans` | Lists CDN plans and their pricing | fast |
| `create_cdn_zone` | Onboards a domain onto the CDN (creates a zone) | fast |
| `get_cdn_zone` | Shows one CDN zone by its uuid | fast |
| `list_cdn_zones` | Lists CDN zones | fast |
| `delete_cdn_zone` | Removes a domain from the CDN | fast |
| `get_nameserver_records` | Shows the nameservers the registrar must point at | fast |
| `list_dns_records` | Lists a zone's DNS records | fast |
| `create_dns_record` | Adds a DNS record to a CDN zone | fast |
| `update_dns_record` | Changes an existing record's ttl, proxy mode or value | fast |
| `delete_dns_record` | Deletes a record, or one value of it | fast |
| `get_operation_status` | Checks progress of a long operation | fast |
| `list_ssl_products` | Lists orderable SSL products + prices | fast |
| `create_ssl_order` | Places an SSL certificate order | fast |
| `process_ssl_order` | Submits CSR + contact, gets verification challenges | fast |
| `get_ssl_challenge` | Re-shows verification challenges | fast |
| `reload_ssl_challenge` | Switches verification method | fast |
| `verify_ssl_challenge` | Marks a completed challenge as verified | fast |
| `get_ssl_certificate` | Downloads the issued certificate | fast |
| `reissue_ssl_certificate` | Reissues a certificate with a new CSR | fast |

Beyond zone and DNS management, a further set of tools configures how the CDN edge itself
behaves. Every one of them is **fast** and takes the zone's `zone_uuid`, so they are
grouped here by family rather than listed one by one — see "CDN edge configuration" below
for when to reach for each.

| Family | Tools | What it covers |
| --- | --- | --- |
| Rule engines | `list`/`create`/`get`/`update`/`delete`/`toggle_cdn_origin_rule`, the same for `_page_rule` and `_transform_rule` | Per-URL overrides: where a request is fetched from, how a page is treated, how a request/response is rewritten |
| ModSec WAF | `get`/`update_cdn_modsec_status`, plus `list`/`create`/`get`/`update`/`delete` for `_cdn_modsec_rule` and `_cdn_modsec_data` | Web application firewall: on/off, rule sets, and the data lists rules match against |
| Edge load balancing | `list`/`create`/`get`/`update`/`delete_cdn_load_balance` and the same for `_cdn_load_balance_server` | Balancing across origins **at the CDN edge** — not the same thing as `create_load_balancer` |
| Zone settings | `get`/`update_cdn_antivirus_status`, `get`/`update_cdn_dnssec_status`, `get_cdn_optimization_status`, `update_cdn_optimization`, `update_cdn_developer_mode`, `update_cdn_maintenance_mode`, `update_cdn_query_string_setting`, `update_cdn_origin_offline` | Per-zone toggles |
| Edge firewall | `list`/`create`/`get`/`update`/`delete_cdn_access_rule`, `get`/`update_cdn_ip_reputation`, `get`/`update_cdn_ddos_actions` | Who may reach the site — again distinct from the VM-network `*_firewall` tools |
| Logs & analytics | `get_cdn_access_log`, `get_cdn_security_log`, `get_cdn_error_log`, `get_cdn_waf_log`, `get_cdn_top_visitors`, `get_cdn_monthly_traffic_usage`, `get`/`update_cdn_upstream_errors` | What the edge has been serving and blocking |
| Network settings | `get`/`update_cdn_https_convertor`, `get`/`update_cdn_edge_to_upstream_connection`, `get`/`update_cdn_web_socket`, `get`/`update_cdn_www_redirection` | How the edge talks to visitors and to the origin |
| Cache | `update_cdn_cache_ttl`, `update_cdn_cache_rule`, `update_cdn_cache_user_agent`, `get_cdn_cache_settings`, `list_cdn_cache_entries`, `get_cdn_cache_entry`, `purge_cdn_cache` | Caching behavior, and clearing what is cached |
| Rate limiting | `list`/`create`/`get`/`update`/`delete_cdn_rate_limit_rule`, `update_cdn_rate_limit_rule_priority` | Throttling abusive traffic |
| Bulklists | `list`/`create`/`get`/`update`/`delete_cdn_bulklist`, `list_cdn_firewall_countries` | Reusable IP/country lists that firewall rules refer to |
| Zone SSL | `get`/`update_cdn_min_tls_version`, `list_cdn_certificates`, `get`/`update_cdn_hsts` | HTTPS settings for the zone (separate from ordering a certificate) |

In that table a slash group such as `list`/`create`/`get`/`update`/`delete_cdn_access_rule`
means one tool per verb over the same resource. The **`list` form is plural** —
`list_cdn_access_rules`, `list_cdn_bulklists`, `list_cdn_load_balances`,
`list_cdn_load_balance_servers`, `list_cdn_modsec_rules`, `list_cdn_origin_rules`,
`list_cdn_page_rules`, `list_cdn_rate_limit_rules`, `list_cdn_transform_rules` — while
every other verb is singular. `list_cdn_modsec_data` is the one exception, singular
because "data" already is.

There is also a `ping` tool, which does nothing but confirm the connection works. Never
offer it to the user as a feature; use it only if you need to check the server is alive.

## Rules that apply to every tool call

1. **Credentials.** Every provider tool requires `api_key` (and `secret_key`, empty for
   Parspack, which uses a single key). Ask the user for their Parspack API key once, then
   pass it on **every** call. Never store it, never echo it back at the user in full.
2. **Look things up, don't guess slugs.** The tools that return ids/slugs exist precisely
   so you do not have to invent them. Before calling a tool that takes an id or slug,
   use the matching list tool (below). Only when no list tool exists may you ask the user.
3. **Confirm destructive actions.** Every `delete_*` tool, plus `release_ip` and
   `restore_vm`, is permanent. Confirm with the user before calling them. `restore_vm`
   deserves an explicit warning: it wipes the server's current disk.
4. **Never block the user.** Long operations return immediately with an
   `operation_id`. Tell the user it's running, and poll in the background (see below).

## Long operations: the `operation_id` + polling pattern

Four tools are long: `create_server`, `create_snapshot`, `restore_vm` and
`create_load_balancer`. Each returns:

```json
{ "operation_id": "...", "status": "pending", "note": "..." }
```

Do **not** tell the user the server is ready. Say it is being created and that you will
report back. Then poll `get_operation_status` with that `operation_id` (pass the
`api_key` too) until it reaches a terminal state:

- `pending` or `running` → still working, wait and check again (a few minutes is normal).
- `succeeded` → the `result` field contains the finished resource; report the useful
  parts (a server's name, IPv4 address and status; a snapshot's id; a balancer's IP).
- `failed` → read `error` and explain it in plain language.

The user can also ask "is my server ready?" at any time — that is the same poll.

Pass the `api_key` on every `get_operation_status` call. If the server restarted while
the operation was in flight, those credentials are what let the call ask the provider
whether the resource actually got created, instead of leaving the user in the dark.

## Creating a server (the main workflow)

`create_server` is the only tool that requires an `api_key`, a `name`, and — in
practice — a `plan_id`. Map the conversation to parameters like this:

- **`name`** — a short hostname (`web-01`, `myapp`). If the user hasn't named it,
  suggest one. It must be unique in the account.
- **`plan_id`** — the size of the machine. **Required in practice:** Parspack only
  accepts a size slug, so you must supply one. If the user names a plan from their
  dashboard, pass that name. If the user describes specs in plain language ("2GB RAM,
  2 cores"), build the slug from the provider's convention
  `s-<cores>vcpu-<ram_in_gb>gb` — e.g. 1 core/1GB → `s-1vcpu-1gb`, 2 cores/2GB →
  `s-2vcpu-2gb`, 2 cores/4GB → `s-2vcpu-4gb`. If you are not confident, ask the user
  which plan they want rather than guessing a slug. (`cpu_cores`/`ram_mb`/`disk_gb`
  are accepted but cannot substitute for `plan_id`.)
- **`ram_mb`** — RAM in megabytes, for reference: 1GB → 1024, 2GB → 2048, 4GB → 4096,
  8GB → 8192.
- **`disk_gb`** — disk in gigabytes (e.g. 25, 40, 80).
- **`region`** — datacenter, e.g. `tehran`. If the user names a city, pass it in
  lower-case (`tehran`); otherwise omit it and let the provider default apply. There is
  no region list tool, so when the user is unsure, ask them where they want the server
  rather than inventing a region slug.
- **`image`** — operating system, e.g. `ubuntu-24.04`. If the user says "Ubuntu",
  "Debian", etc., pass the matching slug. If they did not specify, ask which OS they
  want; do not invent a slug. Omit only if the user truly doesn't care.
- **`ssh_keys`** — ids or fingerprints of **already-registered** keys to install.
  Call `list_ssh_keys` first to get real ids/fingerprints. If the user's key isn't
  registered yet, have them paste their public key and call `register_ssh_key` first.

Workflow to follow:

1. Collect the `api_key` (if not already known) and the server's name/specs.
2. If the user wants SSH access, `list_ssh_keys` (register first if needed) and pass the
   matching ids.
3. Call `create_server`, get the `operation_id`.
4. Tell the user it's provisioning and poll `get_operation_status` until it succeeds or
   fails, then summarize.

## Checking on existing servers

- "What servers do I have?" → `list_servers`.
- "What's the status / IP of server X?" → `get_server` with the `server_id` from
  `list_servers`, or if X is still provisioning, `get_operation_status` with the
  `operation_id`.
- "Delete server X" → confirm, then `delete_server` with the `server_id` from
  `list_servers`. Deleting an already-gone server is reported as done, not an error.

## SSH keys

- "Add my SSH key" → `register_ssh_key` with a `name` (label) and the `public_key`
  contents (`ssh-ed25519 AAAA... user@host`). The returned id/fingerprint is what you
  pass to `create_server`.
- "Which keys do I have?" → `list_ssh_keys`.
- "Remove key X" → confirm, then `delete_ssh_key` with the `key_id` from
  `list_ssh_keys`.

## CDN zones and DNS records

Parspack has no standalone DNS product: a domain's DNS records live inside its **CDN
zone**. So the first step for anything DNS-related is always the zone.

Onboarding a domain:

1. `list_cdn_plans` — show the user the plans and prices, and let them pick.
2. `create_cdn_zone` with `domain` (the bare domain, e.g. `example.com`), `plan` and
   `billing_cycle`. It returns the `zone_uuid` every DNS tool needs. Default MX and NS
   records are created for them automatically.
3. `get_nameserver_records` with the `zone_uuid` — tell the user to point their domain
   registrar at those nameservers. Nothing resolves until they do.

For a domain that is already onboarded, `list_cdn_zones` gives you the `zone_uuid`; never
ask the user for a uuid you can look up.

Then the record tools, all of which take `zone_uuid`:

- **`host`** — the subdomain part, e.g. `api` for `api.example.com`, or `@` for the bare
  domain. Explain it to the user in those terms.
- **`type`** — one of `A`, `CNAME`, `MX`, `TXT`, `NS`, `SRV`, `CAA`. Choose from what the
  user is pointing at.
- **`content`** — the value: an IPv4 address for `A`, a hostname for `CNAME`/`MX`/`NS`,
  arbitrary text for `TXT`.
- **`ttl`** — seconds, and **only a value from the provider's list** (1, 2, 5, 10, 30, 60,
  180, 300, 600, 900, 1800, 2700, 3600, 10800, 18000, 36000, 43200, 86400, 259200,
  604800, 864000, 1296000, 2592000). An arbitrary number is rejected; 3600 is a safe
  default when the user has no preference.
- **`proxy`** — whether the CDN sits in front of the record: `direct` (no CDN, resolves
  straight to the content) or one of the caching modes `cdn-no-caching`,
  `cdn-static-caching`, `cdn-smart-caching`, `cdn-always-caching`. Use `direct` unless the
  user asked for CDN caching — and always `direct` for a TXT record used to verify domain
  ownership.
- **`priority`** for `MX`/`SRV`, **`port`**/**`weight`** for `SRV`, **`flags`**/**`tag`**
  for `CAA`.

Records are grouped by host and type, and one group can hold several values (e.g. two NS
records for the apex):

- "Show me my DNS" → `list_dns_records`.
- "Point api.example.com at this IP" → `create_dns_record`. Adding a record with a host
  and type that already exist **appends** another value rather than replacing.
- "Change the IP / TTL / caching of that record" → `update_dns_record`. It can change ttl,
  proxy mode and value, but cannot add or remove values under that host and type.
- "Remove that record" → confirm, then `delete_dns_record` with `host` and `type`. Pass
  `content` to remove one specific value; omit it to remove every value under that host
  and type.

If the user asks about a domain that has no zone, say it must be onboarded onto the CDN
first, and offer to do it.

## CDN edge configuration

Everything in this section applies to a zone that already exists, and every tool takes its
`zone_uuid` — get it from `list_cdn_zones`, never ask the user for a uuid.

Two naming collisions matter, and getting them wrong changes the wrong system:

- `create_firewall` and friends protect a **server's network**. `create_cdn_access_rule`
  and friends protect a **website at the CDN edge**. "Block this IP from my site" is the
  CDN one; "close port 22 on my server" is the VM one.
- `create_load_balancer` provisions a **load balancer in front of servers**.
  `create_cdn_load_balance` balances **origins at the CDN edge**. When the user says
  "load balancer" without qualifying, ask which they mean before creating anything.

Map what the user says to the right family:

- *"My site is slow / cache it"* → cache settings: `get_cdn_cache_settings` first to see
  where things stand, then `update_cdn_cache_ttl` or `update_cdn_cache_rule`.
- *"I deployed but visitors see the old version"* → `purge_cdn_cache`. It clears the whole
  zone's cache, so say so before calling it.
- *"I'm working on the site, stop caching for now"* → `update_cdn_developer_mode`. Remind
  the user to turn it off afterwards, since it bypasses the cache entirely.
- *"Put up a maintenance page"* → `update_cdn_maintenance_mode`.
- *"Block this IP / this country"* → `create_cdn_access_rule`. For a list reused across
  rules, create a bulklist first (`create_cdn_bulklist`; `list_cdn_firewall_countries` has
  the country codes).
- *"I'm getting attacked"* → `update_cdn_ddos_actions` and `update_cdn_ip_reputation`, and
  `create_cdn_rate_limit_rule` to throttle a specific path.
- *"Turn on the WAF"* → `update_cdn_modsec_status`; individual rules via the
  `_cdn_modsec_rule` tools.
- *"Force HTTPS"* → `update_cdn_https_convertor`; `update_cdn_min_tls_version` and
  `update_cdn_hsts` tighten it further. Warn before enabling HSTS: browsers remember it,
  so a site that later loses HTTPS becomes unreachable for those visitors.
- *"Redirect www"* → `update_cdn_www_redirection`.
- *"Who is visiting / what is being blocked?"* → `get_cdn_top_visitors`,
  `get_cdn_access_log`, `get_cdn_security_log`, `get_cdn_waf_log`. Summarize; never paste
  a raw log at the user.
- *"How much traffic did I use?"* → `get_cdn_monthly_traffic_usage`.
- *"Serve this path from somewhere else / rewrite these URLs"* → the rule engines:
  `create_cdn_origin_rule` for where content is fetched from, `create_cdn_page_rule` for
  per-URL behavior, `create_cdn_transform_rule` for rewriting. List the existing rules
  first — order and priority matter, and a new rule can silently shadow one already there.

The same standing rules apply here: look ids up rather than guessing them, and confirm
before any `delete_*`, before `purge_cdn_cache`, and before a toggle that changes what
visitors see (maintenance mode, developer mode, HSTS).

## Firewalls

`create_firewall` takes a `name` and rule lists, and can attach servers straight away via
`server_ids` (use `list_servers` to get real ids). `update_firewall` **replaces** the whole
configuration — read the current one with `get_firewall` first and send it back with your
change applied, or you will silently drop rules the user still wants.

## Load balancers

`create_load_balancer` needs a `name` and `forwarding_rules`, and usually `server_ids` for
the backends. It is a long operation — poll `get_operation_status` until the balancer is
active, then report its address. As with firewalls, `update_load_balancer` replaces the
whole configuration, so read it first with `get_load_balancer`.

## Reserved IPs

A reserved IP exists on its own and is billed whether or not it is attached:

- "Give me a static IP" → `reserve_ip` with a `region`.
- "Put it on server X" → `assign_ip_to_server` with the address and the `server_id`.
- "Take it off" → `unassign_ip`. The address stays reserved and keeps being billed —
  say so, since users often expect detaching to stop the charge.
- "I don't need it any more" → confirm, then `release_ip`. That one does stop the billing
  and cannot be undone.

## Private networks (VPCs)

`create_vpc` takes a `name` and `region`, optionally a `description` and `ip_range`.
Servers join a VPC through `create_server`'s `vpc_uuid`, so create the network before the
servers that belong in it.

## Snapshots and restore

- "Back up my server" → `create_snapshot` with the `server_id` and a `name`. Long
  operation: poll until it succeeds. The resulting id can be used as the `image` of a new
  server.
- "What backups do I have?" → `list_snapshots`.
- "Roll server X back to that snapshot" → this is `restore_vm`, and it **wipes the
  server's current disk**. Say that plainly and get explicit confirmation before calling
  it. Long operation: poll until it finishes.
- "Delete that backup" → confirm, then `delete_snapshot`.

## SSL certificates

The SSL flow has explicit steps the user must complete in between:

1. `list_ssl_products` — pick a `product_slug` (e.g. `dv-ssl`) and show the price.
2. `create_ssl_order` with `product_slug`, `domain` (no `www`), `billing_cycle`
   (`annually`/`biennially`/`triennially`), and optionally `www` / `sans`. It returns an
   `order_id` and an **invoice that must be paid** before the order can proceed — tell
   the user to pay it, then continue when they say they have.
3. `process_ssl_order` with the `order_id`, a PEM `csr` for the same domain, and the
   user's contact details: `country` is an ISO 3166-1 alpha-2 code (`US`, `DE`, ...),
   `phone` is digits only with the country code (e.g. `12025551234`). OV/EV products
   additionally need jurisdiction/business fields; DV only needs the basic contact.
4. This returns verification challenges. Complete the chosen method:
   - `DNS_TXT` → add the TXT record with `create_dns_record` (the domain needs a CDN
     zone; `proxy` must be `direct`) and tell the user DNS may take a while to
     propagate.
   - `FILE` → give the user the file path + content to upload.
   - `ADMIN` → a verification email is sent to `email_prefix@domain`; ask the user to
     click the link.
5. Once the user says the challenge is done, call `verify_ssl_challenge` with the
   `order_id` and `method`. For DV the certificate is often ready immediately.
6. `get_ssl_certificate` with the `order_id` until `ready` is true, then deliver the
   certificate + CA bundle. If it is not ready, wait and check again.

`reload_ssl_challenge` switches method (regenerates tokens); `reissue_ssl_certificate`
reissues an already-issued certificate with a new CSR.

## Handing errors to the user

- A failed `create_server` or `get_operation_status` with `status: failed` → explain
  the `error` in plain words (e.g. name already taken, plan invalid).
- An invalid or unknown slug → say the option isn't available and ask how they'd like
  to proceed, offering the alternatives you have real values for.
- Never claim a resource was created before a fast tool returned it, or before a long
  operation reached `succeeded`.