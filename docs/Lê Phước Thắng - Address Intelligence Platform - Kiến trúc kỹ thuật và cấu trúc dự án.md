# Address Intelligence Platform
## Technology Stack & Project Structure

| Thuộc tính | Giá trị |
|---|---|
| **Document Type** | Technical Architecture |
| **Project** | Address Intelligence Platform |
| **Author** | Lê Phước Thắng |
| **Status** | Baseline Architecture |
| **Version** | `1.2.0` |
| **Date** | 20/09/2026 |
| **Architecture Style** | Modular Monolith, Service-ready |
| **Primary Backend Language** | Go |

---

# 1. Architecture Overview

## 1.1 Goals

Thiết kế hệ thống theo các nguyên tắc:

- Simple to start
- Easy to maintain
- Easy to test
- Easy to scale
- Không over-engineering
- Có thể tách service sau này mà không phải viết lại toàn bộ codebase

Kiến trúc khuyến nghị:

> **Modular Monolith + Domain-oriented Structure + Clear Module Boundaries**

MVP áp dụng mô hình **place-centric**: canonical identity chỉ tồn tại cho Administrative Unit, Place/Street, Geometry và Delivery Point. `Address` là kết quả của parser/resolver tại thời điểm truy vấn, không phải aggregate canonical, không có repository hay ID bền vững.

Không bắt đầu bằng Microservices.

Các nguyên tắc chính:

- Tổ chức source code theo domain/module.
- Business logic không phụ thuộc trực tiếp database, cache, search engine hoặc framework.
- Module không truy cập trực tiếp implementation nội bộ của module khác.
- Infrastructure có thể thay thế mà ít ảnh hưởng đến business logic.
- API runtime stateless.
- Observability được tích hợp từ đầu.
- Không tạo `common`, `shared`, `utils` để chứa business logic.

Dependency chuẩn:

```text
Transport
    ↓
Application
    ↓
Domain
```

Infrastructure implement các interface cần thiết:

```text
Infrastructure
      ↓
Domain / Application Contract
```

Không cho phép:

```text
Domain → PostgreSQL
Domain → Redis
Module A → Internal implementation của Module B
```

---

# 2. Technology Stack

| Layer | Technology |
|---|---|
| Backend Language | Go |
| HTTP API | `net/http` + lightweight router (`chi`) |
| External API | REST / JSON |
| Internal Communication | gRPC khi cần |
| Database | PostgreSQL |
| Geospatial | PostGIS |
| Database Driver | pgx |
| SQL Code Generation | sqlc |
| Cache | Không thuộc baseline MVP; chỉ bổ sung Redis sau benchmark |
| Search Engine | Elasticsearch |
| Message Broker | Không thuộc baseline MVP |
| Database Migration | Goose v3 (runner riêng trong cmd/migrate; xem migrations/README.md) |
| API Specification | OpenAPI |
| Serialization | JSON / Protobuf |
| Observability | OpenTelemetry |
| Metrics | Prometheus |
| Dashboard | Grafana |
| Logging | `log/slog`, structured JSON |
| Container | Docker |
| Orchestration | Kubernetes khi cần |
| Infrastructure as Code | Terraform |
| CI/CD | GitHub Actions / GitLab CI |
| Frontend | React + TypeScript |
| Data Processing / ML | Python khi cần |

## Backend

Go là ngôn ngữ backend chính cho:

- Address API
- Autocomplete
- Search
- Normalization
- Validation
- Geocoding
- Reverse Geocoding
- Ranking
- Background workers
- Search indexing

Không sử dụng nhiều backend language nếu chưa có lý do kỹ thuật rõ ràng.

## Data

### PostgreSQL + PostGIS

PostgreSQL là source of truth.

PostGIS xử lý:

- point / geometry / polygon
- spatial query
- nearest location
- administrative boundary
- reverse geocoding
- spatial indexing

**Quy tắc ranh giới không gian (Spatial Boundary Rule):**

> **PostGIS-specific geometry types must remain inside the infrastructure layer. sqlc may use go-geom/go-geos type overrides where required; spatial types are mapped into domain value objects before entering the application/domain layers.**

Luồng chuyển đổi dữ liệu không gian:

```text
sqlc
  ↓
go-geom / go-geos (type overrides)
  ↓
Infrastructure Mapper
  ↓
Domain Coordinate / Geometry (Domain Value Object)
```

