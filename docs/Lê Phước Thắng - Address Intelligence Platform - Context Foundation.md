# Context Foundation

## Quy ước bắt buộc

- Hàm thực hiện I/O, usecase điều phối I/O, repository port và adapter nhận
  `ctx context.Context` ở tham số đầu tiên. Interface không đặt tên tham số có
  thể dùng `Search(context.Context, string)`.
- Handler giữ chữ ký `net/http`, lấy `r.Context()` và truyền xuống usecase,
  repository rồi driver. Không truyền `*http.Request` vào application/domain.
- Không lưu context trong service/repository struct, không truyền `nil`, không
  dùng context để chứa logger, DB connection, config hoặc tham số nghiệp vụ.
- Hàm thuần như normalize/validate địa chỉ không cần context. Constructor chỉ
  gán dependency không cần context; constructor thực hiện I/O thì cần.
- Khi cần timeout hẹp hơn, dùng `context.WithTimeout(ctx, budget)` và `defer
  cancel()`. Child context không kéo dài deadline của parent. Không tạo timeout
  tùy ý tại mọi layer; operation owner chọn budget theo cấu hình và SLA thực tế.
- Repository không tạo `Background`, `TODO` hay `WithoutCancel`. Dùng API có
  context: pgx `Query(ctx, ...)`, `Exec(ctx, ...)`, `Begin(ctx)`; database/sql
  `QueryContext`, `ExecContext`, `BeginTx`; outbound HTTP `NewRequestWithContext`.
- Trả lỗi cancellation/deadline hoặc wrap bằng `%w`, không đổi thành not-found,
  không retry khi `ctx.Err() != nil`. Giữ driver error chain cho `errors.Is`.
- Vòng lặp dài kiểm tra `ctx.Err()` hoặc select `ctx.Done()`. Cancellation chỉ
  là tín hiệu hợp tác, không giết goroutine và không tự rollback mọi driver.
- Context values chỉ mang metadata request: request ID, trace context, principal
  đã xác thực; dùng private typed key và accessor. Truyền tenant/query/permission
  nghiệp vụ bằng tham số rõ ràng, kiểm tra quyền ở application.

## Luồng triển khai cho module mới

Ví dụ minh họa (chưa tạo module nghiệp vụ hoặc DB adapter giả):

```go
// application: consumer-owned port, không import HTTP hay driver.
type AddressRepository interface {
    Search(ctx context.Context, query string) ([]Address, error)
}

type Service struct { repository AddressRepository }

func (s *Service) Search(ctx context.Context, query string) ([]Address, error) {
    if err := ctx.Err(); err != nil { return nil, err }
    addresses, err := s.repository.Search(ctx, query)
    if err != nil { return nil, fmt.Errorf("search addresses: %w", err) }
    return addresses, nil
}

// transport/http: r.Context() đã có timeout và correlation từ middleware.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) error {
    addresses, err := h.service.Search(r.Context(), r.URL.Query().Get("query"))
    if err != nil { return err }
    return httpserver.JSON(w, r, http.StatusOK, addresses)
}

// infrastructure/postgres: nhận ctx từ port, truyền vào driver.
// rows, err := r.pool.Query(ctx, statement, query)
// Kiểm tra err, defer rows.Close(), đọc rows và kiểm tra rows.Err().
```

Không log query địa chỉ. Log nghiệp vụ dùng logger được inject và
`InfoContext(ctx, ...)` / `ErrorContext(ctx, ...)` để giữ correlation.

## Root context và lifecycle

`cmd/api`, `cmd/worker`, `cmd/indexer` tạo root qua
`signal.NotifyContext(context.Background(), ...)`. Healthcheck CLI cũng nhận root
context và dùng HTTP request có context, bên cạnh client timeout 2 giây.

Hai ngoại lệ `WithoutCancel` hiện có được giữ nguyên trong bootstrap:

1. HTTP base context không hủy ngay khi có SIGTERM: server ngừng nhận request,
   drain request đang chạy; hết shutdown deadline thì cancel request và force close.
2. Cleanup dùng `WithTimeout(WithoutCancel(ctx), ShutdownTimeout)` để giải phóng
   tài nguyên ngay cả khi root đã bị hủy. Deadline chung giới hạn tổng cleanup.

Không dùng những ngoại lệ này để chạy business job sau response. Công việc cần
sống lâu hơn request phải qua queue/outbox. Worker nhận process context, tạo
budget riêng cho từng job, luôn gọi cancel và dừng/wait goroutine khi shutdown.
Việc truyền trace qua queue cần adapter riêng; không serialize context.

## Enforcement và kiểm thử

`go run ./tools/dependencycheck` (đã có trong CI) kiểm tra thêm source production:

- Tham số khai báo trực tiếp `context.Context` phải đứng đầu, tên `ctx` nếu có tên.
  Áp dụng cả method, interface, callback và import alias.
- Chặn field struct có kiểu trực tiếp `context.Context`.
- Chặn `context.TODO`; `Background` chỉ được dùng trong `cmd/`.
- `WithoutCancel` chỉ được dùng trong hai file bootstrap quản lý drain/cleanup.
- Test được phép tạo root context. Source generated/build-tagged vẫn được quét.

Đây là AST guard, không phải phân tích data flow: không chứng minh mọi hàm I/O
đã nhận/truyền context, không phân giải type alias hoặc wrapper tùy ý. Review và
test adapter phải kiểm tra những phần đó; không dùng alias để né quy ước.
`go vet ./...` bổ sung kiểm tra lost cancel trong các trường hợp nó nhận diện.

Test HTTP thật kiểm tra client cancellation truyền tới handler, deadline cha
không bị kéo dài và request ID còn nguyên. Các test middleware/lifecycle hiện có
kiểm tra timeout, late write, graceful drain và forced cancellation. Chưa có DB
adapter thực tế, nên chưa tuyên bố đã kiểm thử cancellation tới PostgreSQL.
Khi thêm adapter phải có integration test hủy truy vấn thật, timeout và cleanup.

Tham khảo: [Go context](https://pkg.go.dev/context),
[Canceling database operations](https://go.dev/doc/database/cancel-operations),
[Contexts and structs](https://go.dev/blog/context-and-structs).
