/*
Package versions provides agent baker version types and a mapping of versions to localhost addresses
that different agent baker versions are running on.

This package looks into the sub-directory, binaries, which contains folders named for agent baker versions.
Inside each directory, there should be a binary called 'agentbaker' that is the agent baker binary for that version.
This package will extract the binaries and run them on localhost on some port. It returns a mapping of the versions
to the localhost addresses that the agent bakers are running on.

If a directory has a bad version or the agent won't start, an error is returned.

Usage is simple:

	verMap, err := versions.New()
	if err != nil {
		panic(err)
	}

	// Use verMap to get the base address for a version.
	// This can be used to send requests to the agent baker service.
	base := verMap.Base(versions.Latest)
	if base == "" {
		panic("latest version not found")
	}

Substitute panics with proper error handling.
*/
package versions

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/util/wait"
)

//go:embed binaries
var binariesFS embed.FS

// Version describes a AgentBaker version.
type Version string

func (v Version) validate() error {
	// Some detection logic here
	return nil
}

// String implements the fmt.Stringer interface.
func (v Version) String() string {
	return string(v)
}

// Latest is a special version that always points to the latest version.
var Latest = Version("latest")

// Mapping is a map of versions to connections.
type Mapping struct {
	versions map[Version]string
}

// Base returns the base address where the agent baker service for the given version is running.
// If this is empty string, the version is not found. The returned address will be in the form of
// "http://localhost:<port>".
func (m Mapping) Base(v Version) string {
	return m.versions[v]
}

// launchConfig holds configuration elements for launching a version.
// This can be stored next to an agent baker binary to configure it via flags and toggles.
type launchConfig struct {
}

type versionPath struct {
	version Version
	bin     []byte
	addr    string
}

// New creates a new mapping of versions to localhost addresses.
func New() (Mapping, error) {
	// TODO: Need to add some logic to find the latest version and make a mapping to that.
	verPaths, err := extractBinaries()
	if err != nil {
		return Mapping{}, err
	}

	if err := spawnVersions(verPaths); err != nil {
		return Mapping{}, err
	}

	m := Mapping{
		versions: map[Version]string{},
	}

	for _, vp := range verPaths {
		m.versions[vp.version] = vp.addr
	}
	return m, nil
}

type binFS interface {
	fs.ReadDirFS
	fs.ReadFileFS
}

// extractBinaries reads the embedded filesystem and extracts the agent baker binaries.
func extractBinaries(rdfs binFS) ([]versionPath, error) {
	versions, err := rdfs.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("could not read the versions directory: %v", err)
	}

	verPaths := []versionPath{}
	for _, fn := range versions {
		if !fn.IsDir() {
			continue
		}

		ver := Version(fn.Name())
		if err := ver.validate(); err != nil {
			return nil, fmt.Errorf("embed filesystem had version that did not validate: %v", err)
		}

		binPath := filepath.Join(fn.Name(), "agentbaker")
		content, err := binariesFS.ReadFile(binPath)
		if err != nil {
			return nil, fmt.Errorf("could not read agentbaker file for version(%v): %v", ver, err)
		}
		verPaths = append(verPaths, versionPath{version: ver, bin: content})
	}
	return verPaths, nil
}

// spawnVersion takes a list of agent baker versions and the relevant binaries and runs them.
// It modifies the versionPath slice in place to add the address of the running agent baker instances.
func spawnVersions(verPaths []versionPath) error {
	ports := atomic.Int32{}
	ports.Store(8080)

	tmpdir := os.TempDir()

	g := wait.Group{}

	for i, vp := range verPaths {
		i := i
		vp := vp

		g.Go(func(ctx context.Context) error {
			fp := filepath.Join(tmpdir, vp.version.String())

			if err := os.WriteFile(p, vp.bin, 0755); err != nil {
				return fmt.Errorf("could not write agentbaker binary file(%v): %v", vp.version, err)
			}
			port := ports.Add(1) - 1

			vp.addr = fmt.Sprintf("http://localhost:%d", port)

			// NOTE: We would really want to monitor the health of the binary after start. And should decide what to do
			// if an underlying binary crashes.
			if err := exec.Command(fp, "-port", vp.addr).Start(); err != nil {
				return fmt.Errorf("could not start agentbaker binary(%v): %v", vp.version, err)
			}
			verPaths[i] = vp
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}