Ví dụ Domain giữ sự trong sạch, không phụ thuộc PostGIS hay GIS library:

```go
type Coordinate struct {
    Latitude  float64
    Longitude float64
}
```

Infrastructure chịu trách nhiệm toàn bộ việc quét/đọc PostGIS sang `go-geom`/`go-geos` và map sang `domain.Coordinate`.

### Cache

MVP không triển khai Redis. Autocomplete và search đọc trực tiếp Elasticsearch; dữ liệu canonical đọc từ PostgreSQL/PostGIS. Chỉ bổ sung Redis sau khi benchmark chứng minh cache cải thiện đáng kể latency/capacity và đã xác định rõ invalidation, consistency cùng chi phí vận hành.

### Search Engine (Elasticsearch)

Elasticsearch dùng cho:

- autocomplete
- prefix search
- fuzzy matching
- typo tolerance
- relevance scoring
- ranking

**Chiến lược Search Analyzer:**

- **Index analyzer:**
  ```text
  standard tokenizer
        ↓
    lowercase
        ↓
   asciifolding
        ↓
    edge_ngram
  ```
- **Search analyzer:**
  ```text
  standard tokenizer
        ↓
    lowercase
        ↓
   asciifolding
  ```
  *(Lưu ý: Không dùng `edge_ngram` ở phía search query; chỉ dùng ở index time).*
