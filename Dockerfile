FROM golang:1.14

RUN mkdir /app
WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go build -o bzzz-api .

EXPOSE 8080

ENTRYPOINT [ "./bzzz-api" ]