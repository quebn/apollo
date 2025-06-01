title: compile
	./build/apollo play "Lofi Girl - Snowman"

dir: compile
	./build/apollo play "./another"

file: compile
	./build/apollo play "./public/Lofi Girl - Snowman.ogg"

run:
	./build/apollo

rebuild: compile
	./build/apollo "Lofi Girl - Snowmans" foo bar

compile:
	go build -o build/apollo src/main.go

testes:
	go build -o test test.go && ./test
