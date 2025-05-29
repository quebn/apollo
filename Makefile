run: compile
	./build/apollo

compile:
	go build -o build/apollo src/main.go
