package jetstream

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	"github.com/nats-io/nats-server/v2/server"
)

var testJsProviders map[string]terraform.ResourceProvider
var testJsProvider *schema.Provider

func init() {
	testJsProvider = Provider().(*schema.Provider)
	testJsProviders = map[string]terraform.ResourceProvider{
		"jetstream": testJsProvider,
	}
}

func checkErr(t *testing.T, err error, format string, a ...interface{}) {
	t.Helper()

	if err == nil {
		return
	}

	t.Fatalf(format, a...)
}

func createJSServer(t *testing.T) (srv *server.Server) {
	t.Helper()

	dir, err := ioutil.TempDir("", "")
	checkErr(t, err, "could not create temporary js store: %v", err)

	srv, err = server.NewServer(&server.Options{
		Port:      -1,
		StoreDir:  dir,
		JetStream: true,
	})
	checkErr(t, err, "could not start js server: %v", err)

	go srv.Start()
	if !srv.ReadyForConnections(10 * time.Second) {
		t.Errorf("nats server did not start")
	}

	return srv
}
