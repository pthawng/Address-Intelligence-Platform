# ADDRESS INTELLIGENCE PLATFORM
## BACKLOG VÀ NHẬT KÝ CÔNG VIỆC (WORKLOG)

| Thông tin | Nội dung |
|---|---|
| **Tên dự án** | Address Intelligence Platform |
| **Tên tài liệu** | Backlog và Nhật ký công việc |
| **Developer** | Lê Phước Thắng |
| **Phiên bản** | `v2.4.3` |
| **Trạng thái** | Active Baseline MVP |
| **Ngày cập nhật** | 23/09/2026 23:59 |

> **Quy ước ghi chép nhật ký công việc:**  
> - **Sắp xếp:** Tất cả các công việc được sắp xếp theo thời gian giảm dần (từ **MỚI NHẤT** đến **CŨ NHẤT**).  
> - **Định dạng thời gian:** `DD/MM/YYYY HH:mm` (Ngày/Tháng/Năm Giờ:Phút).  
> - **Phạm vi công việc:** Lập kế hoạch, Phân tích thiết kế, Lập trình core, Cơ sở dữ liệu, Docker & DevOps, Testing và Tài liệu hóa.

---

# 1. Bảng Nhật ký công việc & Backlog chi tiết (Mới đến Cũ)

| STT | Thời gian (ngày giờ, phút) | Công việc thực hiện | Chi tiết công việc | Dev thực hiện | Trạng thái |
|:---:|:---:|---|---|:---:|:---:|
| **27** | `23/09/2026 23:59` | Nạp snapshot nguồn theo mô hình place-centric | Thêm Goose v3 `source_records` và importer đọc sáu file SQL legacy theo checksum, không thực thi INSERT vào `admin_units`. Đã backup trước/sau, restore thử cả hai; nạp 186.581 dòng vào DB local trong một transaction: 15.290 đơn vị hành chính cấp 1–3 có external reference/outbox; 171.284 dòng LEVEL_4 chờ đối soát và 7 dòng mã/tên rỗng giữ trạng thái INVALID. Chạy lại no-op, test tích hợp PostGIS, kiểm tra quyền runtime và API readiness 200. Chưa nạp Place từ LEVEL_4. | Lê Phước Thắng | `Hoàn thành trong phạm vi nêu rõ` |
| **26** | `23/09/2026 23:04` | Hoàn tất DB-01 trên database local | Tạo pg_dump custom, snapshot nội dung và checksum; restore và adopt thử trên PostGIS tạm, đối chiếu dữ liệu, rồi provision `address_migrator`/`address_runtime`, chuyển ownership theo allowlist và adopt legacy v1 → Goose v2 trên database local. Xoay credential DBA mẫu sang mật khẩu mạnh; runtime bị chặn DDL/ghi ledger. Invariant SQL, API readiness 200 và kết nối của API/indexer/worker bằng runtime role đã kiểm chứng sau khi Compose recreate. Backup lưu trong `data/backups/` (Git bỏ qua). | Lê Phước Thắng | `Hoàn thành` |
| **25** | `23/09/2026 00:25` | Chuẩn hóa tên và quy tắc tài liệu | Đổi tên HLD xử lý địa chỉ Việt Nam và Middleware Foundation theo tiền tố chung; sửa liên kết cũ, cập nhật bảng tham chiếu và README. Bổ sung quy tắc đặt tên, ngoại lệ README/tool files trong Quy ước coding; đưa hướng dẫn vào AGENTS.md và GEMINI.md để áp dụng cho tài liệu mới. | Lê Phước Thắng | `Hoàn thành` |
| **24** | `23/09/2026 00:05` | Commit và push foundation lên main | Đã push 12 commit từ `9c132b1` đến `fea1524` lên `origin/main`, đúng định dạng `type(scope): description`; kiểm tra từng commit theo tổng dòng thêm + xóa, lớn nhất 966 dòng, không vượt 999. | Lê Phước Thắng | `Hoàn thành` |
| **23** | `23/09/2026 00:05` | Vá dependency và kiểm tra trước khi push | Nâng `golang.org/x/text` lên `v0.39.0` để xử lý `GO-2026-5970`; dependency liên quan `golang.org/x/sync` lên `v0.21.0`. Test race, vet, build, module integrity, dependency boundary và govulncheck pass; scanner không phát hiện lỗ hổng ảnh hưởng đường gọi hiện tại. | Lê Phước Thắng | `Hoàn thành` |
| **22** | `23/09/2026 00:05` | Kiểm chứng database foundation trên PostGIS thật | Test fresh migration, adoption, drift, rollback, concurrent migrators, hierarchy serializable, migrator không có superuser, runtime bị chặn DDL/ledger write, tạo role muộn và reconcile quyền. Smoke test provision bằng psql, CLI up/status bằng migrator và invariant bằng runtime đều pass; container test đã dọn. | Lê Phước Thắng | `Hoàn thành` |
| **21** | `23/09/2026 00:05` | Hoàn thiện Database & Migration Foundation | Tích hợp pgxpool, timeout/lifecycle, readiness kiểm tra schema/quyền và tên kết nối theo service; Goose runner riêng có session lock, baseline embed, adoption kiểm tra drift, credential migration riêng. Thêm script chuyển ownership giới hạn theo object dự án, reconcile quyền chạy lặp lại và lỗi migration an toàn có version/SQLSTATE. Đồng bộ runbook provision/adoption/recovery; giữ chính sách exact schema version và maintenance window khi nâng schema. | Lê Phước Thắng | `Hoàn thành trong code` |
| **20** | `23/09/2026 00:05` | Hoàn thiện Docker development và thứ tự khởi động | Tập trung Dockerfile/Compose/environment mẫu vào docker/, thêm dev hot reload bằng Air, runtime image riêng và quyền container hạn chế. Compose chạy migrator một lần, API/indexer/worker chỉ khởi động sau migration thành công; validate cả dev/runtime Compose pass. | Lê Phước Thắng | `Hoàn thành trong code` |
| **19** | `23/09/2026 00:05` | Tích hợp HTTP middleware và telemetry | Bổ sung recovery, CORS, timeout, rate limiting, authentication/authorization contracts, metrics và OpenTelemetry tracing; nối HTTP lifecycle/health vào bootstrap, có test middleware/cancellation. Authenticator nghiệp vụ và collector triển khai thực tế chưa thuộc phạm vi hoàn tất này. | Lê Phước Thắng | `Hoàn thành foundation` |
| **18** | `23/09/2026 00:05` | Hoàn thiện Error, HTTP Server & Standard API Response Foundation | Thêm application error contracts, mapping lỗi HTTP, router/server, JSON decoding giới hạn kích thước, success/error envelope, pagination và quy ước timestamp; bổ sung test và tài liệu theo ranh giới transport/application/domain. | Lê Phước Thắng | `Hoàn thành` |
| **17** | `22/09/2026 22:54` | Thiết lập Context Foundation | Chuẩn hóa context-first cho HTTP server, truyền context vào healthcheck CLI; thêm AST guard trong dependencycheck và test client cancellation/deadline/correlation; quy định flow handler-usecase-repository-driver và ngoại lệ graceful shutdown. | Lê Phước Thắng | `Hoàn thành` |
| **16** | `22/09/2026 22:30` | Chuẩn hóa toàn bộ tên tài liệu dự án | Đổi tên các file tài liệu trong `docs/` (`api-responses.md`, `errors.md`, `http-server.md`, `logging.md`) sang quy ước `Lê Phước Thắng - Address Intelligence Platform - <Tên tài liệu>.md`, cập nhật toàn bộ các liên kết markdown trong `README.md`, `.github/workflows/README.md` và các tài liệu liên quan. | Lê Phước Thắng | `Hoàn thành` |
| **11** | `22/09/2026 02:10` | Chuẩn hóa tên tài liệu dự án | Đổi các tài liệu mới sang quy ước `Lê Phước Thắng - Address Intelligence Platform - <Tên tài liệu>.md`, đưa về thư mục `docs/`, cập nhật toàn bộ liên kết và đồng bộ tiêu đề tài liệu. Giữ `dependency-policy.json` ở root vì đây là policy máy đọc. | Lê Phước Thắng | `Hoàn thành` |
| **12** | `22/09/2026 01:55` | Hoàn thiện Application Bootstrap & Runtime Lifecycle | Tách `NewAPI`, `NewIndexer`, `NewWorker` và `App.Run(ctx)` trong `internal/bootstrap/`; thêm resource stack cleanup LIFO, graceful HTTP drain, timeout force-close, logger injection theo runtime và test startup/shutdown/error lifecycle. Giữ database/search adapter và task loop ở trạng thái chưa tích hợp. | Lê Phước Thắng | `Hoàn thành` |
| **13** | `22/09/2026 01:20` | Thiết lập Dependency Management & Import Boundary | Chốt registry cho HTTP, PostgreSQL, Elasticsearch, Redis, logger, validation, telemetry, migration và testing; thêm `dependency-policy.json` cùng `tools/dependencycheck` để chặn import infrastructure/repository chéo module, driver trong domain/application và dependency chưa được duyệt. Bổ sung CI module integrity, race test, build và govulncheck. | Lê Phước Thắng | `Hoàn thành` |
| **15** | `22/09/2026 01:18` | Hoàn thiện Logging Foundation | Chuẩn hóa JSON logger với service/environment/version, HTTP access log và request correlation; bổ sung điểm tích hợp trace_id, quy tắc bảo vệ dữ liệu và tests. | Lê Phước Thắng | `Hoàn thành` |
| **14** | `22/09/2026 00:40` | Củng cố Configuration Foundation | Thêm cấu hình theo runtime, PostgreSQL split settings với encode credential an toàn, kiểm tra dependency bắt buộc theo môi trường, pin Go `1.25.13`, cập nhật Compose và test validation/credential/lifecycle; xử lý các lỗ hổng standard library được govulncheck phát hiện ở Go `1.25.12`. | Lê Phước Thắng | `Hoàn thành` |
| **01** | `22/09/2026 00:05` | Khởi tạo tài liệu Backlog công việc | Xây dựng file quản lý Backlog và Worklog chi tiết theo chuẩn định dạng tài liệu của dự án (`Lê Phước Thắng - Address Intelligence Platform - Backlog công việc.md`). Tổng hợp lại toàn bộ lộ trình, tiến độ và công việc từ trước tới nay. | Lê Phước Thắng | `Hoàn thành` |
| **02** | `22/09/2026 00:00` | Chuẩn hóa cấu hình Docker & Environment | Cập nhật file `.env.example`, `.env.docker.example` phục vụ việc khởi chạy ứng dụng trên môi trường Containerization. Đồng bộ cấu hình môi trường local và Docker Compose. | Lê Phước Thắng | `Hoàn thành` |
| **03** | `21/09/2026 23:45` | Bổ sung tài liệu Kiến trúc kỹ thuật & Cấu trúc dự án | Biên soạn file `Lê Phước Thắng - Address Intelligence Platform - Kiến trúc kỹ thuật và cấu trúc dự án.md`. Mô hình hóa Layered Architecture, Clean Code patterns, gói dữ liệu và luồng xử lý chính. | Lê Phước Thắng | `Hoàn thành` |
| **04** | `20/09/2026 23:06` | Hoàn thiện tài liệu Thiết kế Cơ sở Dữ liệu | Biên soạn file `Lê Phước Thắng - Address Intelligence Platform - Thiết kế cơ sở dữ liệu.md`. Định nghĩa chi tiết bảng dữ liệu, khóa chính/ngoại, index PostgreSQL/PostGIS, quản lý phiên bản dữ liệu lịch sử địa chính. | Lê Phước Thắng | `Hoàn thành` |
| **05** | `20/09/2026 22:22` | Thiết lập Dockerfile & Makefile | Cấu hình Dockerfile multi-stage build cho Go web server, Makefile phục vụ các tác vụ tự động hóa (`make build`, `make run`, `make test`, `make migrate-up`, `make clean`). | Lê Phước Thắng | `Hoàn thành` |
| **06** | `20/09/2026 21:25` | Biên soạn tài liệu Tổng quan dự án (Baseline MVP) | Biên soạn file `Lê Phước Thắng - Address Intelligence Platform - Tổng quan dự án.md`. Định nghĩa mục tiêu, bài toán xử lý địa chỉ Việt Nam (tên cũ, tên mới, viết tắt, địa chính thay đổi theo thời gian). | Lê Phước Thắng | `Hoàn thành` |
| **07** | `20/09/2026 21:01` | Khởi tạo cấu trúc Database Migration script | Tạo thư mục `migrations/` chứa các file SQL schema ban đầu cho PostGIS, bảng administrative units (tỉnh/thành, quận/huyện, xã/phường), địa chỉ chi tiết và tọa độ không gian. | Lê Phước Thắng | `Hoàn thành` |
| **08** | `20/09/2026 19:40` | Xây dựng Module Configuration & Infrastructure Foundation | Thêm module đọc cấu hình môi trường trong `internal/config/`, khởi tạo struct cấu hình cho Database Postgres, Redis cache, HTTP Server và Logger. | Lê Phước Thắng | `Hoàn thành` |
| **09** | `20/09/2026 19:39` | Tổ chức Cấu trúc Thư mục Dự án Go Standard | Khởi tạo cấu trúc dự án theo chuẩn Go Standard Layout: `cmd/`, `internal/`, `api/`, `configs/`, `deployments/`, `docs/`, `scripts/`, `test/`. | Lê Phước Thắng | `Hoàn thành` |
| **10** | `20/09/2026 19:38` | Khởi tạo Repository & Go Module Baseline | Tạo git repository, file `.gitignore`, `.dockerignore`, khởi chạy `go mod init address-intelligence-platform`. | Lê Phước Thắng | `Hoàn thành` |

