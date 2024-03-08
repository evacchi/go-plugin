//go:build tinygo.wasm

// Code generated by protoc-gen-go-plugin. DO NOT EDIT.
// versions:
// 	protoc-gen-go-plugin v0.1.0
// 	protoc               v4.25.2
// source: examples/host-function-library/library/json-parser/export/library.proto

package export

import (
	context "context"
	wasm "github.com/knqyf263/go-plugin/wasm"
	_ "unsafe"
)

type parserLibrary struct{}

func NewParserLibrary() ParserLibrary {
	return parserLibrary{}
}

//go:wasmimport json-parser parse_json
func _parse_json(ptr uint32, size uint32) uint64

func (h parserLibrary) ParseJson(ctx context.Context, request *ParseJsonRequest) (*ParseJsonResponse, error) {
	buf, err := request.MarshalVT()
	if err != nil {
		return nil, err
	}
	ptr, size := wasm.ByteToPtr(buf)
	ptrSize := _parse_json(ptr, size)
	wasm.FreePtr(ptr)

	ptr = uint32(ptrSize >> 32)
	size = uint32(ptrSize)
	buf = wasm.PtrToByte(ptr, size)

	response := new(ParseJsonResponse)
	if err = response.UnmarshalVT(buf); err != nil {
		return nil, err
	}
	return response, nil
}
