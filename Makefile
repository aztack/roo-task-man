APP=roo-task-man

.PHONY: build run test tidy clean

build:
	go build -tags js_hooks -o $(APP) ./cmd/roo-task-man

run: build
	./$(APP)

test:
	go test ./...

tidy:
	go mod tidy

clean:
	rm -f $(APP)
