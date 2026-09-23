# Address Intelligence Platform

Nền tảng place-centric để quản lý, tìm kiếm và phân giải dữ liệu địa chỉ Việt Nam.

## Baseline MVP

- Go modular monolith với các executable độc lập trong `cmd/`.
- PostgreSQL/PostGIS là canonical source of truth.
- Elasticsearch là derived search read model.
- Transactional outbox polling đồng bộ canonical data sang search index.
- Canonical identity chỉ thuộc Administrative Unit, Place/Street, Geometry và Delivery Point.
- Address là kết quả parser/resolver, không có canonical ID bền vững.
- Redis, broker, CDC và `LISTEN/NOTIFY` chưa thuộc MVP.

## Cấu trúc

```text
cmd/                    Executable entry points
  api/                  HTTP API runtime
  indexer/              Outbox polling và Elasticsearch indexing
  worker/               Import/background jobs
internal/               Private application packages
  administrative/       Đơn vị hành chính và lịch sử thay đổi
  place/                Street, POI, building và địa danh
  resolution/           Parser/resolver tạo kết quả địa chỉ
  autocomplete/         Gợi ý tìm kiếm
  search/               Search contracts và ranking
  geocoding/            Forward/reverse geocoding
  deliverypoint/        Điểm giao nhận
  datasource/           Nguồn dữ liệu và provenance
  importjob/            Vòng đời import
  outbox/               Transactional outbox
  platform/             Config và infrastructure adapters
  bootstrap/            Composition root cho từng runtime
api/openapi/             OpenAPI contracts
migrations/              PostgreSQL/PostGIS migrations
docker/                  Docker images, Compose and dev hot reload
test/                    Integration, E2E và fixtures
docs/                    Product, architecture và database design
```

## Bắt đầu

Yêu cầu Go 1.25.13 trở lên; toolchain CI/Docker hiện pin 1.25.13 để bao gồm các bản vá bảo mật standard library.

```bash
go test ./...
go vet ./...
go run ./cmd/api
```

### Configuration Foundation

Ba runtime dùng chung `internal/platform/config`: đọc environment variables, áp dụng mặc định và validate trước khi khởi động. Cấu hình gồm môi trường, HTTP address/timeouts, log level, database/search URL, outbox poll interval, shutdown timeout và OTLP endpoint.

Xem [hướng dẫn cấu hình](configs/README.md) để biết đầy đủ biến, mặc định, alias và cách chạy local/Docker. Ứng dụng không tự nạp `.env`; `.env.example` chỉ là mẫu. Database cấu hình bằng `DATABASE_URL` hoặc các trường `DATABASE_*` riêng, bắt buộc ở staging/production, tùy chọn ở development/test. API/indexer cần search URL tường minh ở staging/production; mỗi runtime bỏ qua nhóm cấu hình không sử dụng.

CI chạy gofmt check, dependency boundaries, module integrity, vet, test với race detector, build, validate Compose và vulnerability scan trên mỗi push/pull request; xem [workflow](.github/workflows/ci.yml). Toolchain CI được pin trong `.go-version`.

### Dependency Management

Thư viện được duyệt, quyền import theo layer/module và quy trình nâng cấp nằm trong [Quản lý dependency](docs/L%C3%AA%20Ph%C6%B0%E1%BB%9Bc%20Th%E1%BA%AFng%20-%20Address%20Intelligence%20Platform%20-%20Qu%E1%BA%A3n%20l%C3%BD%20dependency.md). Policy được thực thi bằng `go run ./tools/dependencycheck`; application chỉ giao tiếp module khác qua `contract`, không gọi repository chéo module. Registry chốt lựa chọn nhưng chưa cài thư viện chưa được code sử dụng.

### Application Bootstrap

Entry point dùng `bootstrap.NewAPI/NewIndexer/NewWorker` rồi `App.Run(ctx)`. App sở hữu tài nguyên trong Run, cleanup theo thứ tự ngược kể cả khi startup lỗi, và drain HTTP với deadline trước khi force-close. Logger được tạo riêng theo runtime; xem [Application Bootstrap](docs/L%C3%AA%20Ph%C6%B0%E1%BB%9Bc%20Th%E1%BA%AFng%20-%20Address%20Intelligence%20Platform%20-%20Application%20Bootstrap.md) cho lifecycle, thứ tự wiring và giới hạn skeleton.

API có PostgreSQL pool và readiness kiểm tra schema/quyền database. Indexer/worker mới khởi tạo rồi chờ tín hiệu dừng; search adapter và outbox polling chưa được tích hợp.

### Chạy bằng Docker

Dev có hot reload tự động cho Go bằng Air. Xem [Docker dev guide](docker/README.md)
cho chế độ dev, runtime image và cách giữ dữ liệu khi đổi Compose project.

```powershell
Copy-Item docker/.env.example docker/.env
docker compose --env-file docker/.env -f docker/compose.yaml -f docker/compose.dev.yaml up --build -d
docker compose --env-file docker/.env -f docker/compose.yaml -f docker/compose.dev.yaml ps
```