---

# 2. Danh sách Backlog nhiệm vụ sắp tới (Upcoming Tasks)

| STT / Mã Task | Thời gian (ngày giờ, phút) | Công việc thực hiện | Chi tiết công việc | Dev thực hiện | Trạng thái |
|:---:|:---:|---|---|:---:|:---:|
| **DB-01 / P0** | `23/09/2026 23:04` | Áp dụng setup vào database local | Đã hoàn tất backup/restore thử, provision role, chuyển ownership, adopt schema v2, reconcile quyền và xác nhận readiness/invariant. Xem nhật ký mục 26 và migration runbook. | Lê Phước Thắng | `Hoàn thành` |
| **DATA-01 / P1** | `23/09/2026 00:13` | Hoàn thiện persistence cho luồng nghiệp vụ đầu tiên | Thiết lập sqlc, query và repository PostgreSQL theo từng module; domain/application giữ interface riêng, geometry PostGIS được map tại infrastructure. Chốt transaction boundary và retry có giới hạn cho lỗi serialization/deadlock; không đặt side effect bên ngoài DB trong callback retry. | Lê Phước Thắng | `Chưa bắt đầu` |
| **BIZ-01 / P1** | `23/09/2026 00:13` | Triển khai nghiệp vụ Data Source | Chốt domain rules, use case tạo/cập nhật/truy vấn nguồn dữ liệu, repository và API; kiểm thử validation, trùng source code và provenance phục vụ Place. | Lê Phước Thắng | `Chưa bắt đầu` |
| **DATA-02 / P1** | `23/09/2026 23:59` | Đối soát nguồn LEVEL_4 trước khi nạp Place | Phân loại road/POI có bằng chứng, giải quyết mã trùng và 7 dòng rỗng; tìm mapping mã cha GHTK sang administrative unit hiện hành. Chỉ liên kết Place sau khi đối soát, giữ raw record chưa đủ căn cứ ở trạng thái unresolved và ghi canonical + outbox nhất quán. | Lê Phước Thắng | `Chưa bắt đầu` |
| **BIZ-02 / P1** | `23/09/2026 00:13` | Triển khai Place và transactional outbox | Xây dựng luồng tạo/cập nhật Place xuyên suốt domain → use case → repository → API; kiểm tra hierarchy/lifecycle, ghi canonical data và outbox event trong cùng transaction, xác định revision/idempotency và chứng minh rollback không để lại dữ liệu/event lệch nhau. | Lê Phước Thắng | `Chưa bắt đầu` |
| **IDX-01 / P1** | `23/09/2026 00:13` | Triển khai outbox indexer sang Elasticsearch | Hoàn thiện polling/claim, retry/backoff, xử lý lock hết hạn, idempotency, chống stale revision, shutdown và khả năng reindex. Indexer hiện mới là skeleton, chưa xử lý event thực tế. | Lê Phước Thắng | `Chưa bắt đầu` |
| **API-01 / P2** | `23/09/2026 00:13` | Triển khai search và autocomplete MVP | Chốt mapping/query Elasticsearch, chuẩn hóa tên và alias theo yêu cầu MVP; xây dựng application contracts, API và response, có test kết quả và eventual consistency sau cập nhật Place. | Lê Phước Thắng | `Chưa bắt đầu` |
| **QA-01 / P2** | `23/09/2026 00:13` | Nghiệm thu luồng nghiệp vụ đầu tiên | Kiểm thử E2E Data Source → Place → outbox → Elasticsearch → search; bao phủ lỗi/retry và dữ liệu địa chỉ Việt Nam đại diện. Xác định yêu cầu auth, vận hành backup/restore và tiêu chí trước production; chưa coi foundation pass là production-ready. | Lê Phước Thắng | `Chưa bắt đầu` |

