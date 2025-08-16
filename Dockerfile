FROM golang AS build
WORKDIR /workflow
COPY . .
RUN CGO_ENABLED=0 go build -o /go/bin/workflow ./

FROM alpine
RUN apk --no-cache add ca-certificates
COPY --from=build /go/bin/workflow /bin/workflow
ENTRYPOINT [ "/bin/workflow" ]