API health endpoints:

```text
GET http://localhost:8080/health/live
GET http://localhost:8080/health/ready
```

Worker là profile tùy chọn:

```bash
docker compose --env-file docker/.env -f docker/compose.yaml -f docker/compose.dev.yaml --profile worker up --build -d
```

### Database schema

Goose runner trong `cmd/migrate` quản lý schema; Compose chạy migration trước khi khởi động backend. Migration mới nằm trong `internal/platform/database/schema/`; SQL trong `migrations/` là baseline legacy bất biến.

Xem [database/migration runbook](migrations/README.md) cho khởi tạo mới, ownership/adoption database cũ, phân quyền và deploy/recovery. Không apply baseline thủ công cho database mới.

Kiểm tra các invariant bằng transaction tự rollback:

```powershell
docker exec address-intelligence-platform-postgres-1 `
  psql -v ON_ERROR_STOP=1 -U address_app -d address_intelligence `
  -f /scripts/verify_database.sql
```

Compose dành cho local development và integration test. Elasticsearch single-node đang tắt security, PostgreSQL dùng password local và các port chỉ bind vào loopback; không dùng cấu hình này nguyên trạng trong production.

> [!NOTE]
> Module path hiện là `address-intelligence-platform`. Hãy đổi sang URL repository chính thức trước khi publish module hoặc thêm package được consumer bên ngoài import.

## Tài liệu thiết kế

Quy tắc đóng góp và commit: [Quy ước coding](docs/L%C3%AA%20Ph%C6%B0%E1%BB%9Bc%20Th%E1%BA%AFng%20-%20Address%20Intelligence%20Platform%20-%20Quy%20%C6%B0%E1%BB%9Bc%20coding.md).

- [Tổng quan dự án](docs/Lê%20Phước%20Thắng%20-%20Address%20Intelligence%20Platform%20-%20Tổng%20quan%20dự%20án.md)
- [Kiến trúc kỹ thuật](docs/Lê%20Phước%20Thắng%20-%20Address%20Intelligence%20Platform%20-%20Kiến%20trúc%20kỹ%20thuật%20và%20cấu%20trúc%20dự%20án.md)
- [Thiết kế cơ sở dữ liệu](docs/Lê%20Phước%20Thắng%20-%20Address%20Intelligence%20Platform%20-%20Thiết%20kế%20cơ%20sở%20dữ%20liệu.md)

### Error Foundation

Domain/application errors and the shared HTTP error contract are documented in
[Error Foundation](docs/Lê%20Phước%20Thắng%20-%20Address%20Intelligence%20Platform%20-%20Error%20Foundation.md). Use `apperror` for stable error identities and
`httpserver.Adapt` for centralized status mapping and safe JSON responses.


### HTTP Server Foundation

`internal/platform/httpserver` owns the router, configured server, middleware,
JSON request/response helpers and error mapper. Bootstrap retains listener and
graceful shutdown ownership. See [HTTP Server Foundation](docs/Lê%20Phước%20Thắng%20-%20Address%20Intelligence%20Platform%20-%20HTTP%20Server%20Foundation.md).

### Standard API Response

Success, pagination, validation errors, status codes and UTC timestamps follow
[Standard API Response](docs/Lê%20Phước%20Thắng%20-%20Address%20Intelligence%20Platform%20-%20Standard%20API%20Response.md). HTTP helpers own the envelope;
endpoints pass DTOs and return typed errors.

### Logging Foundation

Runtime logs, slog structure, and HTTP access logging policy are documented in
[Logging Foundation](docs/Lê%20Phước%20Thắng%20-%20Address%20Intelligence%20Platform%20-%20Logging%20Foundation.md).

### Middleware Foundation

See [Middleware Foundation](<docs/Lê Phước Thắng - Address Intelligence Platform - Middleware Foundation.md>) for CORS (localhost:3000),
recovery with stack traces, timeout, rate limiting, OIDC auth integration,
OpenTelemetry tracing and development metrics at `/metrics`.

### Context Foundation

Pass `ctx context.Context` first through usecases, repository ports and adapters.
HTTP handlers forward `r.Context()`; repositories must not create new root contexts.
The existing dependency check also enforces context syntax and root ownership.
See [Context Foundation](<docs/Lê Phước Thắng - Address Intelligence Platform - Context Foundation.md>)
for cancellation, deadlines, worker lifecycles and the limits of static checking.


### Quy tắc tài liệu

Tài liệu chuyên đề trong `docs/` dùng tên `Lê Phước Thắng - Address Intelligence Platform - <Tên tài liệu>.md`. Xem [quy tắc đặt tên và ngoại lệ](<docs/Lê Phước Thắng - Address Intelligence Platform - Quy ước coding.md>) trước khi tạo tài liệu mới.

- [Kiến trúc xử lý địa chỉ Việt Nam (HLD)](<docs/Lê Phước Thắng - Address Intelligence Platform - Kiến trúc xử lý địa chỉ Việt Nam.md>)
