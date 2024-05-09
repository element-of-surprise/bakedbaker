package http

import (
	"testing"

	"github.com/element-of-surprise/bakedbaker/internal/versions"
	"github.com/kylelemons/godebug/pretty"
)

func TestVersionedRequest(t *testing.T) {
	t.Parallel()

	// Config is simply a type that we can use to test the versionedRequest function.
	// As long as json decodes something, we don't care about the actual data.
	// It is up to the agentbaker receiver to deal with the data validation.
	type Config struct {
		Type string
		Data string
	}

	tests := []struct {
		name       string
		body       []byte
		wantConfig Config
		wantVer    string
		err        bool
	}{
		{
			name: "Error: Bad JSON",
			body: []byte(`{`),
			err:  true,
		},
		{
			name: "Error: ABVersion is set, but Req is not",
			body: []byte(`{"ABVersion":"1.0.0"}`),
			err:  true,
		},
		{
			name: "Error: Non-versioned request, but also doesn't configure the config",
			body: []byte(`{"Random": "data"}`), // Doesn't conform to the Config struct
			err:  true,
		},
		{
			name:    "Non-versioned request, has Config so we should get versioned.latest",
			body:    []byte(`{"Type": "test", "Data": "data"}`),
			wantVer: versions.Latest.String(),
			wantConfig: Config{
				Type: "test",
				Data: "data",
			},
		},
		{
			name: "Versioned request, has Config but doesn't set the ABVersion",
			body: []byte(`{"Req":{"Type": "test", "Data": "data"}}`),
			err:  true,
		},
		{
			name:    "Versioned request, has Config and sets the ABVersion",
			body:    []byte(`{"ABVersion":"1.0.0","Req":{"Type": "test", "Data": "data"}}`),
			wantVer: "1.0.0",
			wantConfig: Config{
				Type: "test",
				Data: "data",
			},
		},
	}

	for _, test := range tests {
		gotVer, gotConfig, err := versionedRequest[Config](test.body)
		switch {
		case test.err && err == nil:
			t.Errorf("TestVersionedRequest(%s): got err == nil, want err != nil", test.name)
			continue
		case !test.err && err != nil:
			t.Errorf("TestVersionedRequest(%s): got err != %v, want err == nil", test.name, err)
			continue
		case err != nil:
			continue
		}

		if gotVer.String() != test.wantVer {
			t.Errorf("TestVersionedRequest(%s): got version %s, want %s", test.name, gotVer, test.wantVer)
		}
		if diff := pretty.Compare(test.wantConfig, gotConfig); diff != "" {
			t.Errorf("TestVersionedRequest(%s): -want/+got:\n%s", test.name, diff)
		}
	}
}
