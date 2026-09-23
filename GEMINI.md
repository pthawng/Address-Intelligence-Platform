# Quy tắc dự án (Project Rules) - Address Intelligence Platform

## 1. Quy tắc ghi nhận Backlog & Worklog
* **Tài liệu Backlog chính:** [`docs/Lê Phước Thắng - Address Intelligence Platform - Backlog công việc.md`](<docs/Lê Phước Thắng - Address Intelligence Platform - Backlog công việc.md>)
* **Quy tắc bắt buộc:** Mỗi khi thực hiện một **task có thay đổi lớn** (Major changes: thay đổi kiến trúc, thêm/sửa database migration, thay đổi cấu hình hạ tầng, refactor lớn hoặc bổ sung API endpoint cốt lõi), BẮT BUỘC phải cập nhật bảng nhật ký công việc vào file backlog.
* **Định dạng ghi log:** 
  - Thời gian: `DD/MM/YYYY HH:mm`
  - Sắp xếp từ **MỚI NHẤT** đến **CŨ NHẤT**
  - Ghi rõ Công việc thực hiện, Chi tiết công việc và Tên dev thực hiện (`Lê Phước Thắng`).

## 2. Quy chuẩn code & cấu trúc
* Đảm bảo tuân thủ Go Standard Layout (`cmd/`, `internal/`, `api/`, `configs/`, `deployments/`, `docs/`, `migrations/`).
* Tuân thủ kiến trúc Clean Architecture & Layered Architecture đã quy định trong tài liệu kiến trúc.


## 3. Quy tắc đặt tên tài liệu

Áp dụng [hướng dẫn chung cho agent](AGENTS.md) và mục **Quy tắc đặt tên tài liệu** trong [Quy ước coding](<docs/Lê Phước Thắng - Address Intelligence Platform - Quy ước coding.md>) mỗi khi tạo hoặc đổi tên tài liệu. Tài liệu dự án trong `docs/` dùng mẫu `Lê Phước Thắng - Address Intelligence Platform - <Tên tài liệu>.md`; các file chuẩn như `README.md` giữ nguyên theo ngoại lệ đã quy định. Kiểm tra tài liệu hiện có trước khi tạo mới và cập nhật các liên kết khi đổi tên.
