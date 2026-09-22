# Configuration

Runtime configuration đi qua environment variables. Không lưu secret hoặc production connection string trong thư mục này.

Nguồn triển khai: `internal/platform/config/`. API, indexer và worker gọi `config.LoadFor()` với runtime tương ứng trước khi khởi tạo. `Load()` vẫn hỗ trợ đọc/validate tất cả nhóm cấu hình. Thứ tự áp dụng: biến chính có giá trị → alias có giá trị (nếu có) → mặc định. Biến rỗng được xem như chưa thiết lập; giá trị có khoảng trắng không tự động được trim.

| Biến | Mặc định | Quy tắc / sử dụng |
|---|---|---|
| `APP_ENV` | `development` | `development`, `test`, `staging`, `production`; phân biệt hoa thường |
| `HTTP_ADDRESS` | `:8080` | `host:port`, port 1–65535; IPv6 dùng `[::1]:8080`; API sử dụng |
| `DATABASE_URL` | rỗng | URL `postgres://` hoặc `postgresql://`, có host, port hợp lệ nếu khai báo; ưu tiên hơn các trường `DATABASE_*` riêng |
| `DATABASE_HOST` | rỗng | Hostname hoặc IP không kèm port; IPv6 không có ngoặc vuông |
| `DATABASE_PORT` | `5432` | Port 1–65535 khi dùng cấu hình tách trường |
| `DATABASE_NAME` | rỗng | Bắt buộc khi dùng cấu hình tách trường |
| `DATABASE_USER` | rỗng | Bắt buộc khi dùng cấu hình tách trường |
| `DATABASE_PASSWORD` | rỗng | Giá trị thô, không encode trước; không tự trim |
| `DATABASE_SSLMODE` | `verify-full` | `disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full`; áp dụng khi dùng cấu hình tách trường |
| `ELASTICSEARCH_URL` | `http://localhost:9200` ở development/test | API/indexer bắt buộc khai báo URL HTTP/HTTPS ở staging/production; worker bỏ qua |
| `LOG_LEVEL` | `debug` ở development, `info` ở môi trường khác | `debug`, `info`, `warn`, `error`; không phân biệt hoa thường; structured JSON ra stdout |
| `OUTBOX_POLL_INTERVAL` | `1s` | Duration dương; hiện indexer chỉ đọc và ghi log giá trị |
| `SHUTDOWN_TIMEOUT` | `10s` | Duration dương; deadline chung cho HTTP drain và cleanup tài nguyên của App |
| `HTTP_READ_HEADER_TIMEOUT` | `5s` | Duration dương; thời gian đọc HTTP headers |
| `HTTP_READ_TIMEOUT` | `10s` | Duration dương; thời gian đọc toàn bộ HTTP request |
| `HTTP_WRITE_TIMEOUT` | `15s` | Duration dương; thời gian ghi HTTP response |
| `HTTP_IDLE_TIMEOUT` | `60s` | Duration dương; thời gian chờ request tiếp theo trên kết nối keep-alive |
| `HTTP_MAX_HEADER_BYTES` | `32768` (32 KiB) | Positive integer; API request header limit; worker/indexer ignore |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | rỗng | Nếu có: URL HTTP/HTTPS hợp lệ; mới đọc/validate, chưa khởi tạo exporter |

Các duration dùng cú pháp Go như `500ms`, `1s`, `2m`; từ chối số âm, 0, sai định dạng và overflow. Các URL không chấp nhận fragment. Kiểm tra URL chỉ xác nhận cú pháp, không xác nhận quyền truy cập, TLS hay kết nối dịch vụ.

Alias tương thích tài liệu cũ: `HTTP_PORT=9090` tương đương `HTTP_ADDRESS=:9090`; `SEARCH_URL` tương đương `ELASTICSEARCH_URL`. Khi cả hai có giá trị, tên chính được ưu tiên. Cấu hình mới nên dùng tên chính.

Load thất bại trả về lỗi ghi tên biến, không đưa giá trị thô vào thông báo lỗi; entry point thoát với mã 1. Không log toàn bộ `Config` vì connection string có thể chứa secret.

| Nhóm | API | Indexer | Worker |
|---|---|---|---|
| Environment, log, database, shutdown, OTLP | Đọc/validate | Đọc/validate | Đọc/validate |
| HTTP address và timeout | Đọc/validate | Bỏ qua | Bỏ qua |
| Search URL | Đọc/validate | Đọc/validate | Bỏ qua |
| Outbox poll interval | Bỏ qua | Đọc/validate | Bỏ qua |

Nhóm bị bỏ qua có giá trị zero trong `Config`; biến sai thuộc nhóm đó không chặn runtime. Shutdown dùng cho cleanup chung của App; background runtime chưa có task loop. OTLP mới đọc/validate, exporter chưa được tích hợp.

