mkdir release
NAME=genai-mcp
GOOS=linux GOARCH=amd64 go build -o release/$NAME.linux.amd64
GOOS=darwin GOARCH=amd64 go build -o release/$NAME.darwin.amd64
GOOS=darwin GOARCH=arm64 go build -o release/$NAME.darwin.arm64
GOOS=windows GOARCH=amd64 go build -o release/$NAME.windows.amd64.exe