- **Về plugin `analysis-icu`:** Standard tokenizer vốn đã dựa trên Unicode Text Segmentation (UAX #29) và đáp ứng tốt phần lớn nhu cầu tiếng Việt ban đầu. `analysis-icu` không bắt buộc ngay từ đầu, sẽ được đưa vào thử nghiệm và đánh giá sau khi có benchmark relevance thực tế.

## Async Processing & Search Synchronization

MVP chốt sử dụng: **Transactional Outbox Pattern + Polling**.

```text
               ┌──► outbox_events ───► cmd/indexer ───► Elasticsearch
DB transaction ┤
               └──► administrative_units / places / geometries / delivery_points
```

1. **Transactional Outbox:**
   - Khi API cập nhật dữ liệu canonical (`administrative_units`, `places`, geometry hoặc `delivery_points`), bản ghi sự kiện được ghi đồng thời vào bảng `outbox_events` trong cùng một database transaction (`BEGIN ... COMMIT`).
   - Đảm bảo tính nhất quán tuyệt đối giữa dữ liệu nghiệp vụ và sự kiện đồng bộ tìm kiếm.
2. **Indexer Polling:**
   - `cmd/indexer` định kỳ poll các sự kiện chưa xử lý từ `outbox_events`, xử lý theo batch, bulk index sang Elasticsearch, sau đó đánh dấu `processed`.
3. **Ranh giới MVP:**
   - Không dùng `LISTEN/NOTIFY`, message broker hoặc CDC trong baseline MVP.
   - Chỉ bổ sung cơ chế wake-up/broker khi benchmark chứng minh polling không đáp ứng indexing-lag SLO.

---

# 3. Application Architecture

Các business module chính:

```text
Application
│
├── Resolution
├── Autocomplete
├── Search
├── Geocoding
├── Administrative
├── Place
├── Data Source
└── Import Job
```

Mỗi module sở hữu:

- Domain model
- Application use cases
- Infrastructure adapter
- Transport adapter

Cấu trúc module sở hữu canonical data, ví dụ Place:

```text
internal/place/
│
├── domain/
│   ├── place.go
│   ├── value_object.go
│   ├── repository.go
│   └── errors.go
│
├── application/
│   ├── command/
│   ├── query/
│   └── service.go
│
├── infrastructure/
│   └── postgres/
│       ├── repository.go
│       ├── queries.sql
│       └── mapper.go
│
└── transport/
    └── http/
        ├── handler.go
        ├── routes.go
        ├── request.go
        └── response.go
```

Ví dụ Autocomplete:

```text
HTTP Handler
      ↓
Autocomplete Application
      ↓
Normalize Query
      ↓
Search Engine
      ↓
Ranking
      ↓
Suggestions
```

Cross-module communication phải thông qua package `contract` chứa public interface và DTO thuần. Application của consumer chỉ import contract của provider; không import application, domain hoặc repository implementation của module khác. Bootstrap khởi tạo implementation và inject qua constructor.

Quy tắc import được kiểm tra tự động bằng `go run ./tools/dependencycheck`, registry tại `dependency-policy.json`. Xem [Quản lý dependency](Quản%20lý%20dependency.md) để biết thư viện đã chốt, layer được phép import, quy trình versioning và giới hạn checker. Root module chỉ chứa `doc.go`; khi triển khai public API, thêm `contract/` cạnh domain/application/infrastructure/transport.

Ví dụ đúng:

```text
Autocomplete
     ↓
PlaceSearchReader
     ↓
Place Module
```

Không:

```text
Autocomplete
     ↓
Place PostgreSQL Repository implementation
```

---

# 4. Recommended Repository Structure

```text
address-intelligence-platform/
│
├── cmd/
│   ├── api/
│   │   └── main.go
│   ├── worker/
│   │   └── main.go
│   ├── indexer/
│   │   └── main.go
│   └── admin-web/
│
├── internal/
│   ├── resolution/
│   │   ├── domain/
│   │   ├── application/
│   │   ├── infrastructure/
│   │   └── transport/
│   │
│   ├── autocomplete/
│   │   ├── domain/
│   │   ├── application/
│   │   ├── infrastructure/
│   │   └── transport/
│   │
│   ├── search/
│   │   ├── domain/
│   │   ├── application/
│   │   ├── infrastructure/
│   │   └── transport/
│   │
│   ├── geocoding/
│   │   ├── domain/
│   │   ├── application/
│   │   ├── infrastructure/
│   │   └── transport/
│   │
│   ├── administrative/
│   ├── place/
│   ├── datasource/
│   ├── importjob/
│   │
│   ├── platform/
│   │   ├── database/
│   │   ├── searchengine/
│   │   ├── telemetry/
│   │   ├── logging/
│   │   └── config/
│   │
│   └── bootstrap/
│
├── api/
│   ├── openapi/
│   └── proto/
│
├── migrations/
├── configs/
├── deployments/
│   ├── docker/
│   ├── kubernetes/
│   └── terraform/
│
├── docs/
│   ├── architecture/
│   ├── adr/
│   ├── api/
│   ├── database/
│   └── diagrams/
│
├── scripts/
├── test/
│   ├── integration/
│   ├── e2e/
│   └── fixtures/
│
├── tools/
├── .github/
│   └── workflows/
│
├── .env.example
├── .gitignore
├── docker-compose.yml
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

## Runtime

Ban đầu có thể giữ chung repository nhưng tách runtime:

```text
Repository
│
├── API Runtime
├── Worker Runtime
└── Indexer Runtime
```

Ví dụ:

```text
cmd/api
    ↓
resolution
search
autocomplete

cmd/indexer
    ↓
search
datasource

cmd/worker
    ↓
importjob
geocoding
```

`main.go` chỉ làm bootstrap:

Implementation hiện tại dùng `bootstrap.NewAPI/NewIndexer/NewWorker` rồi `App.Run(ctx)`. Constructor đọc config và tạo logger riêng; Run mở tài nguyên, chạy runtime và cleanup LIFO với deadline chung kể cả khi startup thất bại. HTTP drain quá hạn sẽ cancel request context và force-close. Xem [Application Bootstrap](Application%20Bootstrap.md) cho cấu trúc và quy tắc lifecycle; database/search adapter cùng repositories/services nghiệp vụ vẫn chưa được nối vào runtime.

```text
load config
    ↓
initialize logger
    ↓
initialize telemetry
    ↓
initialize database
    ↓
initialize search engine
    ↓
initialize modules
    ↓
start runtime
```

Không đặt business logic trong `main.go`.

---

# 5. API, Data & Infrastructure Conventions

## API

API versioning:

```text
/v1
```

Ví dụ:

```text
GET  /v1/addresses/autocomplete
POST /v1/addresses/search
POST /v1/addresses/normalize
POST /v1/addresses/resolve
GET  /v1/geocoding/reverse
```

OpenAPI specification:

```text
api/openapi/
├── resolution.yaml
├── autocomplete.yaml
├── geocoding.yaml
└── common.yaml
```

## Database

Ưu tiên:

```text
pgx + sqlc
```

Không ưu tiên ORM lớn.

SQL đặt gần repository của module:

```text
internal/place/infrastructure/postgres/
├── queries.sql
├── repository.go
└── mapper.go
```

**Mapping PostGIS với `sqlc`:**
- `sqlc` cấu hình `overrides` trong `sqlc.yaml` để map các cột `geometry` sang kiểu dữ liệu của `go-geom` hoặc `go-geos`.
- File `mapper.go` tại infrastructure layer chịu trách nhiệm chuyển đổi từ kiểu GIS sang Domain Value Object (ví dụ: `domain.Coordinate{Latitude, Longitude}`). Registry duyệt `go-geom` khi cần; `go-geos`/CGO cần ADR riêng vì Docker build hiện tắt CGO.
- Domain layer tuyệt đối không import thư viện GIS hay PostGIS driver.

Migration:

```text
migrations/
├── 000001_create_administrative_units.sql
├── 000002_create_streets.sql
├── 000003_create_places.sql
├── 000004_create_delivery_points.sql
├── 000005_create_outbox_events.sql
└── 000006_create_indexes.sql
```

Migration đã deploy production phải immutable.

## Vietnamese Text Processing & Normalization

Quy chuẩn xử lý văn bản và địa chỉ tiếng Việt:

Sử dụng package chính thức của Go: `golang.org/x/text/unicode/norm`.

Luồng chuẩn hóa:

```text
Input Text (UTF-8)
       ↓
Unicode Normalization (NFC)
       ↓
Whitespace Normalization (trim, single space)
       ↓
Address-specific Normalization (viết tắt, tiền tố hành chính)
```

**Nguyên tắc bảo toàn dữ liệu chuẩn (Canonical Preservation):**

- **Không lưu toàn bộ dữ liệu ở dạng bỏ dấu:** Dữ liệu gốc/chuẩn (Canonical text) luôn phải được lưu đầy đủ dấu tiếng Việt chuẩn Unicode NFC.
- Dạng không dấu (accent-insensitive / folded) chỉ là **biểu diễn phục vụ tìm kiếm (search representation)**, không thay thế canonical record.

Ví dụ:

| Biểu diễn | Dữ liệu mẫu | Mục đích |
|---|---|---|
| **Original** | `Đường  Nguyễn Thị  Minh Khai` | Đầu vào người dùng / hệ thống cũ |
| **Canonical** | `Đường Nguyễn Thị Minh Khai` | Lưu trữ Source of Truth (NFC, chuẩn hóa khoảng trắng) |
| **Search Normalized** | `đường nguyễn thị minh khai` | Tìm kiếm chính xác có dấu, case-insensitive |
| **Folded / Search Variant** | `duong nguyen thi minh khai` | Tìm kiếm không dấu (accent-insensitive) |

## Configuration

Configuration hiện được triển khai tại `internal/platform/config/`, dùng chung cho `cmd/api`, `cmd/indexer`, `cmd/worker`. Mỗi entry point gọi `LoadFor` theo runtime để chỉ đọc/validate nhóm cấu hình sử dụng; `Load()` hỗ trợ kiểm tra toàn bộ nhóm. Đọc từ process environment, áp dụng mặc định rồi validate trước khi khởi tạo runtime; không tự đọc file `.env`.

```text
APP_ENV
LOG_LEVEL
HTTP_ADDRESS
HTTP_READ_HEADER_TIMEOUT
HTTP_READ_TIMEOUT
HTTP_WRITE_TIMEOUT
HTTP_IDLE_TIMEOUT
DATABASE_URL
DATABASE_HOST / DATABASE_PORT / DATABASE_NAME
DATABASE_USER / DATABASE_PASSWORD / DATABASE_SSLMODE
ELASTICSEARCH_URL
OUTBOX_POLL_INTERVAL
SHUTDOWN_TIMEOUT
OTEL_EXPORTER_OTLP_ENDPOINT
```

Tên chính theo code là `HTTP_ADDRESS` (mặc định `:8080`) và `ELASTICSEARCH_URL` (mặc định `http://localhost:9200` chỉ ở development/test). `HTTP_PORT` và `SEARCH_URL` chỉ là alias tương thích; tên chính có giá trị sẽ được ưu tiên. API/indexer bắt buộc khai báo search URL ở staging/production; worker bỏ qua search và HTTP. Outbox poll interval chỉ được indexer sử dụng.

`APP_ENV` nhận `development`, `test`, `staging`, `production`, mặc định `development`; deployment phải đặt biến này rõ ràng. Database bắt buộc ở staging/production; development/test cho phép trống để chạy skeleton. `DATABASE_URL` ưu tiên hơn trường riêng. Nếu dùng trường riêng, host/name/user bắt buộc, port mặc định `5432`, SSL mode mặc định `verify-full`; code encode credential bằng `net/url`. Compose local dùng trường riêng và SSL mode `disable`. URL được kiểm tra scheme/host/port; duration phải hợp lệ và lớn hơn 0. Lỗi cấu hình làm process thoát mã 1, thông báo không chứa giá trị thô. Parser theo PostgreSQL driver sẽ bổ sung khi triển khai adapter.

HTTP timeout và log level đã được áp dụng vào runtime. `OUTBOX_POLL_INTERVAL` hiện mới được indexer đọc/ghi log; database/search URL và OTLP endpoint mới được đọc/validate, chưa khởi tạo adapter/exporter. Hai health endpoint hiện trả trạng thái tĩnh; readiness chưa kiểm tra dependency. Các phần kiến trúc mô tả adapter, polling và observability bên dưới là hướng triển khai tiếp theo.

Xem [Configuration reference](../configs/README.md) cho bảng mặc định, quy tắc ưu tiên và cách truyền biến trong local/Docker Compose.

Không commit:

- password
- secret
- token
- production connection string

---

# 6. Observability, Error Handling & Testing

## Observability

Logging dùng structured JSON và tối thiểu có:

```text
timestamp
level
service
module
operation
request_id
trace_id
duration
status
```

Metrics quan trọng:

```text
http_request_total
http_request_duration_seconds
autocomplete_request_total
search_latency_seconds
search_error_total
db_query_duration_seconds
```

Tracing sử dụng OpenTelemetry:

```text
Client
  ↓
API
  ↓
Search Engine
  ↓
PostgreSQL
```

Grafana dùng để theo dõi:

- throughput
- error rate
- P95 / P99 latency
- DB latency
- Search Engine latency
- worker backlog

## Error Handling

Infrastructure error không được leak trực tiếp ra client.

```text
PostgreSQL Error
       ↓
Repository
       ↓
Application / Domain Error
       ↓
HTTP Error Mapper
       ↓
API Response
```

Response chuẩn:

```json
{
  "error": {
    "code": "ADDRESS_NOT_FOUND",
    "message": "Address not found"
  }
}
```

## Testing

Unit test đặt cạnh source code:

```text
address.go
address_test.go
```

Integration test:

```text
test/integration/
```

E2E test:

```text
test/e2e/
```

Ưu tiên test với PostgreSQL/PostGIS và Elasticsearch thật bằng container thay vì mock toàn bộ infrastructure.

---

# 7. Maintainability & Scaling Rules

Không tổ chức toàn bộ project theo kiểu:

```text
internal/
├── controllers/
├── services/
├── repositories/
├── models/
└── utils/
```

Nên tổ chức:

```text
internal/
├── address/
├── autocomplete/
├── search/
├── geocoding/
└── place/
```

Tức là:

> **Domain first, layer second.**

Không tạo generic abstraction quá sớm như:

```text
BaseRepository
GenericService
CommonManager
SharedBusinessUtils
```

Chỉ abstraction khi đã có ít nhất một nhu cầu thực tế rõ ràng.

Khi một module cần scale hoặc deployment độc lập:

```text
Modular Monolith
       ↓
Extract Module
       ↓
Independent Service
```

Ví dụ:

```text
Before

API
├── Address
├── Search
└── Autocomplete


After

API Gateway
├── Address Service
└── Autocomplete Service
```

Do boundary đã được thiết kế sẵn, việc chuyển từ:

```text
in-process call
```

sang:

```text
gRPC / HTTP
```

không yêu cầu viết lại business core.

---

# 8. Final Decision

Baseline architecture của Address Intelligence Platform:

```text
Backend
├── Go
├── REST
├── gRPC when required
├── pgx
└── sqlc

Data
├── PostgreSQL
├── PostGIS
└── Elasticsearch

Observability
├── OpenTelemetry
├── Prometheus
└── Grafana

Frontend
├── React
└── TypeScript

Infrastructure
├── Docker
├── Kubernetes when required
└── Terraform
```

Quyết định kiến trúc:

> **Go + Modular Monolith + Domain-oriented Structure + PostgreSQL/PostGIS + Elasticsearch + Transactional Outbox Polling**

Ưu tiên:

- maintainability
- clear ownership
- explicit dependencies
- predictable performance
- simple deployment
- easy testing
- future service extraction

Mục tiêu cuối cùng:

> **Architecture phục vụ business và khả năng phát triển lâu dài, không phải architecture để phô diễn độ phức tạp.**
