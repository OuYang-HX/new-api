# Implementation Plan: Rebase 497 Custom Commits onto Upstream main

## Goal
Rebase our 497 custom commits onto upstream (Calcium-Ion/new-api) latest main, while refactoring custom code into a decoupled architecture to minimize future merge conflicts.

---

## Part 1: Classification of Our Custom Modifications

### Category A: Already Decoupled - custom/ Module (Keep As-Is)

These files live entirely in custom/ and have zero upstream overlap:

- custom/registry.go - Central registration hub
- custom/proxy.go - NO_PROXY wildcard support
- custom/protocol_adapter/doc.go - Package documentation
- custom/protocol_adapter/handler.go - Codex/Claude protocol adapter handlers
- custom/protocol_adapter/responses_to_chat.go - Responses API to Chat Completions conversion
- custom/protocol_adapter/handler_test.go - Tests
- custom/token_config/model.go - TokenConfig and TokenTemplate GORM models
- custom/token_config/controller.go - TokenConfig CRUD controllers
- custom/token_config/service.go - Token refresh scheduler, HTTP login, JSONPath extraction
- custom/token_config/token_template_controller.go - TokenTemplate CRUD controllers
- custom/token_config/token_template_model.go - TokenTemplate GORM model

Conflict risk: NONE

### Category B: Our Unique Files (No Upstream Counterpart) - Need Relocation

These files are entirely ours but live in upstream directories. Relocating to custom/ reduces future conflict risk.

**Move to custom/oauth_provider/:**
- controller/custom_oauth.go -> custom/oauth_provider/controller.go
- model/custom_oauth_provider.go -> custom/oauth_provider/model.go
- model/user_oauth_binding.go -> custom/oauth_provider/binding_model.go
- oauth/generic.go -> custom/oauth_provider/generic.go

**Move to custom/codex/:**
- controller/codex_oauth.go -> custom/codex/controller.go
- controller/codex_usage.go -> custom/codex/usage.go
- service/codex_credential_refresh.go -> custom/codex/credential_refresh.go
- service/codex_credential_refresh_task.go -> custom/codex/credential_refresh_task.go
- service/codex_wham_usage.go -> custom/codex/wham_usage.go
- relay/channel/codex/adaptor.go -> custom/codex/adaptor.go
- relay/channel/codex/oauth_key.go -> custom/codex/oauth_key.go
- relay/channel/codex/constants.go -> custom/codex/constants.go