---

# 3. Tổng kết tiến độ

- **Tổng số hạng mục đã ghi nhận hoàn thành:** 27, trong đó các hạng mục foundation được đánh dấu rõ phạm vi code và kiểm chứng.
- **Tổng số đầu việc còn lại được lập trong đợt cập nhật này:** 7; đây là backlog ưu tiên để bắt đầu nghiệp vụ, không phải toàn bộ phạm vi MVP.
- **Trạng thái hiện tại:** Dữ liệu hành chính từ snapshot đã vào canonical cùng provenance/outbox. Chưa có luồng nghiệp vụ hoàn chỉnh chạy xuyên suốt; sqlc/repository, Data Source/Place use case, đối soát LEVEL_4, outbox processor và search implementation còn ở backlog.
- **Trạng thái triển khai database:** Database local ở Goose v3 sau backup và restore thử; 186.581 dòng nguồn đã nạp, gồm 15.290 đơn vị hành chính canonical, 171.284 dòng LEVEL_4 unresolved và 7 dòng invalid. API/indexer/worker kết nối bằng `address_runtime`; backup local tại `data/backups/source-import-20260923T1652Z/` được Git bỏ qua và cần lưu theo chính sách vận hành trước khi dùng dữ liệu quan trọng.
- **Chính sách triển khai schema:** Binary yêu cầu đúng schema version; nâng schema cần maintenance window. Rolling deployment với nhiều schema version chưa được hỗ trợ.
- **Thứ tự ưu tiên:** DATA-01 → BIZ-01 → DATA-02 → BIZ-02 → IDX-01 → API-01; chuẩn bị dữ liệu/tiêu chí QA-01 trong quá trình phát triển và nghiệm thu khi luồng hoàn chỉnh.
- **Bằng chứng phát hành:** 12 commit foundation đã push lên main, kết thúc tại [fea1524](https://github.com/pthawng/Address-Intelligence-Platform/commit/fea1524f2f4c5446986278e7de40f48450690717). Bản cập nhật backlog này được ghi nhận sau đợt push đó.
- **Tài liệu vận hành:** [Database/migration runbook](../migrations/README.md), [Docker guide](../docker/README.md), [Integration tests](../test/integration/README.md).

> Các mục 18–24 sử dụng thời gian ghi nhận commit ngày 23/09/2026; công việc đã được thực hiện và kiểm chứng trong phiên làm việc trước đó. Các mục lịch sử giữ nội dung tại thời điểm ghi nhận; đường dẫn hoặc trạng thái cũ được thay thế bởi các mục mới và runbook hiện hành.
