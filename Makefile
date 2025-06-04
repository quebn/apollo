title: compile
	./build/apollo "Lofi Girl - Snowman"

dir: compile
	./build/apollo "./another"

file: compile
	./build/apollo "./public/Lofi Girl - Snowman.ogg"

run: compile
	./build/apollo

compile:
	go build -o build/apollo src/main.go

testes: compile
	./build/apollo another
