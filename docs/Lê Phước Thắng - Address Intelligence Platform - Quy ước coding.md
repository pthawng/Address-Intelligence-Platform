# Quy ước coding

## Dependency và ranh giới module

Áp dụng [Quản lý dependency](Quản%20lý%20dependency.md) và [`dependency-policy.json`](../dependency-policy.json) cho mọi Go package.

- Application không import infrastructure, transport, driver hoặc framework.
- Module chỉ giao tiếp qua `contract`; không import application/domain/repository implementation của module khác.
- Domain/contract không phụ thuộc database, HTTP, Redis hay telemetry SDK.
- Bootstrap nối implementation bằng constructor injection; platform không phụ thuộc business module.
- Thư viện mới phải có trong registry/policy và được maintainer review; pin version cụ thể, commit `go.mod`/`go.sum`, không tự động nâng toàn bộ dependency.
- Unit test giữ nguyên boundary; test tích hợp nối adapter trong `test/integration` hoặc `test/e2e`.
- Trước khi gửi PR, chạy `go run ./tools/dependencycheck`, `go mod tidy -diff`, `go mod verify`, test/vet/build và vulnerability scan.

## Application Bootstrap

Quy tắc bootstrap: `main` chỉ xử lý entry point/exit code; signal context nằm trong hàm trả lỗi để defer chạy trước `os.Exit`. Wiring và lifecycle thuộc `internal/bootstrap`, dependency được truyền qua constructor. Tài nguyên mở trong `App.Run` phải đăng ký cleanup ngay, đóng theo thứ tự ngược và dùng chung shutdown deadline. Không thay đổi logger global. Xem [Application Bootstrap](Application%20Bootstrap.md).

## Context Foundation

Context cho I/O và usecase phải là tham số đầu `ctx context.Context`; truyền từ
`r.Context()` tới repository/driver, không tạo root context trong repository.
Không lưu context trong service struct. Hàm thuần không cần context. Xem
[Context Foundation](<Lê Phước Thắng - Address Intelligence Platform - Context Foundation.md>).
`go run ./tools/dependencycheck` kiểm tra thêm quy tắc context trong CI.

## Commit message

Mọi commit của dự án phải theo định dạng:

```text
<type>(<scope>): <description>
```

- `type`: viết thường, thể hiện loại thay đổi: `feat`, `fix`, `refactor`, `docs`, `test`, `ci`, `build`, `chore`, `perf` hoặc `revert`.
- `scope`: bắt buộc, viết thường, chỉ thành phần chính bị tác động, ví dụ `config`, `api`, `indexer`, `worker`, `database`, `search`, `docs` hoặc `ci`.
- `description`: mô tả ngắn gọn, rõ thay đổi; dùng tiếng Anh, bắt đầu bằng động từ, không có dấu chấm cuối.
- Một commit nên tập trung vào một mục đích; test và tài liệu liên quan có thể đi cùng code.
- Nếu cần giải thích thêm lý do hoặc ảnh hưởng, thêm body sau một dòng trống.

Ví dụ:

```text
feat(config): add runtime-specific configuration validation
fix(config): encode database credentials safely
docs(conventions): document commit message format
ci(checks): add Go validation workflow
```