**Stay in current location:**
- service/waffo_pancake.go and service/waffo_pancake_test.go - payment extension
- relay/channel/openai/relay_responses_compact.go - extends upstream
- dto/openai_image_test.go - small test
- web/default/src/custom/** - already in custom/ subdirectory
- web/default/src/components/data-table/** - shared infra

### Category C: Invasive Modifications to Upstream Core Files - MUST Refactor

These are the primary source of the 299 conflicting files.

**C1: main.go** - 4 custom-hook injection points
- import custom, custom.StartSchedulers(), custom.InitProtocolAdapter(), oauth.LoadCustomProviders()
- InjectUmamiAnalytics/InjectGoogleAnalytics with QuantumNous markers
- Risk: MEDIUM. Strategy: Consolidate into custom.Init(InitParams).

**C2: router/relay-router.go** - 3 injection points
- import custom, codex models endpoint, custom.RegisterRelayRoutes()
- Risk: HIGH. Strategy: Move codex models route into custom.RegisterRelayRoutes()

**C3: router/api-router.go** - Many custom routes
- import custom, custom.RegisterRoutes(), plus scattered custom OAuth/Codex/Waffo routes
- Risk: HIGH. Strategy: Move ALL custom route registrations into custom.RegisterRoutes()

**C4: model/main.go** - 3 injection points
- import custom, CustomOAuthProvider/UserOAuthBinding in migration list, custom.RegisterMigrations()
- Risk: HIGH. Strategy: Remove hardcoded custom models; include in custom.RegisterMigrations()

**C5: service/http_client.go** - 2 injection points
- import custom, custom.ProxyFromEnvironmentWithWildcard
- Risk: LOW. Strategy: Keep as-is.

**C6: relay/channel/api_request.go** - 2 injection points
- import custom, custom.ResolveTokenVariables() for token placeholder resolution
- Risk: MEDIUM. Strategy: Keep as-is.

**C7: controller/oauth.go** - Major rewrite (HIGHEST RISK)
- Unified OAuth handler with oauth.Provider interface
- HandleOAuth(), handleOAuthBind(), findOrCreateOAuthUser(), error types
- Risk: CRITICAL. Strategy: Move unified handler to custom/oauth_provider/handler.go.

**C8: constant/channel.go** - Codex channel type
- ChannelTypeCodex = 57, name mapping
- Risk: MEDIUM. Strategy: Keep in place.

**C9: common/api_type.go** - Codex API type mapping
- Risk: LOW. Strategy: Keep in place.

**C10: controller/channel.go** - Codex-specific handlers
- Codex key validation, RefreshCodexChannelCredential
- Risk: MEDIUM. Strategy: Move to custom/codex/ with hooks.

**C11-C12: controller/channel-test.go, relay/common/relay_info.go**
- Risk: LOW. Strategy: Keep in place.

**C13: oauth/registry.go** - Provider registry (we modified significantly)
- Risk: HIGH. Strategy: Move custom registry into custom/oauth_provider/registry.go.

**C14: oauth/provider.go** - Provider interface (we added methods)
- Risk: HIGH. Strategy: Move our Provider interface to custom/oauth_provider/provider.go.

**C15: oauth/types.go** - OAuth error types
- Risk: MEDIUM. Strategy: Move to custom/oauth_provider/types.go.

**C16: oauth/discord.go, github.go, linuxdo.go, oidc.go** - Built-in providers
- We modified to implement Provider interface
- Risk: HIGH. Strategy: Keep our Provider wrapper versions in custom/oauth_provider/providers/.

**C17: Frontend hooks**
- use-sidebar-data.ts - custom sidebar merge
- channel-mutate-drawer.tsx - TokenPicker import and usage
- Risk: MEDIUM. Strategy: Keep custom-hook comments for easy merge.

---

## Part 2: Architecture Design - Target Decoupled Structure

### Target custom/ Directory Layout

```
custom/
  registry.go                    # Central hub: Init(), RegisterRoutes(), RegisterMigrations(), etc.
  proxy.go                       # NO_PROXY wildcard (unchanged)
  init.go                        # NEW: custom.Init(InitParams) consolidates all initialization
  protocol_adapter/              # (unchanged)
  token_config/                  # (unchanged)
  codex/
    constants.go                 # Model list
    oauth_key.go                 # OAuthKey struct
    adaptor.go                   # Channel adaptor (relay/channel/codex/)
    controller.go                # OAuth flow + usage + refresh (from controller/, service/)
    credential_refresh.go        # Credential refresh logic (from service/)
    credential_refresh_task.go   # Auto-refresh task (from service/)
    wham_usage.go                # WHAM usage fetch (from service/)
  oauth_provider/
    model.go                     # CustomOAuthProvider (from model/)
    binding_model.go             # UserOAuthBinding (from model/)
    controller.go                # CRUD + bindings (from controller/custom_oauth.go)
    generic.go                   # GenericOAuthProvider (from oauth/generic.go)
    handler.go                   # Unified OAuth handler (from controller/oauth.go)
    registry.go                  # Provider registry (from oauth/registry.go)
    provider.go                  # Provider interface (from oauth/provider.go)
    types.go                     # Error types (from oauth/types.go)
    providers/
      discord.go                 # Discord Provider impl (from oauth/discord.go)
      github.go                  # GitHub Provider impl (from oauth/github.go)
      linuxdo.go                 # LinuxDO Provider impl (from oauth/linuxdo.go)
      oidc.go                    # OIDC Provider impl (from oauth/oidc.go)
```

### Hook Pattern: custom-hook Comments

All injection points in upstream files use `// custom-hook:` comment markers. This makes them:
1. Searchable with `grep -n "custom-hook"`
2. Self-documenting
3. Easy to re-apply after upstream changes

Current hook points (12 total across 8 upstream files):
```
main.go:3         import custom // custom-hook: decoupled extensions
main.go:122       custom.StartSchedulers() // custom-hook
main.go:124       custom.InitProtocolAdapter() // custom-hook
router/relay-router.go:6    import custom // custom-hook
router/relay-router.go:44   codex models route // custom-hook
router/relay-router.go:172  custom.RegisterRelayRoutes() // custom-hook
router/api-router.go:5      import custom // custom-hook
router/api-router.go:128    custom.RegisterRoutes() // custom-hook
model/main.go:13            import custom // custom-hook
model/main.go:282-283       CustomOAuthProvider, UserOAuthBinding // custom-hook (to be removed)
model/main.go:287           custom.RegisterMigrations() // custom-hook
model/main.go:337           custom.RegisterMigrationsFast() // custom-hook
service/http_client.go:13   import custom // custom-hook
service/http_client.go:43   custom.ProxyFromEnvironmentWithWildcard // custom-hook
relay/channel/api_request.go:19   import custom // custom-hook
relay/channel/api_request.go:172  custom.ResolveTokenVariables() // custom-hook
```

---

## Part 3: Implementation Tasks (Ordered)

### Phase 0: Preparation (Before Rebase)

1. **Create a backup branch** of current main
   - `git branch backup/pre-rebase-main`
   - Acceptance: backup branch exists

2. **Create a working branch** from current main
   - `git checkout -b refactor/decouple-custom`
   - Acceptance: working branch checked out

3. **Run existing tests** to establish baseline
   - `go test ./...`
   - Acceptance: all tests pass (or record failures)

### Phase 1: Create custom/codex/ Subpackage

4. **Create custom/codex/ directory** and move Codex-specific files
   - Move relay/channel/codex/adaptor.go -> custom/codex/adaptor.go
   - Move relay/channel/codex/oauth_key.go -> custom/codex/oauth_key.go
   - Move relay/channel/codex/constants.go -> custom/codex/constants.go
   - Move service/codex_credential_refresh.go -> custom/codex/credential_refresh.go
   - Move service/codex_credential_refresh_task.go -> custom/codex/credential_refresh_task.go
   - Move service/codex_wham_usage.go -> custom/codex/wham_usage.go
   - Move controller/codex_oauth.go -> custom/codex/controller.go
   - Move controller/codex_usage.go -> custom/codex/usage.go
   - Update package declarations from `codex`/`service`/`controller` to `codex`
   - Acceptance: `go build ./...` succeeds

5. **Update all import paths** referencing moved Codex files
   - Files importing from relay/channel/codex/ -> custom/codex/
   - Files importing from service/codex_* -> custom/codex/
   - Files importing from controller/codex_* -> custom/codex/
   - Acceptance: `go build ./...` succeeds, grep confirms no stale imports

6. **Register Codex channel adaptor** from custom/registry.go
   - Add custom/codex/ adaptor registration in relay adaptor factory
   - Acceptance: Codex channel type still works

### Phase 2: Create custom/oauth_provider/ Subpackage

7. **Create custom/oauth_provider/ directory** and move OAuth provider files
   - Move model/custom_oauth_provider.go -> custom/oauth_provider/model.go
   - Move model/user_oauth_binding.go -> custom/oauth_provider/binding_model.go
   - Move oauth/generic.go -> custom/oauth_provider/generic.go
   - Update package declarations to `oauth_provider`
   - Acceptance: `go build ./...` succeeds

8. **Create custom/oauth_provider/controller.go** from controller/custom_oauth.go
   - Move file, update package, update imports
   - Acceptance: `go build ./...` succeeds

9. **Create custom/oauth_provider/handler.go** from controller/oauth.go
   - Move HandleOAuth(), handleOAuthBind(), findOrCreateOAuthUser()
   - Move OAuthUserDeletedError, OAuthRegistrationDisabledError
   - Move handleOAuthError()
   - Leave only GenerateOAuthCode() in controller/oauth.go
   - Add delegation: controller/oauth.go calls custom.HandleOAuth()
   - Acceptance: `go build ./...` succeeds

10. **Create custom/oauth_provider/providers/** for built-in provider wrappers
    - Move oauth/discord.go -> custom/oauth_provider/providers/discord.go
    - Move oauth/github.go -> custom/oauth_provider/providers/github.go
    - Move oauth/linuxdo.go -> custom/oauth_provider/providers/linuxdo.go
    - Move oauth/oidc.go -> custom/oauth_provider/providers/oidc.go
    - Update to implement our Provider interface
    - Register via init() functions
    - Acceptance: `go build ./...` succeeds

11. **Create custom/oauth_provider/registry.go** from oauth/registry.go
    - Move RegisterCustom, Unregister, LoadCustomProviders, etc.
    - Keep oauth/registry.go as thin wrapper delegating to custom/oauth_provider/registry.go
    - Acceptance: `go build ./...` succeeds

12. **Create custom/oauth_provider/provider.go** from oauth/provider.go
    - Move Provider interface definition
    - Keep oauth/provider.go as thin type alias or re-export
    - Acceptance: `go build ./...` succeeds

13. **Create custom/oauth_provider/types.go** from oauth/types.go
    - Move OAuthError, AccessDeniedError, TrustLevelError
    - Keep oauth/types.go with base types only
    - Acceptance: `go build ./...` succeeds

14. **Update all import paths** referencing moved OAuth files
    - controller/ files -> custom/oauth_provider/
    - router/ files -> custom/oauth_provider/
    - main.go -> custom/oauth_provider/
    - Acceptance: `go build ./...` succeeds

### Phase 3: Consolidate custom.Init() and RegisterRoutes()

15. **Create custom/init.go** with consolidated initialization
    - custom.Init(InitParams) calls:
      - custom.oauth_provider.LoadCustomProviders()
      - custom.StartSchedulers()
      - custom.InitProtocolAdapter()
    - InitParams struct contains: RelayFunc, EnabledModelsFunc, DB
    - Acceptance: `go build ./...` succeeds

16. **Update main.go** to use custom.Init()
    - Replace 3 separate custom.* calls with single custom.Init()
    - Move oauth.LoadCustomProviders() call into custom.Init()
    - Acceptance: `go build ./...` succeeds

17. **Expand custom.RegisterRoutes()** to handle all custom API routes
    - Add params for: apiRouter, channelRoute, optionRoute, selfRoute
    - Move Custom OAuth provider route registration from router/api-router.go
    - Move Codex OAuth route registration from router/api-router.go
    - Move User binding route registration from router/api-router.go
    - Move Waffo Pancake route registration from router/api-router.go
    - Acceptance: `go build ./...` succeeds

18. **Update router/api-router.go** to use expanded custom.RegisterRoutes()
    - Remove inline custom route registrations
    - Keep only the single custom.RegisterRoutes() call
    - Acceptance: `go build ./...` succeeds

19. **Expand custom.RegisterRelayRoutes()** to handle codex models
    - Move /v1/codex/models route from router/relay-router.go
    - Acceptance: `go build ./...` succeeds

20. **Update router/relay-router.go** to use expanded custom.RegisterRelayRoutes()
    - Remove inline codex models route
    - Acceptance: `go build ./...` succeeds

21. **Update model/main.go** to remove hardcoded custom models from migration list
    - Remove CustomOAuthProvider and UserOAuthBinding from AutoMigrate list
    - They are already included in custom.RegisterMigrations()
    - Remove from fast migration list too
    - Acceptance: `go build ./...` succeeds

### Phase 4: Verify and Test

22. **Run full test suite**
    - `go test ./...`
    - Acceptance: all tests pass

23. **Build binary**
    - `go build -o new-api-test .`
    - Acceptance: binary builds successfully

24. **Verify custom-hook grep** shows all injection points
    - `grep -rn "custom-hook" --include="*.go" .`
    - Acceptance: 12+ hook points found, all in upstream files

### Phase 5: Perform the Rebase

25. **Fetch latest upstream**
    - `git fetch upstream`
    - Acceptance: upstream/main is up to date

26. **Start interactive rebase** onto upstream/main
    - `git rebase -i upstream/main`
    - Acceptance: rebase started

27. **Resolve conflicts** using the custom-hook pattern
    - For each conflict in a file with custom-hook comments:
      1. Take upstream version as base
      2. Re-apply custom-hook additions at the same structural points
      3. Search for `custom-hook` markers to find where hooks go
    - For conflicts in files we moved to custom/:
      1. These should have NO conflicts (files don't exist upstream)
      2. If there are conflicts, it means the move wasn't clean
    - Acceptance: rebase completes

28. **Run full test suite** after rebase
    - `go test ./...`
    - Acceptance: all tests pass

29. **Build and smoke test**
    - `go build -o new-api-rebased .`
    - Start server, test basic endpoints
    - Test Codex OAuth flow
    - Test Custom OAuth provider flow
    - Test Internal Token feature
    - Acceptance: all features work

### Phase 6: Frontend Rebase

30. **Handle frontend conflicts** during rebase
    - web/default/src/custom/ - should have no conflicts (our directory)
    - web/default/src/components/data-table/ - may conflict if upstream added similar components
    - web/default/src/hooks/use-sidebar-data.ts - may conflict
    - web/default/src/features/channels/... - likely conflicts
    - web/classic/... - likely conflicts
    - Strategy: take upstream base, re-apply custom-hook imports and usage
    - Acceptance: frontend builds with `bun run build`

31. **Verify frontend builds**
    - `cd web/default && bun install && bun run build`
    - `cd web/classic && npm install && npm run build`
    - Acceptance: both frontends build successfully

---

## Part 4: Files to Modify

### Files to CREATE (new)
- custom/init.go
- custom/codex/ (entire directory, files moved from relay/channel/codex/, service/, controller/)
- custom/oauth_provider/ (entire directory, files moved from model/, controller/, oauth/)

### Files to MOVE (git mv)
- controller/custom_oauth.go -> custom/oauth_provider/controller.go
- controller/codex_oauth.go -> custom/codex/controller.go
- controller/codex_usage.go -> custom/codex/usage.go
- model/custom_oauth_provider.go -> custom/oauth_provider/model.go
- model/user_oauth_binding.go -> custom/oauth_provider/binding_model.go
- oauth/generic.go -> custom/oauth_provider/generic.go
- relay/channel/codex/adaptor.go -> custom/codex/adaptor.go
- relay/channel/codex/oauth_key.go -> custom/codex/oauth_key.go
- relay/channel/codex/constants.go -> custom/codex/constants.go
- service/codex_credential_refresh.go -> custom/codex/credential_refresh.go
- service/codex_credential_refresh_task.go -> custom/codex/credential_refresh_task.go
- service/codex_wham_usage.go -> custom/codex/wham_usage.go

### Files to MODIFY (hook consolidation)
- main.go - consolidate custom.Init()
- router/relay-router.go - expand custom.RegisterRelayRoutes()
- router/api-router.go - expand custom.RegisterRoutes()
- model/main.go - remove hardcoded custom models from migrations
- controller/oauth.go - delegate to custom/oauth_provider/handler.go
- custom/registry.go - expand RegisterRoutes(), RegisterMigrations(), add Init()

### Files to MODIFY (import path updates)
- All files importing from moved packages (approximately 20-30 files)

---

## Part 5: Dependencies

- Tasks 4-6 (codex move) must complete before task 15 (consolidate init)
- Tasks 7-14 (oauth_provider move) must complete before task 15
- Task 15 (custom.Init) must complete before task 16 (update main.go)
- Task 17-21 (route consolidation) can run in parallel with task 15-16
- Tasks 22-24 (verify) depend on all prior tasks
- Tasks 25-29 (rebase) depend on tasks 22-24
- Tasks 30-31 (frontend) can run in parallel with backend rebase resolution

---

## Part 6: Risks

### HIGH RISK

1. **OAuth Provider Interface Divergence** - Upstream may have its own OAuth abstraction that differs from ours. If they introduced a similar Provider pattern, our custom/oauth_provider/provider.go may need significant adaptation. MITIGATION: During rebase, carefully compare upstream's oauth/ with our custom/oauth_provider/.

2. **controller/oauth.go Rewrite** - This file was essentially rewritten by us. If upstream also modified it significantly, the rebase will be very complex. MITIGATION: Our strategy of moving the unified handler to custom/ means controller/oauth.go becomes minimal (just GenerateOAuthCode + delegation), making the rebase conflict smaller.

3. **router/api-router.go Route Explosion** - Both we and upstream have added many routes. The structural layout may differ significantly. MITIGATION: Moving all our routes into custom.RegisterRoutes() means the upstream api-router.go only has one custom hook line.

4. **relay/channel/codex/ Adaptor Registration** - The relay channel adaptor factory uses registration tables. Moving codex/ to custom/ requires updating the registration mechanism. MITIGATION: Use init() function or explicit registration in custom.Init().

### MEDIUM RISK

5. **Circular Import Dependencies** - Moving files between packages may create circular imports. For example, custom/codex/ importing from controller/ while controller/ imports from custom/codex/. MITIGATION: Use interface injection (like SetRelayFunc pattern already used in protocol_adapter).

6. **GORM Model Migration** - Moving CustomOAuthProvider and UserOAuthBinding to custom/oauth_provider/ changes their Go package but NOT their database table names (TableName() method stays the same). This should be safe, but needs verification.

7. **Frontend Build Breaking** - Moving CodexOAuthModal.jsx or changing import paths in frontend may break builds. MITIGATION: Keep frontend custom/ directory structure stable.

### LOW RISK

8. **Lost Git History** - Using git mv preserves history for most git operations but may not for complex rebases. MITIGATION: The backup branch preserves full history.

9. **Test Coverage Gaps** - Some custom features may lack tests, making verification harder after rebase. MITIGATION: Manual smoke testing of all custom features.

---

## Part 7: Post-Rebase Maintenance Strategy

### Ongoing Conflict Minimization

1. **All new custom code goes in custom/** - No exceptions
2. **Upstream files only get custom-hook comment lines** - One import + one function call
3. **custom/registry.go is the single integration point** - All hooks route through it
4. **Quarterly rebase cadence** - Don't let the gap grow again
5. **Automated conflict detection** - CI job that checks for custom-hook markers in upstream files

### Future Rebase Procedure

1. Fetch upstream
2. Search for custom-hook markers in current code
3. Rebase, taking upstream versions
4. Re-apply custom-hook markers at same structural points
5. Run tests
6. Build and verify

---

## Appendix: Complete custom-hook Marker List

All current injection points (searchable with grep -rn "custom-hook"):

```
main.go:16        import custom // custom-hook: decoupled extensions
main.go:122       custom.StartSchedulers() // custom-hook
main.go:124       custom.InitProtocolAdapter() // custom-hook
router/relay-router.go:6    import custom // custom-hook
router/relay-router.go:44   Codex models route // custom-hook
router/relay-router.go:172  custom.RegisterRelayRoutes() // custom-hook
router/api-router.go:5      import custom // custom-hook
router/api-router.go:128    custom.RegisterRoutes() // custom-hook
model/main.go:13            import custom // custom-hook
model/main.go:282-283       CustomOAuthProvider, UserOAuthBinding // custom-hook
model/main.go:287           custom.RegisterMigrations() // custom-hook
model/main.go:337           custom.RegisterMigrationsFast() // custom-hook
service/http_client.go:13   import custom // custom-hook
service/http_client.go:43   custom.ProxyFromEnvironmentWithWildcard // custom-hook
relay/channel/api_request.go:19   import custom // custom-hook
relay/channel/api_request.go:172  custom.ResolveTokenVariables() // custom-hook
web/.../use-sidebar-data.ts:36    import custom sidebar // custom-hook
web/.../use-sidebar-data.ts:167   merge custom sidebar items // custom-hook
web/.../channel-mutate-drawer.tsx:168  import TokenPicker // custom-hook
```
