---
name: parspack-infra
description: "How to use the do0ps MCP server to manage Parspack infrastructure on the user's behalf: create/inspect/delete cloud servers (VPS), register SSH keys, add DNS records, and order SSL certificates. Use whenever the user asks in natural language to create, check on, change, or delete servers, to add SSH access or DNS records, or to obtain an SSL certificate — and whenever the tool set above is available but the user has not said exactly which tool to call."
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
| `register_ssh_key` | Registers an SSH public key | fast |
| `list_ssh_keys` | Lists registered SSH keys | fast |
| `delete_ssh_key` | Deletes a registered SSH key | fast |
| `create_dns_record` | Adds a DNS record to a hosted domain | fast |
| `get_operation_status` | Checks progress of a long operation | fast |
| `list_ssl_products` | Lists orderable SSL products + prices | fast |
| `create_ssl_order` | Places an SSL certificate order | fast |
| `process_ssl_order` | Submits CSR + contact, gets verification challenges | fast |
| `get_ssl_challenge` | Re-shows verification challenges | fast |
| `reload_ssl_challenge` | Switches verification method | fast |
| `verify_ssl_challenge` | Marks a completed challenge as verified | fast |
| `get_ssl_certificate` | Downloads the issued certificate | fast |
| `reissue_ssl_certificate` | Reissues a certificate with a new CSR | fast |

## Rules that apply to every tool call

1. **Credentials.** Every provider tool requires `api_key` (and `secret_key`, empty for
   Parspack, which uses a single key). Ask the user for their Parspack API key once, then
   pass it on **every** call. Never store it, never echo it back at the user in full.
2. **Look things up, don't guess slugs.** The tools that return ids/slugs exist precisely
   so you do not have to invent them. Before calling a tool that takes an id or slug,
   use the matching list tool (below). Only when no list tool exists may you ask the user.
3. **Confirm destructive actions.** `delete_server` and `delete_ssh_key` are permanent.
   Confirm with the user before calling them.
4. **Never block the user.** Long operations return immediately with an
   `operation_id`. Tell the user it's running, and poll in the background (see below).

## Long operations: the `operation_id` + polling pattern

Only `create_server` is long. Calling it returns:

```json
{ "operation_id": "...", "status": "pending", "note": "..." }
```

Do **not** tell the user the server is ready. Say it is being created and that you will
report back. Then poll `get_operation_status` with that `operation_id` (pass the
`api_key` too) until it reaches a terminal state:

- `pending` or `running` → still working, wait and check again (a few minutes is normal).
- `succeeded` → the `result` field contains the created server; report the name, IPv4
  address and status to the user.
- `failed` → read `error` and explain it in plain language.

The user can also ask "is my server ready?" at any time — that is the same poll.

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

## DNS records

`create_dns_record` adds a record to a domain Parspack already hosts. Params:

- **`zone`** — the bare domain, e.g. `example.com`.
- **`name`** — the subdomain part, e.g. `api` for `api.example.com`, or `@` for the bare
  domain. Explain this to the user in those terms.
- **`type`** — `A` (IPv4 address), `AAAA` (IPv6), `CNAME` (alias to another hostname),
  `TXT` (free text), `MX`/`NS`/`SRV`. Choose based on what the user is pointing at.
- **`value`** — the record's target: an IP for `A`, a hostname for `CNAME`, text for
  `TXT`.
- **`ttl`** — seconds (e.g. 3600 = 1 hour). Omit unless the user asks.
- **`priority`** — only for `MX`/`SRV`.

There is no list-zone tool; if the user asks about a domain that isn't theirs, say it
must be hosted at Parspack first.

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
   - `DNS_TXT` → add the TXT record with `create_dns_record` and tell the user DNS may
     take a while to propagate.
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