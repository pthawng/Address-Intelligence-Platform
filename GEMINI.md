# Quy tắc dự án (Project Rules) - Address Intelligence Platform

## 1. Quy tắc ghi nhận Backlog & Worklog
* **Tài liệu Backlog chính:** [`docs/Lê Phước Thắng - Address Intelligence Platform - Backlog công việc.md`](file:///D:/address-intelligence-platform/docs/L%C3%AA%20Ph%C6%B0%E1%BB%BC%20Th%E1%BA%AFng%20-%20Address%20Intelligence%20Platform%20-%20Backlog%20c%C3%B4ng%20vi%E1%BB%87c.md)
* **Quy tắc bắt buộc:** Mỗi khi thực hiện một **task có thay đổi lớn** (Major changes: thay đổi kiến trúc, thêm/sửa database migration, thay đổi cấu hình hạ tầng, refactor lớn hoặc bổ sung API endpoint cốt lõi), BẮT BUỘC phải cập nhật bảng nhật ký công việc vào file backlog.
* **Định dạng ghi log:** 
  - Thời gian: `DD/MM/YYYY HH:mm`
  - Sắp xếp từ **MỚI NHẤT** đến **CŨ NHẤT**
  - Ghi rõ Công việc thực hiện, Chi tiết công việc và Tên dev thực hiện (`Lê Phước Thắng`).

## 2. Quy chuẩn code & cấu trúc
* Đảm bảo tuân thủ Go Standard Layout (`cmd/`, `internal/`, `api/`, `configs/`, `deployments/`, `docs/`, `migrations/`).
* Tuân thủ kiến trúc Clean Architecture & Layered Architecture đã quy định trong tài liệu kiến trúc.
