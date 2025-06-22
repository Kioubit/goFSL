BINARY=goFSL
BUILDFLAGS=-trimpath
VERSION=`git describe --tags`

build:
	go build -o bin/${BINARY} .

release:
	go build -ldflags="-extldflags=-static" ${BUILDFLAGS} -o bin/${BINARY} .

release-all:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ${BUILDFLAGS} -o bin/${BINARY}_${VERSION}_linux_amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ${BUILDFLAGS} -o bin/${BINARY}_${VERSION}_linux_arm64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm go build ${BUILDFLAGS} -o bin/${BINARY}_${VERSION}_linux_arm

clean:
	if [ -d "bin/" ]; then find bin/ -type f -delete ;fi
	if [ -d "bin/" ]; then rm -d bin/ ;fi