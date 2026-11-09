package ipc

import (
	"fmt"
	"sync"
	"testing"

	"github.com/beilunyang/openhongguo/sdk"
)

func TestRuntimeIdentityConcurrentReadWrite(t *testing.T) {
	service := &HongguoService{
		userAgent:   "initial-agent",
		storeRegion: "cn-gd",
	}

	var writers sync.WaitGroup
	const writersCount = 8
	const updatesPerWriter = 250
	for writer := 0; writer < writersCount; writer++ {
		writers.Add(1)
		go func(writer int) {
			defer writers.Done()
			for update := 0; update < updatesPerWriter; update++ {
				profile := &sdk.DeviceProfile{
					DeviceID:    fmt.Sprintf("device-%d-%d", writer, update),
					InstallID:   fmt.Sprintf("install-%d-%d", writer, update),
					StoreRegion: fmt.Sprintf("region-%d-%d", writer, update),
				}
				service.applyDeviceProfile(profile)
				service.setUserAgent(fmt.Sprintf("agent-%d-%d", writer, update))
			}
		}(writer)
	}

	var readers sync.WaitGroup
	for reader := 0; reader < writersCount; reader++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for update := 0; update < updatesPerWriter; update++ {
				userAgent, storeRegion := service.runtimeIdentity()
				if userAgent == "" || storeRegion == "" {
					t.Errorf("runtime identity became empty: %q/%q", userAgent, storeRegion)
					return
				}
			}
		}()
	}

	writers.Wait()
	readers.Wait()
}
