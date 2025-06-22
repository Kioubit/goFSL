BINARY=goFSL
BUILDFLAGS=-trimpath

build:
	go build -o bin/${BINARY} .

release:
	go build -ldflags="-extldflags=-static" ${BUILDFLAGS} -o bin/${BINARY} .

clean:
	if [ -d "bin/" ]; then find bin/ -type f -delete ;fi
	if [ -d "bin/" ]; then rm -d bin/ ;fi