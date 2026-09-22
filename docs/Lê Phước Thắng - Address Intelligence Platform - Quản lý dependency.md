# Quản lý dependency

## Quyết định nền tảng

Dự án là Go modular monolith. Dependency được quản lý ở hai mức: thư viện bên ngoài và quyền import giữa module/layer. Quy tắc thực thi nằm trong [`dependency-policy.json`](../dependency-policy.json); CI chạy [`tools/dependencycheck`](../tools/dependencycheck/main.go).

Registry dưới đây chốt lựa chọn cho team. **Được duyệt không có nghĩa đã cài đặt**: hiện runtime chỉ dùng standard library. Chỉ thêm thư viện vào `go.mod` khi có implementation thực sự sử dụng; không thêm blank import để giữ dependency dự phòng.

| Hạng mục | Lựa chọn | Ai được import / trạng thái |
|---|---|---|
| HTTP | `net/http`; router `github.com/go-chi/chi/v5` | Hiện dùng ServeMux; chi là router được duyệt khi bổ sung routes/middleware, chỉ bootstrap và `*/transport/http`. Không dùng thêm Gin/Echo/Fiber |
| PostgreSQL | `github.com/jackc/pgx/v5`; `pgxpool` | pgx chỉ `platform/database` và `*/infrastructure/postgres`; pgxpool chỉ `platform/database` để quản lý pool tập trung. Chưa cài |
| SQL generation | `sqlc` | Tool lúc build/dev, không import vào runtime; code sinh trong `*/infrastructure/postgres`, map sang domain trước khi trả về. Pin version khi tích hợp |
| Elasticsearch | `github.com/elastic/go-elasticsearch/v9` | `platform/searchengine`, `*/infrastructure/elasticsearch`; chọn major phù hợp Elasticsearch 9. Chưa cài |
| Redis | Không thuộc MVP | Import bị chặn. Nếu benchmark và ADR duyệt cache, ưu tiên `github.com/redis/go-redis/v9`, chỉ adapter; phải bổ sung policy trước |
| Logger | `log/slog`, JSON stdout | Factory ở `platform/logging`; application có thể nhận `*slog.Logger` qua constructor. Domain/contract không log. Không thêm Zap/Logrus |
| Validation | Go thuần cho business/config; `github.com/go-playground/validator/v10` cho HTTP DTO khi cần | Validator chỉ `*/transport/http`; bật `WithRequiredStructEnabled` khi tích hợp. Domain tự bảo vệ invariant, không phụ thuộc tag validator |
| Telemetry | OpenTelemetry Go | API/instrumentation chỉ transport, infrastructure, `platform/telemetry`; SDK/exporter chỉ `platform/telemetry`. Domain/application dùng `context.Context`, không import OTel |
| Migration | Goose v3, SQL migration | Chốt Goose thay cho lựa chọn Goose/Atlas. CLI/tool deploy riêng, pin version khi tích hợp; không import vào API/indexer/worker. Runner đã tích hợp trong `cmd/migrate`, pin qua go.mod |
| Testing | `testing`, `httptest`, table tests, fake viết tay | Mặc định standard library; `testify` chỉ `_test.go` nếu cần. Testcontainers chỉ `_test.go` trong `test/integration` hoặc `test/e2e`. Không mock repository của module khác để vượt boundary |
| Geospatial | PostGIS; `github.com/twpayne/go-geom` nếu sqlc cần mapping | Chỉ `*/infrastructure/postgres`; chưa cài. `go-geos`/CGO cần ADR vì Docker runtime hiện dùng `CGO_ENABLED=0` |
| Security tooling | `golang.org/x/vuln/cmd/govulncheck@v1.7.0` | Dev/CI, không thêm vào runtime module; chọn bản tương thích Go 1.25 |

`go.mod` và `go.sum` là nguồn version thực tế khi thư viện được dùng. `.go-version` pin toolchain CI, hiện đồng bộ với Dockerfile ở Go `1.25.13`; `go.mod` cũng yêu cầu tối thiểu bản vá này sau khi govulncheck phát hiện lỗ hổng standard library ở `1.25.12`. Major trong registry là ranh giới API được duyệt, không phải cho phép dùng version tùy ý trong build.

