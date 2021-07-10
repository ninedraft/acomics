build:
	mkdir -p ./bin
	go build -o ./bin/acomic ./

install:
	go install ./
