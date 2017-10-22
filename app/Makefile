

MAIN=main.go
BINARY=app


clean:
	if [ -f "${BINARY}" ] ; then rm -f ${BINARY} ; fi


build: clean
	CGO_ENABLED=0 GOOS=linux go build -o app main.go


docker: build
	docker build -t runyonsolutions/appinfo:1 . -f Dockerfile.v1
	docker build -t runyonsolutions/appinfo:2 . -f Dockerfile.v2
	docker build -t runyonsolutions/appinfo:3 . -f Dockerfile.v3



push:
	docker push runyonsolutions/appinfo:1
	docker push runyonsolutions/appinfo:2
	docker push runyonsolutions/appinfo:3