## Ranh giới import

Mỗi module dùng các package rõ ràng: `domain`, `application`, `contract`, `infrastructure`, `transport`. Root `internal/<module>/doc.go` chỉ chứa package documentation, không đặt service/repository tại root để tránh bypass layer.

| Package nguồn | Được import trong dự án |
|---|---|
| `cmd/*` | `internal/bootstrap` |
| `internal/bootstrap` | Các layer/module và platform để khởi tạo, inject dependency |
| `internal/platform/*` | Platform khác; không phụ thuộc business module |
| `<module>/domain` | Domain của chính module |
| `<module>/contract` | Contract của chính module; không import domain hay contract module khác |
| `<module>/application` | Application/domain/contract cùng module; contract của module khác |
| `<module>/infrastructure` | Infrastructure/application/domain/contract cùng module; platform; contract của module khác |
| `<module>/transport` | Transport/application/domain/contract cùng module; platform logging/telemetry |
| Unit test `_test.go` | Giữ nguyên boundary của package; thêm assertion library được duyệt |
| `test/integration`, `test/e2e` (`_test.go`) | Được nối các module/adapter để kiểm thử tích hợp; không phải đường import cho production |
| `tools/*` | Standard library và tool dependency được duyệt; không import application code |

Domain, application và contract chỉ dùng nhóm standard library thuần được liệt kê trong policy. `context`, `time`, `errors`, `fmt`, nhóm xử lý chuỗi/encoding/toán nằm trong danh sách; `net/http`, `database/sql`, `os`, `os/exec`, `unsafe` bị chặn ở các layer này. `database/sql` chỉ được dùng trong PostgreSQL adapters và integration/e2e tests; các package `testing`/`httptest` chỉ trong test. Application có thêm `log/slog`, unit test có `testing` và assertion library được duyệt. Khi cần stdlib khác, team cập nhật policy có lý do; không lách qua package `utils`.

Các module đã đăng ký: administrative, autocomplete, datasource, deliverypoint, geocoding, importjob, outbox, place, resolution, search. Dự án không có canonical `address` module; ví dụ Address của bài toán ánh xạ sang `resolution` trong code hiện tại. Thêm module/platform component/layer phải cập nhật policy và tài liệu.

## Giao tiếp qua contract và dependency injection

Ví dụ dependency hợp lệ:

```text
resolution/application -> autocomplete/contract
autocomplete/application -> autocomplete/contract
bootstrap -> khởi tạo Autocomplete service -> inject vào Resolution service
```

Ví dụ minh họa cho use case tương lai (không phải API đã triển khai):

```go
// internal/autocomplete/contract/suggestions.go
package contract

import "context"

type Suggestion struct {
    Label string
}

type Suggester interface {
    Suggest(ctx context.Context, query string) ([]Suggestion, error)
}
```

```go
// internal/resolution/application/service.go
package application

import autocomplete "address-intelligence-platform/internal/autocomplete/contract"

type Service struct {
    suggestions autocomplete.Suggester
}

func NewService(suggestions autocomplete.Suggester) *Service {
    return &Service{suggestions: suggestions}
}
```

Implementation của Autocomplete nằm trong `autocomplete/application`; bootstrap gọi constructor của cả hai và truyền service đáp ứng interface. Có thể khai báo compile-time assertion `var _ contract.Suggester = (*Service)(nil)` ở phía implementation.

- Contract là capability nghiệp vụ nhỏ và DTO/value thuần. Không lộ `pgx.Rows`, transaction, SQL models, HTTP request, Elasticsearch response hoặc domain entity của provider.
- Repository interface thuộc domain/application của **module sở hữu dữ liệu**. Không export repository dưới tên contract để module khác truy cập bảng trực tiếp.
- Nếu consumer cần abstraction khác, khai báo port tại application của consumer và viết adapter trong infrastructure của consumer, adapter chỉ gọi contract của provider.
- Không khởi tạo repository/client trong application; dependency đi qua constructor. Chỉ bootstrap quyết định implementation và lifecycle.
- Transaction nằm trong module sở hữu write. Luồng liên module dùng application contract hoặc outbox/event contract; không truyền DB transaction hay gọi repository chéo module.
- Import checker không phát hiện SQL truy cập bảng module khác hoặc vòng gọi service ở runtime. Reviewer phải kiểm tra ownership dữ liệu, vòng gọi, side effect và backward compatibility của contract.

