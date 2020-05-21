.PHONY: serve wasm

GOPATH=$(shell go env GOROOT)
CMDPATH=./cmd/fire

run:
	go run ${CMDPATH}/main.go

serve:
	python3 -m http.server --directory www 8080

wasm:
	@cp ${GOPATH}/misc/wasm/wasm_exec.js www/
	GOOS=js GOARCH=wasm go build -o www/fire.wasm ${CMDPATH}/main.go

linux:
	go build -o build/fire ${CMDPATH}/main.go

windows:
	env GOOS=windows GOARCH=amd64 go build -o build/fire.exe ${CMDPATH}//main.go