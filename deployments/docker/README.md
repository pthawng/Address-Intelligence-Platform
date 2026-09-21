# Local containers

Local stack được định nghĩa tại `compose.yaml` ở repository root:

- PostgreSQL 17 + PostGIS 3.5;
- Elasticsearch 9.5.4 single-node;
- API và indexer Go chạy bằng image tối giản, non-root;
- worker bật tùy chọn bằng Compose profile.

Docker resources dùng prefix kebab-case `address-intelligence-platform`: container do Compose sinh theo mẫu `address-intelligence-platform-<service>-1`; network và volume có tên tường minh. Không đặt `container_name` để vẫn hỗ trợ scale và nhiều project instance.

Sao chép `.env.docker.example` thành `.env.docker`, đổi mật khẩu local rồi chạy:

```bash
docker compose --env-file .env.docker up --build -d
docker compose --env-file .env.docker --profile worker up --build -d
docker compose --env-file .env.docker ps
```

Các cổng chỉ bind vào `127.0.0.1` theo mặc định.

Backend nhận các trường `DATABASE_HOST/PORT/NAME/USER/PASSWORD/SSLMODE` riêng; Go encode credential khi tạo URL. `DATABASE_URL` được Compose đặt rỗng để tránh ghi đè các trường này. Password có `$`/`#` nên được bọc nháy đơn trong `.env.docker`; xem [configuration reference](../../configs/README.md).

> [!WARNING]
> Compose này dành cho local development/integration test. Elasticsearch đang tắt security và chạy single-node; không dùng nguyên trạng cho production.
