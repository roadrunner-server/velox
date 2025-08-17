test:
	go test -v -race ./...

regenerate:
	rm -rf ./gen
	buf generate
	buf format -w