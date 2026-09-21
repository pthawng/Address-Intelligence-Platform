# Application Bootstrap

`internal/bootstrap` là composition root. Package này chọn implementation, nối dependency và sở hữu lifecycle của runtime; không chứa nghiệp vụ module.

## Entry point

`cmd/api`, `cmd/indexer`, `cmd/worker` có `main()` chỉ gọi `run()`, ghi lỗi cuối cùng bằng JSON ra stderr và trả exit code 1. `run()` sở hữu signal context, gọi constructor rồi `App.Run(ctx)`; defer giải phóng signal handler chạy trước khi `main` gọi `os.Exit`.

```go
app, err := bootstrap.NewAPI() // hoặc NewIndexer / NewWorker
if err != nil {
    return err
}
return app.Run(ctx)
```

Constructor đọc/validate config theo runtime và tạo logger riêng, chưa mở socket/connection hay khởi chạy goroutine. Caller không cần `Close()` khi tạo App nhưng chưa Run. `RunAPI/RunIndexer/RunWorker` vẫn có sẵn như hàm tiện ích tương đương constructor + Run.

## Cấu trúc và thứ tự

| File | Trách nhiệm |
|---|---|
| `app.go` | Config/logger, App một lần chạy, cleanup chung |
| `api.go` | HTTP listener/server, timeout, graceful drain |
| `indexer.go` | Composition root của indexer; hiện chờ cancellation |
| `worker.go` | Composition root của worker; hiện chờ cancellation |
| `resources.go` | Stack cleanup theo thứ tự ngược, tổng hợp lỗi |
| `health.go` | Health routes của skeleton |

Luồng đang hoạt động:

```text
main → run (signal context) → NewAPI (config → logger)
                           → App.Run (listen → serve → wait)
                           → cleanup (drain HTTP → close listener)
```

Khi tích hợp adapter, mở PostgreSQL/Elasticsearch trong runner trước khi tạo repositories → services → handlers → HTTP listener. Đăng ký cleanup ngay sau mỗi lần mở thành công. Nếu bước sau thất bại, `App.Run` vẫn thu dọn tất cả tài nguyên đã đăng ký.

Redis không thuộc MVP. Repositories/services/handlers nghiệp vụ và database/search clients chưa được tích hợp; bước này không tự mở các kết nối giả hoặc thêm driver chưa sử dụng.

## Quy tắc lifecycle

- Mỗi App chỉ được Run một lần, kể cả khi lần đầu thất bại hoặc context đã canceled. Lần gọi tiếp theo/concurrent trả lỗi. Không copy App sau khi sử dụng.
- Config sai thất bại tại constructor. Port bị chiếm thất bại tại Run; chỉ log HTTP đang lắng nghe sau khi bind thành công.
- Context đã canceled trước Run không mở tài nguyên. Signal cancellation là dừng bình thường; lỗi startup, Serve hoặc cleanup vẫn được trả về.
- Cleanup chạy LIFO: dừng HTTP trước khi đóng dependency đã mở trước đó. `errors.Join` giữ cả lỗi gốc và các lỗi cleanup; một closer lỗi không ngăn closer tiếp theo chạy. Stack cleanup không chạy lặp.
- Toàn bộ cleanup dùng **một** deadline `SHUTDOWN_TIMEOUT`, tách khỏi cancellation của runtime. Các closer phải tuân thủ context và có thể được gọi khi deadline đã hết; không tạo goroutine bỏ mặc cleanup đang chạy.
- HTTP ngừng nhận kết nối mới, chờ request đang chạy. Request context không bị hủy ngay khi nhận signal. Nếu drain quá hạn, cancel request context, gọi `Server.Close`, trả lỗi timeout và tiếp tục cleanup.
- Không thể cưỡng bức kết thúc goroutine handler bỏ qua context. Handler phải truyền context xuống tác vụ phụ thuộc. WebSocket/hijacked connection chưa được triển khai; khi thêm phải đăng ký cơ chế drain/close riêng.
- Logger được inject theo runtime với `service` và `environment`; không thay đổi `slog.Default`. Lỗi cuối cùng do entry point log một lần. Không log toàn bộ config/credential.
- Worker/indexer chưa có task loop. Khi thêm, phải hỗ trợ dừng nhận việc và kết thúc tác vụ có giới hạn trước khi giải phóng dependency; không coi skeleton hiện tại là job lifecycle hoàn chỉnh.

`/health/live` và `/health/ready` vẫn trả 200 tĩnh cho skeleton. Chỉ khi có adapter mới bổ sung dependency probes/readiness; việc HTTP bind thành công không chứng minh database/search sẵn sàng.

## Kiểm thử

Test dùng TCP loopback với port tự cấp để kiểm tra health routes, request đang chạy khi drain, force-close sau timeout, lỗi listener, startup rollback theo thứ tự ngược và lỗi cleanup. Chạy `go test -race ./internal/bootstrap` hoặc toàn bộ CI.

Tham khảo hành vi HTTP: [Go Server.Shutdown/Close](https://pkg.go.dev/net/http#Server.Shutdown), [context.WithoutCancel](https://pkg.go.dev/context#WithoutCancel).
