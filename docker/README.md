# Docker development

Run commands from the repository root. Docker Desktop must use Linux containers.

```powershell
Copy-Item docker/.env.example docker/.env
# Edit local credentials; do not overwrite an existing docker/.env.
docker compose --env-file docker/.env -f docker/compose.yaml -f docker/compose.dev.yaml up --build -d --wait
```

Use `make docker-up`, `docker-logs`, `docker-down`, or `docker-config` if Make is
available. Enable the optional worker with `make docker-worker-up` or add
`--profile worker` before `up`. API: http://localhost:8080/health/live.

Compose runs the one-shot `migrate` service before starting backends. It receives
`MIGRATION_PASSWORD`; runtimes receive `DATABASE_PASSWORD`. Existing legacy volumes
require the [ownership/adoption procedure](../migrations/README.md) first.

## Hot reload

The dev image pins Air v1.64.5, compatible with Go 1.25. Each backend watches the
read-only source mount and rebuilds its cmd/$SERVICE binary into /tmp/air/service.
Polling every second supports Docker Desktop Windows. Test files, docs and
unchanged files are ignored. No host Air installation or foreground watcher is
required. Build failures stop the previous binary to avoid serving stale code.

Reload sends SIGINT and allows 12 seconds before killing. The application shutdown
budget defaults to 10 seconds. Increase Air kill_delay and Compose stop_grace_period
if increasing that budget. Requests may briefly fail during a rebuild/restart.

Module/build caches use named volumes. Dev runs as UID 10001 with a read-only
root/source and a writable 1 GiB tmpfs. Run go mod tidy on the host when adding
dependencies; container builds use -mod=readonly. Initial compilation can take
longer; the API healthcheck has a 90-second startup grace period.

The dev tmpfs explicitly permits execution of the rebuilt binary. Dev builds use
-buildvcs=false to avoid Git ownership errors across Windows/Linux bind mounts;
runtime build metadata is still supplied by Docker build arguments.

Changes to docker/.env, Air config, Compose or Dockerfiles require running
up --build again. Source/module changes reload automatically. These local dev
containers can read repository files; do not use this image for production.

## Runtime image

Omit the dev override to use the multi-stage scratch image without hot reload:

```powershell
docker compose --env-file docker/.env -f docker/compose.yaml up --build -d --wait
```

The runtime image uses UID 65532, CA certificates and embedded timezone data.
Compose remains local-only: Elasticsearch security is disabled, ports bind to
loopback, and production needs its own secrets, ingress and access policies.

## Data

Normal Compose down preserves volumes. Retain existing database credentials;
changing POSTGRES_PASSWORD in the environment does not reset an existing database.
Postgres init scripts only run on empty volumes. Goose migrations are embedded from
`internal/platform/database/schema`; `migrations/` retains the legacy baseline.

Elasticsearch defaults to address-intelligence-platform-elasticsearch-data. Set
ELASTICSEARCH_DATA_VOLUME to an existing volume and ELASTICSEARCH_DATA_EXTERNAL=true when migrating a project name;
never run two Elasticsearch containers against that volume simultaneously.
This workspace reuses address-intelligence_elasticsearch_data; the previous
address-intelligence-elasticsearch-1 container is retained stopped.

Dockerfile-specific ignore files exclude local secrets and Git metadata from image
builds. CI validates both Compose modes. Air is pinned dev tooling inside the dev
image and does not change the application's go.mod.

References: [Air](https://github.com/air-verse/air/tree/v1.64.5).
