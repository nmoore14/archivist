# Archivist Enterprise Refactor and Stack Migration Plan

## Purpose and target state

Move Archivist from its current local-first education MVP to a multi-tenant enterprise knowledge assistant. Replace the Go server, server-rendered HTMX interface, SQLite persistence, and local Ollama dependency with a TypeScript Nuxt 4 application, an ASP.NET Core/.NET 10 API, and Azure AI Foundry for model inference and embeddings.

The target architecture should keep the central product promise: answers are grounded in approved organization content and show verifiable citations. Unlike the current application, it must support enterprise identity, authorization, operational visibility, scalable indexing, and Azure-native secret and model management.

Recommended initial deployment shape:

```text
Browser
  -> Nuxt 4 (Vue 3 + TypeScript) web application
  -> ASP.NET Core 10 API
       -> Microsoft Entra ID / enterprise identity provider
       -> PostgreSQL + pgvector (transactional data and vector retrieval)
       -> Azure Blob Storage (original documents)
       -> background worker / queue (ingestion, extraction, embedding)
       -> Azure AI Foundry (chat completions and embeddings)
       -> Azure Key Vault, Application Insights, Azure Monitor
```

Use PostgreSQL with `pgvector` in the first production version when retrieval filters must closely match application authorization and tenancy rules. Reassess Azure AI Search later if hybrid search, semantic ranker, multi-format enrichment, or very large corpus scale outweigh the operational simplicity of keeping retrieval in PostgreSQL.

## Current-project assessment

The current repository is a Go 1.24 application with a single HTTP server, server-rendered Go templates, HTMX partial updates, vanilla JavaScript, SQLite, filesystem uploads, and a two-container Docker Compose stack. Ollama is a second container that supplies both chat and embedding calls. The application stores serialized embedding vectors in SQLite and ranks every workspace chunk in process using cosine similarity.

This design is clear and appropriate for an offline capstone MVP, but it does not provide a production queue, scalable vector search, SSO, tenant separation, enterprise roles, managed secrets, audit coverage, document lifecycle controls, or robust web/API security. The `chat_sessions`, `jobs`, and `audit_logs` tables are schema foundations only; the current UI and services do not yet implement their intended behavior.

## Feature inventory and disposition worksheet

Use this as the feature-selection list for the fork. `Keep`, `replace`, and `remove` are proposed defaults, not irreversible decisions.

