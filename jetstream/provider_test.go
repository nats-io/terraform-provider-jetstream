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
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nkeys"
)

const testInboxPrefixConfig = `
provider "jetstream" {
  servers = "%s"
  user = "terraform"
  password = "s3cret"
  %s
}

resource "jetstream_kv_bucket" "test" {
  name = "TEST"
}

resource "jetstream_kv_entry" "test_entry" {
  bucket = jetstream_kv_bucket.test.name
  key = "foo"
  value = "bar"
}
`

var testJsProviders map[string]func() (tfprotov5.ProviderServer, error)
var caPEM string
var certPEM string
var keyPEM string

func init() {
	testJsProviders = map[string]func() (tfprotov5.ProviderServer, error){
		"jetstream": func() (tfprotov5.ProviderServer, error) {
			return schema.NewGRPCProviderServer(Provider()), nil
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

func createJSInboxRestrictedServer(t *testing.T) (srv *server.Server) {
	t.Helper()

	srv, err := server.NewServer(&server.Options{
		Port:      -1,
		StoreDir:  t.TempDir(),
		JetStream: true,
		Users: []*server.User{
			{Username: "admin", Password: "s3cret"},
			{
				Username: "terraform",
				Password: "s3cret",
				Permissions: &server.Permissions{
					Publish:   &server.SubjectPermission{Allow: []string{"$JS.API.>", "$KV.>"}},
					Subscribe: &server.SubjectPermission{Allow: []string{"_INBOX_terraform.>"}},
				},
			},
		},
	})
	checkErr(t, err, "could not start js server: %v", err)

	go srv.Start()
	if !srv.ReadyForConnections(10 * time.Second) {
		t.Errorf("nats server did not start")
	}

	return srv
}

func TestProviderWithInboxPrefix(t *testing.T) {
	srv := createJSInboxRestrictedServer(t)
	defer srv.Shutdown()

	nc, err := nats.Connect(srv.ClientURL(), nats.UserInfo("admin", "s3cret"))
	if err != nil {
		t.Fatalf("could not connect: %s", err)
	}
	defer nc.Close()

	mgr, err := jsm.New(nc)
	if err != nil {
		t.Fatalf("could not connect: %s", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatalf("could not connect: %s", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: testJsProviders,
		CheckDestroy:             testBucketDoesNotExist(t, mgr, "TEST"),
		Steps: []resource.TestStep{
			{
				Config:      fmt.Sprintf(testInboxPrefixConfig, nc.ConnectedUrl(), `inbox_prefix = "_INBOX_terraform."`),
				ExpectError: regexp.MustCompile(`invalid inbox_prefix "_INBOX_terraform."`),
			},
			{
				Config:      fmt.Sprintf(testInboxPrefixConfig, nc.ConnectedUrl(), ""),
				ExpectError: regexp.MustCompile(`context deadline exceeded`),
			},
			{
				Config: fmt.Sprintf(testInboxPrefixConfig, nc.ConnectedUrl(), `inbox_prefix = "_INBOX_terraform"`),
				Check: resource.ComposeTestCheckFunc(
					testBucketExist(t, mgr, "TEST"),
					testEntryExist(ctx, t, js, "TEST", "foo"),
				),
			},
		},
	})
}

func createJSCluster(t *testing.T) []*server.Server {
	t.Helper()

	var ports []int
	var routes []string
	for range 3 {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		checkErr(t, err, "could not allocate cluster port: %v", err)
		port := l.Addr().(*net.TCPAddr).Port
		l.Close()
		ports = append(ports, port)
		routes = append(routes, fmt.Sprintf("nats://127.0.0.1:%d", port))
	}

	var servers []*server.Server
	for i, port := range ports {
		srv, err := server.NewServer(&server.Options{
			ServerName: fmt.Sprintf("n%d", i+1),
			Tags:       jwt.TagList{"az:1"},
			Host:       "127.0.0.1",
			Port:       -1,
			StoreDir:   t.TempDir(),
			JetStream:  true,
			Cluster: server.ClusterOpts{
				Name: "C1",
				Host: "127.0.0.1",
				Port: port,
			},
			Routes: server.RoutesFromStr(strings.Join(routes, ",")),
		})
		checkErr(t, err, "could not start js server: %v", err)

		go srv.Start()
		t.Cleanup(srv.Shutdown)
		servers = append(servers, srv)
	}

	for _, srv := range servers {
		if !srv.ReadyForConnections(20 * time.Second) {
			t.Fatalf("nats server did not start")
		}
	}

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		for _, srv := range servers {
			if srv.JetStreamIsLeader() && srv.NumRoutes() >= 2 {
				return servers
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("cluster did not elect a JetStream meta leader")

	return nil
}

func TestProviderCredentialsConflict(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: testJsProviders,
		Steps: []resource.TestStep{
			{
				Config: `
provider "jetstream" {
  servers         = "nats://127.0.0.1:4222"
  credentials     = "/path/to/creds"
  credential_data = "data"
}

resource "jetstream_kv_bucket" "test" {
  name = "TEST"
}
`,
				ExpectError: regexp.MustCompile(`only one of credentials and credential_data may be set`),
			},
		},
	})
}

const testProviderCredentialDataOverridesEnv = `
provider "jetstream" {
  servers         = "%s"
  credential_data = "not a credentials file"
}

resource "jetstream_kv_bucket" "test" {
  name = "TEST"
}
`

func TestProviderCredentialDataOverridesEnv(t *testing.T) {
	srv := createJSServer(t)
	defer srv.Shutdown()

	t.Setenv("NATS_CREDS", "/nonexistent.creds")

	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: testJsProviders,
		Steps: []resource.TestStep{
			{
				Config:      fmt.Sprintf(testProviderCredentialDataOverridesEnv, srv.ClientURL()),
				ExpectError: regexp.MustCompile(`no nkey seed found`),
			},
		},
	})
}

const testProviderEmptyCredentials = `
provider "jetstream" {
  servers         = "%s"
  credentials     = %q
  credential_data = %q
}

resource "jetstream_kv_bucket" "test" {
  name = "TEST"
}
`

func TestProviderEmptyCredentialsIgnored(t *testing.T) {
	srv := createJSServer(t)
	defer srv.Shutdown()

	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: testJsProviders,
		Steps: []resource.TestStep{
			{
				Config:      fmt.Sprintf(testProviderEmptyCredentials, srv.ClientURL(), "", "not a credentials file"),
				ExpectError: regexp.MustCompile(`no nkey seed found`),
			},
			{
				Config:      fmt.Sprintf(testProviderEmptyCredentials, srv.ClientURL(), "/nonexistent.creds", ""),
				ExpectError: regexp.MustCompile(`open /nonexistent.creds`),
			},
		},
	})
}

