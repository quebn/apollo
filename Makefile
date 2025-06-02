title: compile
	./build/apollo play "Lofi Girl - Snowman"

dir: compile
	./build/apollo play "./another"

file: compile
	./build/apollo play "./public/Lofi Girl - Snowman.ogg"

run: compile
	./build/apollo play "public"

compile:
	go build -o build/apollo src/main.go

testes:
	go build -o test test.go && ./test
