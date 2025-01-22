// Copyright 2025 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jetstream

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/nats-io/nats-server/v2/server"
)

var testJsProviders map[string]func() (*schema.Provider, error)
var caPEM string
var certPEM string
var keyPEM string

func init() {
	testJsProviders = map[string]func() (*schema.Provider, error){
		"jetstream": func() (*schema.Provider, error) {
			return Provider(), nil
		},
	}

	var err error
	caPEM, certPEM, keyPEM, err = generateCerts()

	if err != nil {
		log.Fatal(err)
	}
}

func checkErr(t *testing.T, err error, format string, a ...any) {
	t.Helper()

	if err == nil {
		return
	}

	t.Fatalf(format, a...)
}

func createJSServer(t *testing.T) (srv *server.Server) {
	t.Helper()

	dir, err := os.MkdirTemp("", "")
	checkErr(t, err, "could not create temporary js store: %v", err)

	srv, err = server.NewServer(&server.Options{
		Port:      -1,
		StoreDir:  dir,
		JetStream: true,
		JetStreamLimits: server.JSLimitOpts{
			MaxRequestBatch: 1,
		},
	})
	checkErr(t, err, "could not start js server: %v", err)

	go srv.Start()
	if !srv.ReadyForConnections(10 * time.Second) {
		t.Errorf("nats server did not start")
	}

	return srv
}

func createJSTLSServer(t *testing.T, verifyClientCert bool) (srv *server.Server) {
	t.Helper()

	dir, err := os.MkdirTemp("", "")
	checkErr(t, err, "could not create temporary js store: %v", err)

	tlsConfig, err := newTLSConfig(caPEM, certPEM, keyPEM)

	checkErr(t, err, "could not create TLS Config: %v", err)

	srv, err = server.NewServer(&server.Options{
		Port:      -1,
		Host:      "localhost",
		StoreDir:  dir,
		JetStream: true,
		TLS:       true,
		TLSConfig: tlsConfig,
		TLSVerify: verifyClientCert,
	})
	checkErr(t, err, "could not start js server: %v", err)

	go srv.Start()
	if !srv.ReadyForConnections(10 * time.Second) {
		t.Errorf("nats server did not start")
	}

	return srv
}
