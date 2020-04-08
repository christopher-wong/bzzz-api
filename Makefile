.PHONY: all run

VERSION=0.0.2

all:
	docker build -t bzzz-api:${VERSION} .
	docker tag bzzz-api:${VERSION} registry.digitalocean.com/teslatrack/bzzz-api:${VERSION}

deploy:
	docker push registry.digitalocean.com/teslatrack/bzzz-api:${VERSION}
	kubectl apply -f k8s

run:
	docker run -p 8080:8080 bzzz-api:${VERSION}

local:
	go run main.go