
build:
	GOOS=linux GOARCH=amd64 go build -o bootstrap main.go
	mkdir -p terraform/lambda
	zip terraform/lambda/main.zip bootstrap
	rm bootstrap

deploy: build
	cd terraform && terraform apply --auto-approve