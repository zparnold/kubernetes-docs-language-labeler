build:
	dep ensure
	env GOOS=linux go build -ldflags="-s -w" -o bin/ingress ./webhook-ingress/...
	env GOOS=linux go build -ldflags="-s -w" -o bin/processor processor/main.go

deploy:
	sls deploy

all: build deploy