| Area | Current feature | Current behavior | Proposed enterprise disposition |
| --- | --- | --- | --- |
| Product | Trusted-source RAG Q&A | Answers use retrieved workspace documents and return document/page references | **Keep and strengthen** |
| Product | Strict evidence boundary | Prompt instructs the model to answer only from retrieved material and reject unsupported answers | **Keep and make configurable per tenant/assistant** |
| Product | No web retrieval | URLs in user questions are declined | **Keep as default**; add governed connectors only if approved |
| Product | Markdown and math responses | Markdown rendered server-side; KaTeX is bundled | **Keep** with Vue markdown sanitization and KaTeX rendering |
| Product | Citation viewer | Citations open the original uploaded file in an iframe/new tab; PDF citations include pages | **Keep and improve** with page/deep-link preview and citation metadata |
| Product | Workspace-scoped Q&A | A question is answered only from one course workspace | **Keep, rename to knowledge space/collection**; permit governed cross-space search later |
| Product | Global search | Keyword search over accessible indexed chunks and the current user’s chat history | **Keep and improve** with hybrid search, filters, facets, and authorization enforcement |
| Product | Personal notes | One editable note per user/workspace; assistant replies can be appended | **Product decision:** keep as personal workspace, replace with enterprise notes, or remove |
| Product | Workspace dashboard | Lists recent workspaces and document/member counts | **Keep, redesign** as enterprise dashboard |
| Product | Ask landing page | Lists available workspaces | **Keep only if useful**; can become a knowledge-space picker |
| Content | Upload source files | Admin can batch upload `.txt`, `.md`, `.html`, `.htm`, and text-based `.pdf` | **Keep and expand** with policy, quotas, malware scan, metadata, versioning, and connectors |
| Content | Text extraction | Plain text/Markdown/HTML cleanup and page-based PDF text extraction | **Keep, replace implementation** with a resilient extraction pipeline; add OCR decision |
| Content | Scanned-PDF limitation | Image-only/scanned PDFs fail | **Decide:** add Azure Document Intelligence/OCR or retain an explicit unsupported-file policy |
| Content | Chunking profiles | Focused/balanced/broad profiles change chunk size and overlap | **Keep conceptually**, but replace hand-tuned UI with versioned retrieval profiles managed by admins |
| Content | Retrieval-count setting | Workspace admin selects 3, 4, or 6 passages per answer | **Keep as configuration**, likely an advanced admin policy rather than end-user setting |
| Content | Automatic stale reindex on startup | Rebuilds non-ready/outdated indexes after Ollama becomes healthy | **Replace** with explicit queue-driven ingestion/reindex jobs |
| Content | Manual workspace reindex | Admin can rebuild all documents in a workspace | **Keep** with job state, scope, retries, and cancellation |
| Content | Document progress UI | Queued/preparing/indexing/ready/error status and polling | **Keep and improve** with real job events, failure reasons, retries, and telemetry |
| Content | Delete document | Deletes database record, chunks, and local file | **Keep**, adding soft delete, retention policy, audit event, and Blob deletion workflow |
| Content | Original source serving | Serves uploaded source inline, including HTML in a sandbox | **Keep but replace** with short-lived authorized Blob access/proxy and content-security policy |
| Collaboration | Workspace visibility | Draft/published state gates student access | **Keep, rename** to lifecycle/access state (draft, active, archived) |
| Collaboration | Membership assignment | Admin assigns individual students to a workspace | **Replace** with Entra groups, teams, roles, and optional direct grants |
| Identity | First-run admin creation | First user initializes an admin account | **Replace** with tenant bootstrap and platform-admin process |
| Identity | Local email/password login | Bcrypt password hashes and 7-day HTTP-only session cookies | **Replace** with OIDC/OAuth2 via Microsoft Entra ID; support SCIM/JIT provisioning as needed |
| Identity | Two roles | `admin` and `student` | **Replace** with platform admin, tenant admin, knowledge-space admin/editor, contributor, viewer, auditor, and service roles as required |
| Identity | User directory | Admin lists and manually creates student accounts | **Replace** with identity-provider directory synchronization/provisioning |
| Chat | Persistent chat messages | Stores each user and assistant message per workspace/user | **Keep, redesign** around actual conversations and retention controls |
| Chat | Chat-session table | Table exists but is not used by UI/service behavior | **Implement or remove**; recommended: implement named conversations, titles, archive/delete, and retention |
| Chat | Source-only chat-history search | Users can search only their own messages | **Keep**, subject to tenant retention and discovery policy |
| Chat | Copy answer and keyboard shortcuts | Client-side copy, composer focus, navigation shortcuts | **Keep selectively** as UI polish |
| UI | Theme preference | Light/dark/system stored in browser local storage | **Keep** |
| UI | Responsive web interface | HTML/CSS template application, HTMX partial refreshes | **Replace** with Nuxt pages/components, accessible component library, typed API client, and composables |
| Operations | Docker Compose app + Ollama | App and model service run privately in Docker | **Replace** with API/web/worker deployment; **remove Ollama container and bundled model assets** |
| Operations | Offline Linux installer/Nginx deployment | Shell scripts install and verify a host-local isolated deployment | **Remove or replace** with Azure IaC and deployment runbooks; retain only if disconnected/on-prem remains a supported requirement |
| Operations | CLI maintenance | CLI can reindex or ask a workspace | **Replace** with secured admin APIs, worker commands, and optionally an internal .NET CLI |
| Operations | Bubble Tea TUI | Minimal terminal UI foundation | **Remove** unless a real operator CLI requirement exists |
| Operations | Health wait/retry | Server waits for local Ollama health before stale reindex | **Replace** with health probes, queue resiliency, Foundry retry/backoff, and circuit breakers |
| Evaluation | RAG vs. no-RAG test suite | Python evaluation cases, latency reporting, manual review template | **Keep and evolve** into automated regression/evaluation gates against Azure AI Foundry |
| Capstone artifacts | Notebooks, reports, presentation, recordings, experiment outputs | Academic documentation and model experiment evidence | **Archive outside the production application**; retain the evaluation assets that support quality gates |

