# CI workflows

`ci.yml` chạy trên mỗi push/pull request: kiểm tra gofmt, dependency boundaries, module integrity (`tidy -diff`, `verify`), `go vet ./...`, `go test -race -count=1 ./...`, build, validate Compose (bao gồm worker profile) và govulncheck pin `v1.7.0`.

Go toolchain lấy từ `.go-version`, đồng bộ với Dockerfile; `go.mod` là minimum version. Job chỉ có quyền đọc repository, không deploy và không dùng production secret. Cache tắt vì dự án chưa có dependency runtime ngoài hoặc `go.sum`. `GOWORK=off` và `GOFLAGS=-mod=readonly` tránh implicit dependency changes. Xem [Dependency Management](../../docs/Lê%20Phước%20Thắng%20-%20Address%20Intelligence%20Platform%20-%20Quản%20lý%20dependency.md) cho giới hạn và quy tắc review.
