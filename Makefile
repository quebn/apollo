title: compile
	./build/apollo "Lofi Girl - Snowman"

dir: compile
	./build/apollo "./another"

file: compile
	./build/apollo "./public/Lofi Girl - Snowman.ogg"

run:
	./build/apollo

rebuild: compile
	./build/apollo "Lofi Girl - Snowmans" foo bar

compile:
	go build -o build/apollo src/main.go
