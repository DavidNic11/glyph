# Fork notes — DavidNic11/glyph

Fork of [imaustink/glyph](https://github.com/imaustink/glyph), deployed to the
`nicholascloudlab.com` Pi cluster ([DavidNic11/pi-cluster](https://github.com/DavidNic11/pi-cluster)).

Three deliberate divergences from upstream. Everything else should track upstream, and
`main` should be rebased on it rather than allowed to drift.

## 1. OIDC endpoints come from discovery, not from Google

**This is a bug fix, not a customisation, and it should go upstream.**

`setupOIDCAuth` built its `oauth2.Config` with `Endpoint: google.Endpoint`, so the
authorize and token URLs were Google's regardless of `OIDC_ISSUER_URL` — while
`OIDC_ISSUER_URL` *was* honoured for JWKS discovery and for the `jwt.WithIssuer` check
on the returned ID token.

That combination cannot work for any provider except Google, and fails confusingly:

* `/auth/login` redirects the user to `accounts.google.com` carrying a client_id Google
  has never heard of.
* Had a code come back, `Exchange` would POST it to `oauth2.googleapis.com/token`.
* Any ID token Google *did* issue would carry `iss: https://accounts.google.com` and be
  rejected by `jwt.WithIssuer(OIDC_ISSUER_URL)`.

`internal/auth/discovery.go` adds `DiscoverEndpoint`, which reads
`authorization_endpoint` and `token_endpoint` out of the provider's
`/.well-known/openid-configuration` — the same document this package already fetches for
`jwks_uri`. It also enforces OIDC Discovery §4.3 (the document's own `issuer` must match
`OIDC_ISSUER_URL`), because a mismatch there otherwise survives the entire browser
round-trip and only surfaces as an opaque `invalid_id_token`.

Discovery failure is fatal at boot. Falling back to a hardcoded provider is exactly the
bug being removed.

Google publishes a conforming discovery document, so **this is not a behaviour change for
a Google deployment** — which is what makes it a clean upstream PR.

This cluster's provider is [Pocket ID](https://github.com/pocket-id/pocket-id) at
`https://idp.nicholascloudlab.com`.

## 2. `helm/glyph/migrations/` is committed

Upstream `.gitignore`s it and `cd.yml` copies `api/migrations` into the chart immediately
before `helm upgrade`. That works for a pipeline that runs Helm from a checkout.

This cluster deploys with Argo CD, which renders `helm/glyph` straight from git. Nothing
runs first — so an ignored `migrations/` means `.Files.Glob "migrations/*.sql"` matches
nothing, the migrations ConfigMap renders **empty**, the `run-migrations` init container
applies nothing and reports success, and the API starts against an empty schema.

`scripts/sync-migrations.sh` regenerates the copy; `--check` verifies it and runs in CI
before anything is pushed. Run it after adding a migration.

> The ConfigMap holds every `.sql` file (~136KB across 32 files today) against a 1MiB
> object limit. Not close yet, but it is the ceiling this approach eventually hits.

## 3. `migrate.job.enabled`

New chart value, defaulting to `true` so upstream behaviour is unchanged.

The chart's standalone migration Job is named `<fullname>-migrate-{{ .Release.Revision }}`.
Under a GitOps controller that renders with `helm template` there is no release history, so
the revision is always `1` and the name never changes. That breaks two ways:

* `ttlSecondsAfterFinished: 300` deletes the Job five minutes after it completes; the
  controller then sees it missing from live state and recreates it. Migrations re-run every
  five minutes, forever.
* A Job's pod template is immutable, so any later change to the migrate image or command is
  rejected on apply — an unclosable diff that parks the application out of sync.

The Job is redundant under GitOps anyway: the api Deployment runs the same `migrate up` as
a `run-migrations` init container on every pod start, and a chart change that alters
migrations alters the api pod spec too, so the pods roll and the migrations run.

## 4. `.github/workflows/pi-cluster-images.yml`

Builds both images arm64 on an in-cluster ARC runner (`runs-on: arc-glyph`) and pushes to
`registry.nicholascloudlab.com`. Upstream's `ci.yml` build-and-push and `cd.yml` are both
guarded by `github.repository == 'imaustink/glyph'` and stay inert here.

There is no deploy step: Argo reconciles continuously, so deploying is a tag bump in
`pi-cluster`'s `charts/glyph/values.yml`. The workflow's job summary prints the exact
command.

Requires `REGISTRY_USERNAME` / `REGISTRY_PASSWORD` repo secrets —
`manifests/glyph/seal-secrets.sh` in `pi-cluster` sets them from the cluster's own
credentials, so the two cannot drift.
