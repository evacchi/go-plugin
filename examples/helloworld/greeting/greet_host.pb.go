//go:build !tinygo.wasm

// Code generated by protoc-gen-go-plugin. DO NOT EDIT.
// versions:
// 	protoc-gen-go-plugin v0.1.0
// 	protoc               v3.21.12
// source: examples/helloworld/greeting/greet.proto

package greeting

import (
	context "context"
	errors "errors"
	fmt "fmt"
	wazero "github.com/tetratelabs/wazero"
	api "github.com/tetratelabs/wazero/api"
	wasi_snapshot_preview1 "github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	sys "github.com/tetratelabs/wazero/sys"
	io "io"
	fs "io/fs"
	os "os"
)

const GreeterPluginAPIVersion = 1

type GreeterPluginOption struct {
	Stdout io.Writer
	Stderr io.Writer
	FS     fs.FS
}

type GreeterPlugin struct {
	cache  wazero.CompilationCache
	config wazero.ModuleConfig
}

func NewGreeterPlugin(ctx context.Context, opt GreeterPluginOption) (*GreeterPlugin, error) {

	// Create a new WebAssembly CompilationCache.
	cache := wazero.NewCompilationCache()

	// Combine the above into our baseline config, overriding defaults.
	config := wazero.NewModuleConfig().
		// By default, I/O streams are discarded and there's no file system.
		WithStdout(opt.Stdout).WithStderr(opt.Stderr)

	// configure FS only when not nil
	if opt.FS != nil {
		config = config.WithFS(opt.FS)
	}

	return &GreeterPlugin{
		cache:  cache,
		config: config,
	}, nil
}

func (p *GreeterPlugin) Close(ctx context.Context) (err error) {
	if c := p.cache; c != nil {
		err = c.Close(ctx)
	}
	return
}

func (p *GreeterPlugin) Load(ctx context.Context, pluginPath string) (Greeter, error) {
	b, err := os.ReadFile(pluginPath)
	if err != nil {
		return nil, err
	}

	// Create an empty namespace so that multiple modules will not conflict
	r := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCompilationCache(p.cache))

	if _, err = wasi_snapshot_preview1.NewBuilder(r).Instantiate(ctx); err != nil {
		return nil, err
	}

	// Compile the WebAssembly module using the default configuration.
	code, err := r.CompileModule(ctx, b)
	if err != nil {
		return nil, err
	}

	// InstantiateModule runs the "_start" function, WASI's "main".
	module, err := r.InstantiateModule(ctx, code, p.config)
	if err != nil {
		// Note: Most compilers do not exit the module after running "_start",
		// unless there was an Error. This allows you to call exported functions.
		if exitErr, ok := err.(*sys.ExitError); ok && exitErr.ExitCode() != 0 {
			return nil, fmt.Errorf("unexpected exit_code: %d", exitErr.ExitCode())
		} else if !ok {
			return nil, err
		}
	}

	// Compare API versions with the loading plugin
	apiVersion := module.ExportedFunction("greeter_api_version")
	if apiVersion == nil {
		return nil, errors.New("greeter_api_version is not exported")
	}
	results, err := apiVersion.Call(ctx)
	if err != nil {
		return nil, err
	} else if len(results) != 1 {
		return nil, errors.New("invalid greeter_api_version signature")
	}
	if results[0] != GreeterPluginAPIVersion {
		return nil, fmt.Errorf("API version mismatch, host: %d, plugin: %d", GreeterPluginAPIVersion, results[0])
	}

	greet := module.ExportedFunction("greeter_greet")
	if greet == nil {
		return nil, errors.New("greeter_greet is not exported")
	}

	malloc := module.ExportedFunction("malloc")
	if malloc == nil {
		return nil, errors.New("malloc is not exported")
	}

	free := module.ExportedFunction("free")
	if free == nil {
		return nil, errors.New("free is not exported")
	}
	return &greeterPlugin{module: module,
		malloc: malloc,
		free:   free,
		greet:  greet,
	}, nil
}

type greeterPlugin struct {
	module api.Module
	malloc api.Function
	free   api.Function
	greet  api.Function
}

func (p *greeterPlugin) Greet(ctx context.Context, request GreetRequest) (response GreetReply, err error) {
	data, err := request.MarshalVT()
	if err != nil {
		return response, err
	}
	dataSize := uint64(len(data))

	var dataPtr uint64
	// If the input data is not empty, we must allocate the in-Wasm memory to store it, and pass to the plugin.
	if dataSize != 0 {
		results, err := p.malloc.Call(ctx, dataSize)
		if err != nil {
			return response, err
		}
		dataPtr = results[0]
		// This pointer is managed by TinyGo, but TinyGo is unaware of external usage.
		// So, we have to free it when finished
		defer p.free.Call(ctx, dataPtr)

		// The pointer is a linear memory offset, which is where we write the name.
		if !p.module.Memory().Write(uint32(dataPtr), data) {
			return response, fmt.Errorf("Memory.Write(%d, %d) out of range of memory size %d", dataPtr, dataSize, p.module.Memory().Size())
		}
	}

	ptrSize, err := p.greet.Call(ctx, dataPtr, dataSize)
	if err != nil {
		return response, err
	}

	// Note: This pointer is still owned by TinyGo, so don't try to free it!
	resPtr := uint32(ptrSize[0] >> 32)
	resSize := uint32(ptrSize[0])

	// The pointer is a linear memory offset, which is where we write the name.
	bytes, ok := p.module.Memory().Read(resPtr, resSize)
	if !ok {
		return response, fmt.Errorf("Memory.Read(%d, %d) out of range of memory size %d",
			resPtr, resSize, p.module.Memory().Size())
	}

	if err = response.UnmarshalVT(bytes); err != nil {
		return response, err
	}

	return response, nil
}
