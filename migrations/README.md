# Database migrations

Các migration PostgreSQL/PostGIS được đánh số tăng dần, chạy theo thứ tự tên file và bất biến sau khi đã deploy.

Quy tắc:

- Mỗi migration chạy trong một transaction khi PostgreSQL cho phép.
- Constraint nghiệp vụ quan trọng phải nằm trong database.
- Không sửa migration đã được áp dụng; tạo migration mới để thay đổi schema.
- Không hard-delete dữ liệu canonical đã tham gia history hoặc provenance.
- `schema_migrations` ghi nhận migration đã áp dụng.

Apply thủ công trong local container:

```bash
docker exec address-intelligence-platform-postgres-1 \
  psql -v ON_ERROR_STOP=1 -U address_app -d address_intelligence \
  -f /migrations/000001_create_core_schema.sql
```

Migration runner chính thức sẽ được tích hợp vào CI/CD trước khi có môi trường shared; application runtime không tự động migrate khi khởi động.

Dependency Management đã chốt **Goose v3** cho runner tương lai. SQL bootstrap hiện tại chưa có định dạng Goose; không đưa trực tiếp vào runner hoặc sửa migration đã áp dụng. Phải có kế hoạch baseline/adoption và test database mới/cũ trước khi chuyển đổi; xem [quy tắc migration](../docs/dependency-management.md#migration-hiện-tại).
