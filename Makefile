run: compile
	./build/apollo "./public/Lofi Girl - Snowman.ogg" foo bar

compile:
	go build -o build/apollo src/main.go