const testProviderNkey = `
provider "jetstream" {
  servers = "%s"
  nkey    = "%s"
}

resource "jetstream_kv_bucket" "test" {
  name = "TEST"
}
`

func TestProviderWithNkey(t *testing.T) {
	user, err := nkeys.CreateUser()
	checkErr(t, err, "could not create nkey: %v", err)
	pub, err := user.PublicKey()
	checkErr(t, err, "could not read public key: %v", err)
	seed, err := user.Seed()
	checkErr(t, err, "could not read seed: %v", err)

	seedFile := filepath.Join(t.TempDir(), "user.nk")
	err = os.WriteFile(seedFile, seed, 0600)
	checkErr(t, err, "could not write seed: %v", err)

	srv, err := server.NewServer(&server.Options{
		Port:      -1,
		StoreDir:  t.TempDir(),
		JetStream: true,
		Nkeys:     []*server.NkeyUser{{Nkey: pub}},
	})
	checkErr(t, err, "could not start js server: %v", err)
	go srv.Start()
	defer srv.Shutdown()
	if !srv.ReadyForConnections(10 * time.Second) {
		t.Fatalf("nats server did not start")
	}

	nko, err := nats.NkeyOptionFromSeed(seedFile)
	checkErr(t, err, "could not load seed: %v", err)
	nc, err := nats.Connect(srv.ClientURL(), nko)
	checkErr(t, err, "could not connect: %v", err)
	defer nc.Close()

	mgr, err := jsm.New(nc)
	checkErr(t, err, "could not connect: %v", err)

	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: testJsProviders,
		CheckDestroy:             testBucketDoesNotExist(t, mgr, "TEST"),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testProviderNkey, srv.ClientURL(), seedFile),
				Check:  testBucketExist(t, mgr, "TEST"),
			},
		},
	})
}

const testProviderAuthConflict = `
provider "jetstream" {
  servers = "nats://127.0.0.1:4222"
  %s
}

resource "jetstream_kv_bucket" "test" {
  name = "TEST"
}
`

func TestProviderAuthConflicts(t *testing.T) {
	for _, auth := range []string{
		"user = \"u\"\n  nkey = \"/seed.nk\"",
		"user = \"u\"\n  credentials = \"/user.creds\"",
		"user = \"u\"\n  credential_data = \"data\"",
		"nkey = \"/seed.nk\"\n  credentials = \"/user.creds\"",
		"nkey = \"/seed.nk\"\n  credential_data = \"data\"",
	} {
		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			Steps: []resource.TestStep{
				{
					Config:      fmt.Sprintf(testProviderAuthConflict, auth),
					ExpectError: regexp.MustCompile(`conflicts with`),
				},
			},
		})
	}
}

const testProviderServersFromEnv = `
provider "jetstream" {}

resource "jetstream_kv_bucket" "test" {
  name = "TEST"
}
`

func TestProviderServersFromEnv(t *testing.T) {
	srv := createJSServer(t)
	defer srv.Shutdown()

	nc, err := nats.Connect(srv.ClientURL())
	checkErr(t, err, "could not connect: %v", err)
	defer nc.Close()

	mgr, err := jsm.New(nc)
	checkErr(t, err, "could not connect: %v", err)

	t.Setenv("NATS_URL", srv.ClientURL())

	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: testJsProviders,
		CheckDestroy:             testBucketDoesNotExist(t, mgr, "TEST"),
		Steps: []resource.TestStep{
			{
				Config: testProviderServersFromEnv,
				Check:  testBucketExist(t, mgr, "TEST"),
			},
		},
	})
}