Ở staging/production, mọi runtime cần database cấu hình bằng `DATABASE_URL` hoặc các trường riêng. Nếu bất kỳ trường riêng nào có giá trị, `DATABASE_HOST`, `DATABASE_NAME`, `DATABASE_USER` phải được cung cấp đầy đủ. Code tạo URL bằng `net/url`, encode username/password an toàn. Khi `DATABASE_URL` có giá trị, các trường riêng bị bỏ qua; URL do người dùng cung cấp phải encode credential sẵn. SSL mode của URL tường minh do URL đó quyết định, không bị `DATABASE_SSLMODE` ghi đè.

Deployment staging/production phải đặt `APP_ENV` rõ ràng. Ứng dụng không thể tự nhận diện môi trường triển khai: bỏ quên biến này vẫn dùng mặc định development. Loader mới kiểm tra cú pháp; validation option riêng và kết nối PostgreSQL sẽ được bổ sung cùng driver/adapter.

## Chạy local

Ứng dụng **không tự đọc `.env`**. `.env.example` là mẫu tham khảo; shell hoặc công cụ triển khai cần đưa biến vào process environment. Chạy skeleton mặc định bằng `go run ./cmd/api`, hoặc thiết lập trực tiếp trong PowerShell:

```powershell
$env:APP_ENV = 'development'
$env:HTTP_ADDRESS = ':8080'
$env:LOG_LEVEL = 'debug'
go run ./cmd/api
```

Ở development/test, database có thể để trống để chạy skeleton. Nếu dùng PostgreSQL local không có TLS, đặt `DATABASE_SSLMODE=disable` khi dùng trường riêng (hoặc `sslmode=disable` trong URL). Hiện chưa có adapter kết nối PostgreSQL/Elasticsearch, vòng lặp xử lý outbox hay telemetry exporter. `/health/ready` hiện trả cùng kết quả tĩnh với `/health/live`, chưa kiểm tra dependency.

## Docker Compose

```powershell
Copy-Item docker/.env.example docker/.env
docker compose --env-file docker/.env -f docker/compose.yaml -f docker/compose.dev.yaml config --quiet
docker compose --env-file docker/.env -f docker/compose.yaml -f docker/compose.dev.yaml up --build -d
```

`docker/.env` được Compose đọc; đây không phải tính năng dotenv của ứng dụng. `APP_ENV`, `OUTBOX_POLL_INTERVAL`, `SHUTDOWN_TIMEOUT` có mặc định trong Compose. Các biến log, HTTP timeout và OTLP được truyền qua `env_file` nếu khai báo; nếu thiếu, Go dùng mặc định nêu trên.

Compose đặt `DATABASE_URL` rỗng để dùng trường riêng: host `postgres`, port `5432`, name/user/password lấy từ `POSTGRES_DB`/`POSTGRES_USER`/`POSTGRES_PASSWORD`, SSL mode `disable` cho local. Password không còn được ghép trực tiếp vào URL. Với password chứa `$` hoặc `#` trong `docker/.env`, dùng dấu nháy đơn để giữ nguyên giá trị, ví dụ `POSTGRES_PASSWORD='local$p@ss/#%'` (chỉ minh họa).

Compose đặt `ELASTICSEARCH_URL=http://elasticsearch:9200`. API luôn lắng nghe `HTTP_ADDRESS=:8080` bên trong container; `API_PORT` chỉ đổi port công bố trên máy host. Các giá trị trong `environment` của Compose ưu tiên hơn `env_file`.

`HEALTHCHECK_URL` chỉ do lệnh `api healthcheck` đọc, mặc định `http://127.0.0.1:8080/health/live`, HTTP client timeout cố định `2s`; không thuộc `config.Load()`. Khi chạy local với HTTP port khác, phải đặt biến này nếu dùng lệnh healthcheck.

`stop_grace_period` của backend hiện là `15s`; nếu tăng `SHUTDOWN_TIMEOUT`, cần tăng thời gian này để container đủ thời gian shutdown. Compose hiện phục vụ local development; không phải cấu hình production.


## Middleware configuration

| Variable | Default | Scope |
| --- | --- | --- |
| HTTP_REQUEST_TIMEOUT | 8s | API; positive and below HTTP_WRITE_TIMEOUT |
| HTTP_CORS_ORIGINS | http://localhost:3000 in development/test, empty otherwise | API; comma-separated exact origins |
| HTTP_RATE_PER_SECOND | 100 | API; positive integer |
| HTTP_RATE_BURST | 200 | API; positive integer |

OTEL_EXPORTER_OTLP_ENDPOINT now enables the API OTLP/HTTP trace exporter when
nonempty. Worker/indexer still do not export telemetry. No external collector is
configured by default. See [Middleware Foundation](../docs/middleware.md).

## Migration credentials

`cmd/migrate` uses only `MIGRATION_DATABASE_URL` or split `MIGRATION_DATABASE_HOST`,
`PORT`, `NAME`, `USER`, `PASSWORD`, `SSLMODE` (all with the `MIGRATION_DATABASE_`
prefix). URL takes precedence; split passwords remain raw and SSL defaults to
`verify-full`. The CLI does not read .env files. Compose supplies these fields using
`MIGRATION_PASSWORD`, separately from runtime credentials. See the
[database runbook](../migrations/README.md) for provision/adoption/recovery.
