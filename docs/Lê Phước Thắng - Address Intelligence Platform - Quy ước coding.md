# Quy ước coding

## Dependency và ranh giới module

Áp dụng [Quản lý dependency](<Lê Phước Thắng - Address Intelligence Platform - Quản lý dependency.md>) và [`dependency-policy.json`](../dependency-policy.json) cho mọi Go package.

- Application không import infrastructure, transport, driver hoặc framework.
- Module chỉ giao tiếp qua `contract`; không import application/domain/repository implementation của module khác.
- Domain/contract không phụ thuộc database, HTTP, Redis hay telemetry SDK.
- Bootstrap nối implementation bằng constructor injection; platform không phụ thuộc business module.
- Thư viện mới phải có trong registry/policy và được maintainer review; pin version cụ thể, commit `go.mod`/`go.sum`, không tự động nâng toàn bộ dependency.
- Unit test giữ nguyên boundary; test tích hợp nối adapter trong `test/integration` hoặc `test/e2e`.
- Trước khi gửi PR, chạy `go run ./tools/dependencycheck`, `go mod tidy -diff`, `go mod verify`, test/vet/build và vulnerability scan.

## Application Bootstrap

Quy tắc bootstrap: `main` chỉ xử lý entry point/exit code; signal context nằm trong hàm trả lỗi để defer chạy trước `os.Exit`. Wiring và lifecycle thuộc `internal/bootstrap`, dependency được truyền qua constructor. Tài nguyên mở trong `App.Run` phải đăng ký cleanup ngay, đóng theo thứ tự ngược và dùng chung shutdown deadline. Không thay đổi logger global. Xem [Application Bootstrap](<Lê Phước Thắng - Address Intelligence Platform - Application Bootstrap.md>).

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


## Quy tắc đặt tên tài liệu

Quy tắc này áp dụng cho mọi tài liệu dự án mới và khi đổi tên tài liệu hiện có, kể cả tài liệu do công cụ hoặc AI tạo.

- Tài liệu thiết kế, kiến trúc, nghiệp vụ, foundation, kế hoạch và worklog đặt trong `docs/` theo đúng mẫu:

  ```text
  Lê Phước Thắng - Address Intelligence Platform - <Tên tài liệu>.md
  ```

- Giữ nguyên tiền tố, dấu tiếng Việt, khoảng trắng và dấu phân cách ` - `. Tên chủ đề ngắn gọn, mô tả nội dung; dùng tiếng Việt có dấu hoặc thuật ngữ đã thống nhất như `HTTP Server Foundation`.
- Không dùng tên không dấu, snake_case, tên tạm như `final`, `new`, `copy` hoặc gắn ngày/version vào tên file. Version, trạng thái và ngày cập nhật đặt trong nội dung khi tài liệu có quản lý phiên bản.
- Trước khi tạo file, kiểm tra xem chủ đề đã có tài liệu chưa; ưu tiên cập nhật tài liệu hiện có, không tạo bản song song chỉ khác tên.
- Ngoại lệ: `README.md` giữ nguyên tại root hoặc thư mục để hướng dẫn sử dụng/vận hành; `AGENTS.md`, `GEMINI.md`, `SKILL.md` và các tên đặc biệt do công cụ/quy trình yêu cầu giữ tên chuẩn của chúng. File dữ liệu, cấu hình, source code, migration và tài liệu bên thứ ba không áp dụng tiền tố này.
- Liên kết nội bộ dùng đường dẫn tương đối. Với tên có khoảng trắng, dùng `[Nhãn](<đường dẫn>)` hoặc URL-encode đường dẫn; không dùng đường dẫn tuyệt đối trên máy cá nhân hay `file://`.
- Khi đổi tên: cập nhật toàn bộ liên kết/tham chiếu hiện hành, mục lục và bảng tài liệu liên quan; kiểm tra đích liên kết tồn tại. Nhật ký lịch sử có thể giữ tên cũ nếu ghi rõ đó là tên tại thời điểm ghi nhận.
- Trước khi hoàn tất: kiểm tra tên mới đúng mẫu, không có bản trùng, liên kết không hỏng và ghi nhận thay đổi tài liệu đáng kể vào backlog.

Ví dụ:

```text
docs/Lê Phước Thắng - Address Intelligence Platform - Kiến trúc xử lý địa chỉ Việt Nam.md
docs/Lê Phước Thắng - Address Intelligence Platform - Middleware Foundation.md
docker/README.md
```
