title: compile
	./build/apollo "Lofi Girl - Snowman"

dir: compile
	./build/apollo "./another"

file: compile
	./build/apollo "./another/Jet Set Radio Future - Birthday Cake.ogg"

run: compile
	./build/apollo

compile:
	go build -o build/apollo src/*.go

test:
	go build -o build/test tests/*.go && ./build/test