## Target technical design

### Front end: Nuxt 4, Vue 3, TypeScript

- Build a Nuxt 4 TypeScript application using Vue 3 Composition API, SSR where it improves authenticated first paint, and client-side interaction for chat and ingestion status.
- Organize routes around tenant-neutral enterprise terms: dashboard, knowledge spaces, documents, conversations, search, administration, and audit.
- Use a generated TypeScript client from the ASP.NET OpenAPI document; do not duplicate request/response contracts manually.
- Use a small accessible component system and preserve existing useful UX: source drawer, citations, progress status, global search, responsive theme selection, Markdown/KaTeX output, and keyboard support.
- Authenticate through OIDC authorization-code flow with PKCE. Keep tokens out of local storage; favor an httpOnly BFF/session pattern or Nuxt server proxy appropriate to the selected identity topology.
- Stream chat responses using Server-Sent Events initially; add SignalR only when bidirectional real-time collaboration or richer job updates justify it.

### Backend: ASP.NET Core on .NET 10

Create a solution that separates transport, application rules, domain model, and infrastructure without premature microservices:

```text
src/
  Archivist.Api/             HTTP API, auth, OpenAPI, SSE endpoints
  Archivist.Application/     use cases, commands/queries, authorization policies
  Archivist.Domain/          entities, value objects, interfaces, events
  Archivist.Infrastructure/  EF Core, Blob storage, Foundry clients, queue, observability
  Archivist.Worker/          ingestion/reindex/background job host
  Archivist.Contracts/       API DTOs and generated OpenAPI configuration
web/                         Nuxt 4 application
tests/                       unit, integration, API contract, E2E, evaluation tests
infra/                       Bicep or Terraform and environment configuration
```

- Use ASP.NET Core controllers or minimal APIs consistently; publish versioned OpenAPI contracts.
- Use EF Core migrations with PostgreSQL. Do not carry forward the SQLite schema as an implementation constraint.
- Enforce authorization in the application/query layer and database predicates for every tenant, knowledge space, document, chunk, conversation, and audit query.
- Use FluentValidation (or equivalent) at API boundaries; use problem-details responses and correlation IDs.
- Use policy-based authorization, rate limiting, CORS allow lists, CSRF protection when cookies are used, secure headers, request-size/type limits, and anti-malware scanning hooks.

### Data and retrieval

Recommended first schema domains:

- `tenants`, tenant settings, data-region and retention policies
- users/service principals and external identity mappings
- roles, group mappings, memberships, and scoped grants
- knowledge spaces and their lifecycle/configuration
- document records, document versions, source metadata, and Blob references
- ingestion jobs, job attempts, checkpoints, errors, and reindex reasons
- chunks, embeddings, embedding model/version, source location/page, and ACL projection
- conversations, messages, citations, feedback, and retention/deletion markers
- prompt/retrieval configurations and immutable version records
- audit events, security events, and model-usage/cost events

Implement retrieval as a filtered query that scopes tenant and authorization before ranking. Store embeddings in `pgvector`; add keyword/full-text retrieval and reciprocal-rank fusion when evaluation demonstrates a gain. Never retrieve chunks merely by workspace ID without also applying the caller's effective access policy.

### Azure AI Foundry integration

- Replace `internal/models/ollama.go` with provider interfaces such as `IChatCompletionClient` and `IEmbeddingClient`; make Azure AI Foundry the initial implementation.
- Configure Foundry project/endpoint, deployment names, API version, timeout, retry budget, content-safety behavior, and model routing through environment-specific configuration and Key Vault—not user-editable database fields.
- Authenticate workloads with managed identity where supported; use Key Vault references for any remaining secrets.
- Record model deployment, embedding model, prompt version, retrieval profile, latency, token usage, request outcome, and cost attribution per tenant/knowledge space.
- Preserve the evidence-boundary prompt, but make it a versioned policy asset with automated prompt-injection, groundedness, citation, and refusal tests.
- Do not send document content to AI Foundry until the enterprise data-classification, regional-residency, and contractual review explicitly approves that flow.

