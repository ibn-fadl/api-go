# api

[![Test](https://github.com/naturallyfunny/api-go/actions/workflows/test.yaml/badge.svg)](https://github.com/naturallyfunny/api-go/actions/workflows/test.yaml)
[![Go Reference](https://pkg.go.dev/badge/go.naturallyfunny.dev/api.svg)](https://pkg.go.dev/go.naturallyfunny.dev/api)
![Go 1.25](https://img.shields.io/badge/go-1.25-00ADD8?logo=go&logoColor=white)
![Dependencies: none](https://img.shields.io/badge/dependencies-0-success)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

A small, dependency-free toolkit for moving request-scoped identity and context
between **HTTP headers** and Go's **`context.Context`** — in both directions,
using nothing but the standard library.

```
        inbound request                                outbound request
  ┌──────────────────────────┐                   ┌──────────────────────────┐
  │  user-id: u_123          │                   │  user-id: u_123          │
  │  session-id: s_789       │                   │  session-id: s_789       │
  │  time-zone: Asia/Jakarta │                   │  time-zone: Asia/Jakarta │
  └───────────┬──────────────┘                   └─────────────▲────────────┘
              │  HTTPWith… (ingest middleware)                 │  Transport + Propagator
              ▼                                                │  (re-emit)
        context.Context ───────────►  your handler  ───────────┘
        user.ContextKey = u_123        business logic
        session.ContextKey = s_789     downstream calls
        time.ContextKey  = Asia/Jakarta
```

The middleware **reads** headers into the context at the edge of your service.
The `http.Transport` **writes** those same values back onto every outbound
request. The loop is symmetric on purpose: context that enters your service can
be propagated, unchanged, to the services it calls.

---

## Table of contents

- [Why this exists](#why-this-exists)
- [Design principles](#design-principles)
- [Install](#install)
- [Quick start (end-to-end)](#quick-start-end-to-end)
- [Packages](#packages)
- [Design decisions & trade-offs](#design-decisions--trade-offs)
- [Non-goals](#non-goals)
- [Compatibility](#compatibility)
- [Status & roadmap](#status--roadmap)
- [License](#license)

---

## Why this exists

Almost every HTTP service ends up doing the same three chores:

1. Pull a handful of ambient values off the incoming request — who the caller
   is, which session they belong to, which timezone to render times in.
2. Carry those values through the call stack without threading a parameter
   through every function signature.
3. Put them back on the wire when calling the next service, so the trail is not
   lost at the first hop.

None of that is hard. It is, however, easy to do *inconsistently* — a slightly
different context key here, a header typo there, a validation check that one
handler forgot. This module is the boring, correct version of those three
chores, factored into packages small enough to read in a single sitting and
audit line by line.

It is deliberately unambitious. There is no framework, no router, no reflection,
no code generation, no configuration file. That is the point.

## Design principles

- **Standard library only.** `go.mod` has zero `require` lines. Depending on
  this module pulls in nothing else. See [Compatibility](#compatibility).
- **Small packages named for what they provide.** `user.IDFromContext`,
  `session.HTTPWithID`, `time.ZoneFromContext` read as sentences. The package
  name *is* part of the API.
- **Fail closed at the boundary.** Constructors validate their input and return
  an `error`; invalid data never reaches your context.
- **Honest names over ceremonial ones.** Headers are `user-id`, `session-id`,
  `time-zone` — not `X-`-prefixed. See
  [the header naming decision](#3-non-standard-lowercase-header-names) for why
  that is the *more* correct choice, with citations.
- **No magic.** Every exported symbol does exactly one obvious thing. If a
  reviewer has to guess what a line does, that line is a bug.

## Install

```sh
go get go.naturallyfunny.dev/api
```

```go
import (
	apihttp "go.naturallyfunny.dev/api/http"
	"go.naturallyfunny.dev/api/session"
	apitime "go.naturallyfunny.dev/api/time"
	"go.naturallyfunny.dev/api/user"
)
```

> The `http` and `time` packages intentionally share their name with the
> standard library. Alias them at the import site (`apihttp`, `apitime`) rather
> than renaming the packages — the domain name is the right name; the collision
> is the caller's to resolve. See
> [the aliasing decision](#7-owning-a-package-named-http).

## Quick start (end-to-end)

The two halves below are the whole library in use: ingest on the way in,
propagate on the way out.

### Server — headers into context

```go
package main

import (
	"net/http"

	apihttp "go.naturallyfunny.dev/api/http"
	"go.naturallyfunny.dev/api/session"
	apitime "go.naturallyfunny.dev/api/time"
	"go.naturallyfunny.dev/api/user"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /whoami", whoami)

	// Compose the ingest middleware. Each layer is fail-closed: a missing or
	// invalid header short-circuits with a problem response, so `whoami` only
	// ever runs with a fully populated context.
	handler := user.HTTPWithID(
		session.HTTPWithID(
			apitime.HTTPWithZone(mux),
		),
	)

	_ = http.ListenAndServe(":8080", handler)
}

func whoami(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// The errors are safe to ignore here precisely because the middleware
	// already guaranteed these values are present and valid.
	uid, _ := user.IDFromContext(ctx)
	sid, _ := session.IDFromContext(ctx)
	tz, _ := apitime.ZoneFromContext(ctx)

	apihttp.WriteJSON(w, http.StatusOK, map[string]any{
		"user":     uid,
		"session":  sid,
		"timezone": tz,
	})
}
```

Try it:

```sh
curl -s localhost:8080/whoami \
  -H 'user-id: u_123' \
  -H 'session-id: s_789' \
  -H 'time-zone: Asia/Jakarta'
# {"session":"s_789","timezone":"Asia/Jakarta","user":"u_123"}

curl -si localhost:8080/whoami -H 'session-id: s_789' -H 'time-zone: Asia/Jakarta'
# HTTP/1.1 401 Unauthorized
# {"detail":"user ID cannot be empty"}

curl -si localhost:8080/whoami -H 'user-id: u_123' -H 'session-id: s_789' -H 'time-zone: Mars/Olympus'
# HTTP/1.1 400 Bad Request
# {"detail":"invalid IANA timezone \"Mars/Olympus\": unknown time zone Mars/Olympus"}
```

### Client — context back onto the wire

```go
// Build a client that re-emits the ambient identity on every outbound request.
// Installing a propagator is always safe: it is a no-op when the value is
// absent from the context, so the same client works for authenticated and
// anonymous flows alike.
client := &http.Client{
	Transport: &apihttp.Transport{
		Propagators: []apihttp.Propagator{
			apihttp.WithHeader(user.ContextKey, "user-id"),
			apihttp.WithHeader(session.ContextKey, "session-id"),
			apihttp.WithHeader(apitime.ContextKey, "time-zone"),
		},
	},
}

// `ctx` flows in from the inbound request (r.Context()); the identity rides
// along to the downstream service without any manual header plumbing.
req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://downstream/internal", nil)
resp, err := client.Do(req)
```

That is the complete round trip: `HTTPWithID` put `u_123` into the context on
the way in; `WithHeader(user.ContextKey, "user-id")` took it back out on the way
out — and the key that links the two is exported but
[impossible to forge](#2-an-exported-context-key-of-an-unexported-type).

## Packages

| Package                       | Ingest (header → context)              | Accessor (context → value)      | Context handle          |
| ----------------------------- | -------------------------------------- | ------------------------------- | ----------------------- |
| `.../user`                    | `HTTPWithID` — `user-id` → **401**     | `IDFromContext`                 | `user.ContextKey`       |
| `.../session`                 | `HTTPWithID` — `session-id` → **401**  | `IDFromContext`                 | `session.ContextKey`    |
| `.../time`                    | `HTTPWithZone` — `time-zone` → **400** | `ZoneFromContext`               | `time.ContextKey`       |
| `.../http`                    | —                                      | `Transport`, `Propagator`, `WithHeader` | —               |
| `.../http` (responses)        | —                                      | `WriteJSON`, `WriteProblem`     | —                       |

Each domain package follows the identical two-file shape:

- **`context.go`** — the `ContextKey` handle, a validating `ContextWith…`
  constructor, and a `…FromContext` accessor. No HTTP knowledge.
- **`http.go`** — a single middleware that bridges one header to that context
  value. No business logic.

That regularity is intentional: once you have read one package, you have read
them all, and adding a fourth (a request ID, a locale, a tenant) is a
copy-paste-rename away.

## Design decisions & trade-offs

This is the section a reviewer should read. Nothing here is accidental; each
choice below had a plausible alternative that was considered and rejected for a
stated reason.

### 1. Per-domain packages, not one `middleware` package

**Choice.** `user`, `session`, and `time` are separate packages, each owning its
key, constructor, accessor, and middleware.

**Alternative.** A single `middleware` (or `ctxkit`) package with
`middleware.UserID`, `middleware.SessionID`, and so on — fewer directories,
one import.

**Why this way.** In Go the package name is a mandatory, un-droppable prefix on
every identifier, so it should carry meaning. `user.IDFromContext(ctx)` reads as
a sentence; `middleware.UserIDFromContext(ctx)` stutters and says nothing about
*user*. Separate packages also mean a consumer that only needs timezones does
not compile in the user and session code, and each package has a blast radius of
exactly one concern. The cost — more files, more import lines — is real but
cheap, and the earlier history of this repo did in fact remove a generic
middleware abstraction in favour of this shape.

### 2. An exported context key of an *unexported* type

**Choice.**

```go
type key struct{}          // unexported type
var ContextKey = key{}     // exported value of that type
```

This is the subtle one, and it is load-bearing.

The canonical Go advice is to keep context keys **unexported** to prevent
collisions between packages ([`context`
documentation](https://pkg.go.dev/context#WithValue): *"The provided key must be
comparable and should not be of type string or any other built-in type to avoid
collisions between packages using context."*). Following that advice literally
would make the key invisible outside its package — but the outbound
`Propagator` layer lives in a *different* package (`http`) and must name the key
to read the value back out.

The resolution: export the **value** (`ContextKey`) while keeping its **type**
(`key`) unexported. Because `key` is an unexported, zero-size struct, no other
package can construct a second `key{}` to collide with it — the type is unique
to this package and un-nameable from outside. Yet `WithHeader(user.ContextKey,
…)` can still hand the key across the package boundary as an opaque token.

**Alternatives considered.**
- *Unexported key + an exported getter function passed around* — more
  indirection, and `Propagator` would need a `func(context.Context) (string,
  bool)` per key instead of a uniform `any` handle.
- *An exported `string`/`int` key* — collision-prone and exactly what the
  standard library warns against.

The chosen form is collision-safe *and* usable across packages, which is
precisely what the two-directional design needs. `struct{}` also costs zero
bytes.

### 3. Non-standard, lowercase header names

**Choice.** `user-id`, `session-id`, `time-zone` — plain, lowercase, no `X-`
prefix.

**Why not `X-User-Id`.** The `X-` convention is cargo-culted and was formally
retired years ago:

- **[RFC 6648](https://www.rfc-editor.org/rfc/rfc6648)** (2012) — *"Deprecating
  the 'X-' Prefix and Similar Constructs in Application Protocols."* The prefix
  never conferred any protection or special handling; it only created a
  permanent `X-`/non-`X-` migration problem once a header became standardised.
- **[RFC 9113 §8.3](https://www.rfc-editor.org/rfc/rfc9113)** (HTTP/2) — field
  names are **lowercase on the wire regardless**, so any capitalisation you
  choose is cosmetic. Go's `net/http` canonicalises to `User-Id` for HTTP/1.x
  presentation ([RFC 9110
  §5.1](https://www.rfc-editor.org/rfc/rfc9110#section-5.1): field names are
  case-insensitive), which is why `r.Header.Get("user-id")` and
  `h.Set("user-id", …)` interoperate cleanly whatever case the source uses.

So `X-` would add letters, add a future rename hazard, and buy nothing. The
honest name is the correct one. The trade-off we accept in exchange: these
names are not from any registry, so they must be documented — which is what this
table and the middleware source do.

### 4. Fail-closed constructors with 401 vs 400 semantics

**Choice.** `ContextWith…` rejects empty input; `ContextWithZone`
additionally validates the value against `time.LoadLocation`. The middleware
translates those failures into HTTP status codes that mean different things:

- **Missing/empty `user-id` or `session-id` → `401 Unauthorized`** — the caller
  has not established *who* they are. ([RFC 9110
  §15.5.2](https://www.rfc-editor.org/rfc/rfc9110#section-15.5.2).)
- **Missing/invalid `time-zone` → `400 Bad Request`** — the request itself is
  malformed; this is not an authentication question. ([RFC 9110
  §15.5.1](https://www.rfc-editor.org/rfc/rfc9110#section-15.5.1).)

**Why.** Pushing validation to the boundary means every handler downstream can
treat the context values as trusted and non-empty, and the `_ =` on the
accessors in the quick start is *safe by construction*, not by laziness. The
alternative — let handlers each decide what an absent user means — spreads the
same check across the codebase and invites drift.

**Known nuance (on the roadmap).** A strict reading of RFC 9110 §15.5.2 pairs
`401` with a `WWW-Authenticate` header. This module omits it because no
authentication *scheme* is defined here (the header is expected to be set by a
trusted edge). Emitting a scheme is deferred until one is formalised; `403` is
the alternative if these headers are ever treated as authorization rather than
identity.

### 5. `WriteJSON` and `WriteProblem` look identical — on purpose

**Choice.** Two nearly line-for-line functions, one for success payloads and one
for error payloads.

**Why not one function.** They *are* the same today. They will not stay that
way, and the call sites should not have to change when they diverge:

- `WriteProblem` is named and shaped after **[RFC 9457](https://www.rfc-editor.org/rfc/rfc9457)**
  (Problem Details for HTTP APIs, formerly RFC 7807): the `{"detail": …}` body
  is already the RFC's field. When it grows `type`/`title`/`status` and switches
  its media type to `application/problem+json`, only `WriteProblem` changes —
  every error call site is already correct.
- Reading `WriteProblem(w, 401, …)` at a call site tells you *this is the error
  path* without decoding the status argument.

The duplication is a deliberate seam for future divergence, and the cost is two
short, obvious functions. If a linter flags them as clones, that is the linter
missing the intent, not a defect — this note exists so a human reviewer does
not make the same mistake.

### 6. `Transport` clones the request before touching headers

**Choice.** `RoundTrip` does `clone := req.Clone(ctx)` and mutates `clone`,
never `req`.

**Why.** The
[`http.RoundTripper` contract](https://pkg.go.dev/net/http#RoundTripper) is
explicit: *"RoundTrip should not modify the request, except for consuming and
closing the Request's Body."* Callers may retry or inspect the original request;
mutating it in place is a latent data race and a contract violation. Cloning is
the small, correct price. `Propagators` run in order, and each is a no-op when
its value is absent, so a `Transport` is safe to install unconditionally — the
anonymous path simply emits no identity headers.

### 7. Owning a package named `http`

**Choice.** The response/transport helpers live in a package literally named
`http`, and internally the code aliases the standard library as
`nethttp "net/http"`.

**Alternative.** Rename the package (`httpx`, `apihttp`, `transport`) to dodge
the clash entirely.

**Why this way.** `http` is the honest, minimal name for what the package holds.
The collision is purely lexical and is resolved with a one-line import alias at
each site that needs both — a cost paid by the *caller who chose to import
both*, not baked into the package's identity forever. The same reasoning applies
to `time`. The README's import block shows the convention (`apihttp`,
`apitime`), so the friction is documented rather than surprising.

## Non-goals

To keep the surface honest, this module deliberately does **not** provide:

- A router, mux, or framework — bring your own; the middleware is a plain
  `func(http.Handler) http.Handler`.
- Authentication or authorization — it *transports* an identity, it does not
  *verify* one. Trust of the source headers is the edge's job.
- Structured logging, tracing, or metrics — a `Propagator` can carry a trace ID,
  but this module ships none.
- Serialization beyond `encoding/json` for the two response helpers.

## Compatibility

- **Go 1.25+** (per `go.mod`).
- **Zero external dependencies.** `go.mod` declares no `require`d modules; the
  entire library is standard-library code. This keeps your dependency graph, and
  your supply-chain audit, one line longer than it was.

## Development

Run the checks locally:

```sh
go vet ./...
go test ./...
```

A `pre-push` hook (`.githooks/pre-push`) runs the same checks automatically and
**aborts the push** if they fail, so nothing that breaks the suite reaches the
remote. Git does not clone hook configuration, so enable it once per clone:

```sh
git config core.hooksPath .githooks
```

The hook is a local, fast gate on your machine; the
[`Test`](.github/workflows/test.yaml) workflow is the independent cloud check
(with `-race`) that also runs on anything pushed without the hook.

## Status & roadmap

The code is stable and the public API above is what it will remain. Packaging
polish is still in progress:

- [x] MIT `LICENSE`.
- [x] Table-driven tests across all four packages (`go test ./...`).
- [ ] Published, versioned tags for `pkg.go.dev` indexing and semantic import
      versioning.
- [ ] Full RFC 9457 problem bodies behind `WriteProblem`.

## License

[MIT](LICENSE) © 2026 Ardian.
