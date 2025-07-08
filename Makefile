server:
	@go build -o bin/server ./game_server

server-amd64:
	@GOOS=windows GOARCH=amd64 go build -o bin/server_amd64.exe game_server/main.go

server-windows:
	@go build -o bin/server_amd64.exe game_server/main.go