### Ingestion and storage

- Upload originals directly to a private Azure Blob container through short-lived scoped upload access, then create an ingestion job through the API.
- A background worker validates file type/size, scans for malware, extracts text, applies OCR where enabled, chunks text, requests embeddings, writes retrieval records, and emits progress.
- Make ingestion idempotent through document-version hashes; support retries, dead-letter handling, cancellation, reindex by policy/model version, and downloadable error diagnostics for administrators.
- Store original documents separately from extracted text. Preserve page/section offsets so every citation can resolve to a source location.

## Migration approach

Do not attempt a line-by-line Go-to-C# rewrite. Build the enterprise platform beside the current application, prove feature parity for selected features, migrate only data that has business value, and retire the old runtime after acceptance.

### Phase 0 — product and governance decisions

1. Complete the disposition worksheet above and define the first enterprise persona set, tenant model, data classification, retention, residency, and compliance requirements.
2. Decide whether the fork is single-tenant initially or true multi-tenant from day one. Recommended: build tenant-aware data and authorization from day one, even for a single launch tenant.
3. Choose PostgreSQL/pgvector versus Azure AI Search after a corpus-size, connector, hybrid-search, and cost assessment.
4. Define the Azure AI Foundry model deployments, acceptable content flow, evaluation thresholds, fallback behavior, rate/token budgets, and human support path.
5. Freeze the current application as a read-only behavioral reference; retain its Go tests and evaluation cases as migration fixtures.

**Exit criteria:** signed product scope, data/security architecture decision record, approved Azure environment, and prioritized feature backlog.

### Phase 1 — foundation and delivery platform

1. Create the .NET 10 solution, Nuxt 4 TypeScript application, shared OpenAPI generation, local developer orchestration, and CI pipelines.
2. Provision Azure resources with Bicep or Terraform: networking, Container Apps/App Service/AKS choice, PostgreSQL, Blob Storage, queue, Key Vault, Application Insights, Log Analytics, and Azure AI Foundry connections/deployments.
3. Implement Entra ID authentication, tenant resolution, role/group mapping, policy-based authorization, health checks, structured logging, distributed tracing, and a baseline audit-event pipeline.
4. Add automated quality gates: C# unit/integration tests, TypeScript lint/type checks, component/E2E tests, dependency scanning, secret scanning, container scanning, and database migration validation.

**Exit criteria:** users can securely sign in, receive the correct tenant/role claims, view an empty dashboard, and all telemetry has tenant-safe correlation.

### Phase 2 — core enterprise knowledge spaces

1. Implement knowledge-space CRUD, lifecycle state, administrators/editors/viewers, group grants, and administration views.
2. Implement document metadata, private Blob upload, document listing, deletion/retention workflow, and source-preview authorization.
3. Implement the queue-backed ingestion worker and status model. Start with the existing supported formats; add OCR only when the decision from Phase 0 is approved.
4. Implement document/chunk/version persistence and pgvector retrieval with authorization filters. Add a reindex operation as an observable job, not an in-request process.

**Exit criteria:** an authorized administrator can upload a document, observe every ingestion state, retry a failure, view the secured original, and retrieve only authorized chunks.

### Phase 3 — Azure AI Foundry RAG experience

1. Implement Azure AI Foundry embeddings and chat through provider interfaces, managed identity/Key Vault configuration, resilient HTTP policies, quotas, and cost telemetry.
2. Port and enhance the grounded-answer orchestration: query policy, retrieval, prompt assembly, response streaming, citation creation, refusal when evidence is insufficient, and prompt-injection isolation of source text.
3. Build Nuxt conversation UI, streaming response rendering, Markdown/KaTeX sanitation, source-citation drawer, document deep links, copy action, and accessible error/retry states.
4. Implement conversations rather than preserving the current flat per-workspace message list. Add search/deletion/retention consistent with enterprise policy.

**Exit criteria:** accepted evaluation cases produce grounded answers with correct, clickable citations; unsupported questions reliably identify the evidence gap; every model operation is observable and attributable.

### Phase 4 — enterprise search, administration, and hardening

