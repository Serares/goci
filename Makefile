build:
	@go build -o ./bin/goci .

test:
	go test -v .

run: build
	./bin/goci