## Quy trình thêm hoặc nâng cấp dependency

1. Tác giả PR nêu use case, lý do standard library/thư viện hiện có chưa đáp ứng, license, maintenance, Go compatibility, dependency bắc cầu và ảnh hưởng vận hành.
2. Maintainer dự án review thay đổi registry/policy. Dependency hoặc framework mới chưa có trong policy bị CI chặn; thay major, thêm CGO, cache/broker hoặc thay migration tool cần ADR. Hiện chủ dự án là maintainer; chưa cấu hình CODEOWNERS/branch protection tự động.
3. Chọn version cụ thể: `go get <module>@vX.Y.Z`, không dùng `@latest`, branch hoặc `go get -u ./...` trong CI/build. Tool cũng pin version cụ thể; không cài vào runtime nếu chỉ phục vụ dev.
4. Chạy `go mod tidy`, review `go.mod`/`go.sum`, commit cả hai nếu có thay đổi. Không sửa `go.sum` bằng tay. Không dùng `replace` local, `go.work` hoặc nested module để né review; checker chặn các trường hợp này.
5. Chạy boundary checker, test, build, module integrity và vulnerability scan. Không tự động merge dependency update; review breaking changes và migration notes.

Không import trực tiếp dependency bắc cầu chỉ vì nó có trong cache/go.sum: phải được registry duyệt như dependency trực tiếp. `go mod verify` kiểm tra integrity module cache, không thay thế vulnerability scan hoặc license review.

## Kiểm tra local và CI

```powershell
go run ./tools/dependencycheck
go mod tidy -diff
go mod verify
go test -race -count=1 ./...
go vet ./...
go build ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...
```

Chạy với toolchain trong `.go-version` để khớp CI/Docker. Các target Makefile tương ứng: `deps-check`, `deps-verify`, `vuln`.

Checker đọc AST trên toàn bộ Go source, gồm `_test.go`, generated code, blank/alias/dot imports và file có build tag (không phụ thuộc file có được build trên OS hiện tại hay không). Bỏ qua thư mục ẩn, `vendor`, `node_modules`, `testdata`; các thư mục này không dùng để đặt implementation ứng dụng. Package lạ hoặc Go file lỗi cú pháp làm kiểm tra thất bại. Go compiler tiếp tục kiểm tra import cycle và type compatibility.

CI còn chạy `go mod tidy -diff`, `go mod verify`, test với race detector, build và govulncheck; bật `GOWORK=off`, `GOFLAGS=-mod=readonly`. Vulnerability scan đánh giá code build trên runner Linux hiện tại, không chứng nhận mọi build tag/OS/dependency tùy chọn. Việc bật required status checks trên GitHub là thiết lập riêng của repository.

## Migration hiện tại

`migrations/000001_create_core_schema.sql` là SQL baseline legacy bất biến; bản embed trong `internal/platform/database/legacy.sql` được Goose Go migration v1 thực thi trong transaction. Không sửa file đã áp dụng hoặc chạy baseline thủ công cho database mới.

Goose runner dùng `goose_db_version`, hỗ trợ `up`, `status`, `adopt-legacy`; SQL migration mới nằm trong `internal/platform/database/schema/`. Compose chạy migrator riêng trước runtime. Test bao gồm fresh/adoption, ownership và role không có superuser. Xem [runbook](../migrations/README.md) cho provision, adoption và recovery. Không tự migrate trong API/indexer/worker.

## Nguồn tham khảo

- [Go: managing dependencies](https://go.dev/doc/modules/managing-dependencies)
- [Go vulnerability management](https://go.dev/doc/security/vuln/)
- [Chi](https://github.com/go-chi/chi), [pgx](https://github.com/jackc/pgx), [Validator](https://github.com/go-playground/validator)
- [OpenTelemetry Go](https://opentelemetry.io/docs/languages/go/)
- [Goose SQL annotations](https://pressly.github.io/goose/documentation/annotations/)