1. Add hybrid global search across authorized documents and permitted conversation history, with filters for knowledge space, document, date, and content type.
2. Add admin controls for retrieval/prompt profiles, model deployment selection (where governance permits), quotas, content policy, data retention, and reindex operations.
3. Implement complete audit coverage: identity, grants, document lifecycle, ingestion actions, settings changes, search/access where policy requires, and AI usage events.
4. Add rate limiting, abuse detection, export/deletion workflows, backup/restore testing, disaster-recovery runbooks, accessibility review, load tests, and threat modeling.

**Exit criteria:** security review, accessibility acceptance, load/recovery test results, operational dashboards/alerts, and documented support procedures are complete.

### Phase 5 — data migration, rollout, and retirement

1. Inventory each existing SQLite database and local upload directory. Classify each record as migrate, archive, or discard; do not automatically migrate capstone/test data.
2. Build a one-time migration utility that imports users only if identity mapping is approved, maps workspaces to knowledge spaces, copies source files to Blob Storage, recreates document metadata, and reindexes with the chosen Foundry embedding deployment.
3. Do not migrate serialized Ollama vectors: embeddings from different models/spaces are not interchangeable. Re-chunk/re-embed all retained documents and validate citations.
4. Migrate chat history only if retention/privacy policy permits it; otherwise archive it securely or omit it from the enterprise system.
5. Run pilot tenants in parallel, reconcile document counts/access rules/evaluation results, cut over DNS and access, retain the legacy environment read-only for the agreed period, then decommission it.

**Exit criteria:** migration reconciliation is signed off, pilot acceptance targets are met, rollback and archival plans are tested, and the Go/Ollama deployment has been retired.

## Explicit removals and replacements

Remove these from the runtime once the replacement platform is accepted:

- Ollama service, `models/` image/model blobs, Ollama environment variables, local model health checks, and local-model copy in the UI.
- Go web server, Go templates, HTMX, embedded static assets, and Bubble Tea TUI.
- SQLite database files and filesystem upload paths as the production source of truth.
- Local password/session authentication and the first-run admin bootstrap.
- In-process document indexing and in-process full-corpus cosine scanning.
- Offline Linux Docker/Nginx deployment scripts unless disconnected/on-prem operation remains an approved product requirement.

Retain temporarily as migration references:

- The grounded system-prompt intent and prompt-injection restrictions.
- Go unit tests that define current behavior.
- `evaluation/cases.json` and the evaluation/report pipeline, adapted to call the new API and Foundry deployment.
- The source-document, chunking, citation, and access-control test fixtures.

## Key risks and decisions to resolve early

- **Tenant isolation:** decide database-per-tenant, schema-per-tenant, or shared database with strict `tenant_id` predicates and database controls. Shared database is economical but raises the bar for defense-in-depth testing.
- **Authorization-aware retrieval:** vector and keyword search must never surface chunks outside a caller's effective grants, including group membership changes and document reclassification.
- **Azure AI Foundry data handling:** document which data is sent, to which region/deployment, under which identity, and for how long logs are retained before implementation begins.
- **Model quality and cost:** retrieved evidence, response citations, prompt versions, latency, tokens, and per-tenant spend need measurement from the first pilot.
- **Content connectors and OCR:** each connector/OCR path adds permissions, data-provenance, malware, retention, and reindex requirements. Keep the first release upload-only unless a connector has a clear business case.
- **Enterprise identity:** group-to-role mapping, external guests, service accounts, deprovisioning, and break-glass administration need explicit policy rather than being inferred from the old admin/student model.
- **Document lifecycle:** legal hold, retention, versioning, soft delete, and source preview permissions should be product requirements, not afterthoughts.

## Suggested first-release scope

For a disciplined enterprise v1, ship Entra sign-in, tenant-aware knowledge spaces, group-based access, upload-only PDF/text/Markdown ingestion, Blob Storage, PostgreSQL/pgvector, queue-backed indexing, Azure AI Foundry chat/embeddings, grounded streaming Q&A with citations, secured source preview, admin ingestion/reindex controls, basic global search, audit events, telemetry, and the evaluation gate.

Defer external content connectors, collaborative notes, cross-space answering, advanced model-routing UI, OCR, complex compliance packs, custom role builders, and full conversation sharing until the core authorization/retrieval/audit foundation is proven.
