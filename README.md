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
deployments/docker/      Local container deployment
test/                    Integration, E2E và fixtures
docs/                    Product, architecture và database design
```

## Bắt đầu

Yêu cầu Go 1.25 trở lên.

```bash
go test ./...
go vet ./...
go run ./cmd/api
```

### Configuration Foundation

Ba runtime dùng chung `internal/platform/config`: đọc environment variables, áp dụng mặc định và validate trước khi khởi động. Cấu hình gồm môi trường, HTTP address/timeouts, log level, database/search URL, outbox poll interval, shutdown timeout và OTLP endpoint.

Xem [hướng dẫn cấu hình](configs/README.md) để biết đầy đủ biến, mặc định, alias và cách chạy local/Docker. Ứng dụng không tự nạp `.env`; `.env.example` chỉ là mẫu. Database cấu hình bằng `DATABASE_URL` hoặc các trường `DATABASE_*` riêng, bắt buộc ở staging/production, tùy chọn ở development/test. API/indexer cần search URL tường minh ở staging/production; mỗi runtime bỏ qua nhóm cấu hình không sử dụng.

CI chạy gofmt check, vet, test với race detector, build và validate Compose trên mỗi push/pull request; xem [workflow](.github/workflows/ci.yml).

Hiện API có health endpoint; indexer/worker mới khởi tạo rồi chờ tín hiệu dừng. Database/search adapter, outbox polling và telemetry exporter chưa được tích hợp. `/health/ready` hiện trả trạng thái tĩnh, chưa xác nhận kết nối PostgreSQL/Elasticsearch.

### Chạy bằng Docker

```powershell
Copy-Item .env.docker.example .env.docker
docker compose --env-file .env.docker up --build -d
docker compose --env-file .env.docker ps
```

API health endpoints:

```text
GET http://localhost:8080/health/live
GET http://localhost:8080/health/ready
```

Worker là profile tùy chọn:

```bash
docker compose --env-file .env.docker --profile worker up --build -d
```

### Database schema

Core schema nằm trong `migrations/000001_create_core_schema.sql`. Với database volume mới, apply migration một lần:

```powershell
docker exec address-intelligence-platform-postgres-1 `
  psql -v ON_ERROR_STOP=1 -U address_app -d address_intelligence `
  -f /migrations/000001_create_core_schema.sql
```

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

Quy tắc đóng góp và commit: [Coding convention](coding%20convention.md).

- [Tổng quan dự án](docs/Lê%20Phước%20Thắng%20-%20Address%20Intelligence%20Platform%20-%20Tổng%20quan%20dự%20án.md)
- [Kiến trúc kỹ thuật](docs/Lê%20Phước%20Thắng%20-%20Address%20Intelligence%20Platform%20-%20Kiến%20trúc%20kỹ%20thuật%20và%20cấu%20trúc%20dự%20án.md)
- [Thiết kế cơ sở dữ liệu](docs/Lê%20Phước%20Thắng%20-%20Address%20Intelligence%20Platform%20-%20Thiết%20kế%20cơ%20sở%20dữ%20liệu.md)
