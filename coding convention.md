# Coding convention

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
