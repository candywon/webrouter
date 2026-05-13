.PHONY: build clean test cross

# 当前平台
build:
	go build -o wr-proxy .

# 交叉编译所有平台
cross:
	GOOS=linux   GOARCH=amd64 go build -o bin/wr-proxy-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -o bin/wr-proxy-linux-arm64 .
	GOOS=darwin  GOARCH=amd64 go build -o bin/wr-proxy-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 go build -o bin/wr-proxy-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -o bin/wr-proxy-windows-amd64.exe .

# 运行（开发模式）
run: build
	./wr-proxy

# 清理
clean:
	rm -f wr-proxy bin/*

# 下载依赖
deps:
	go mod tidy
	go mod download

# 测试
test:
	go test -v ./...
