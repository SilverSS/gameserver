client:
	@go build -o bin/client game_client/main.go

client-amd64:
	@GOOS=windows GOARCH=amd64 go build -o bin/client_amd64.exe game_client/main.go

server:
	@go build -o bin/server game_server/main.go

server-amd64:
	@GOOS=windows GOARCH=amd64 go build -o bin/server_amd64.exe game_server/main.go

server-windows:
	@go build -o bin/server_amd64.exe game_server/main.go