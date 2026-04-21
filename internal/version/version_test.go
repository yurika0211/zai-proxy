package version

import (
	"testing"
)

func TestGetFeVersion_Initial(t *testing.T) {
	// feVersion starts empty
	v := GetFeVersion()
	// Just verify it doesn't panic and returns a string
	_ = v
}

func TestGetFeVersion_AfterSet(t *testing.T) {
	versionLock.Lock()
	feVersion = "prod-fe-1.2.3"
	versionLock.Unlock()

	v := GetFeVersion()
	if v != "prod-fe-1.2.3" {
		t.Errorf("expected prod-fe-1.2.3, got %s", v)
	}

	// Reset
	versionLock.Lock()
	feVersion = ""
	versionLock.Unlock()
}

func TestGetFeVersion_ConcurrentAccess(t *testing.T) {
	versionLock.Lock()
	feVersion = ""
	versionLock.Unlock()

	done := make(chan bool)
	// Writer
	go func() {
		for i := 0; i < 100; i++ {
			versionLock.Lock()
			feVersion = "prod-fe-test"
			versionLock.Unlock()
		}
		done <- true
	}()
	// Reader
	go func() {
		for i := 0; i < 100; i++ {
			_ = GetFeVersion()
		}
		done <- true
	}()

	<-done
	<-done
}