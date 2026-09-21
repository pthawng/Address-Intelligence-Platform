# CI workflows

`ci.yml` chạy trên mỗi push/pull request: kiểm tra gofmt, `go vet ./...`, `go test -race -count=1 ./...`, build và validate Compose (bao gồm worker profile).

Go version lấy từ `go.mod`. Job chỉ có quyền đọc repository, không deploy và không dùng production secret. Cache tắt vì dự án chưa có dependency ngoài hoặc `go.sum`.
