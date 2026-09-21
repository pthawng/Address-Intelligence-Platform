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
