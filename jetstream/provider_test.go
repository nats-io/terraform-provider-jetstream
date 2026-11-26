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
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nkeys"
	ntfservice "github.com/synadia-io/orbit.go/ntf"
	ntf "github.com/synadia-io/orbit.go/ntf-client"
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

var testJsProviders = map[string]func() (tfprotov5.ProviderServer, error){
	"jetstream": func() (tfprotov5.ProviderServer, error) {
		return schema.NewGRPCProviderServer(Provider()), nil
	},
}

var ntfService *ntfservice.Service

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	if os.Getenv("TESTER_NATS_URL") == "" {
		var err error
		ntfService, err = ntfservice.New(context.Background(), ntfservice.Options{})
		if err != nil {
			log.Printf("could not start the ntf service: %v", err)
			return 1
		}
		defer ntfService.Close()
	}

	return m.Run()
}

func ntfURL() string {
	url := os.Getenv("TESTER_NATS_URL")
	if url == "" {
		return fmt.Sprintf("nats://127.0.0.1:%d", ntfService.Port())
	}

	return url
}

func checkErr(t *testing.T, err error, format string, a ...any) {
	t.Helper()

	if err == nil {
		return
	}

	t.Fatalf(format, a...)
}

func newJS(t *testing.T, nc *nats.Conn) jetstream.JetStream {
	t.Helper()

	js, err := jetstream.New(nc)
	checkErr(t, err, "could not create a jetstream context: %v", err)

	return js
}

func withJSServer(t *testing.T, cb func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, inst *ntf.Instance), opts ...ntf.CreateOption) {
	t.Helper()

	ntfc := ntf.New(t, ntfURL())
	defer ntfc.Close(t)

	ntfc.WithJetStreamServer(t, func(_ testing.TB, nc *nats.Conn, inst *ntf.Instance) {
		mgr, err := jsm.New(nc)
		checkErr(t, err, "could not create manager: %v", err)

		cb(t, nc, mgr, inst)
	}, append([]ntf.CreateOption{ntf.WithJetStream("limits { max_request_batch: 1 }")}, opts...)...)
}

func withJSCluster(t *testing.T, cb func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, inst *ntf.Instance), opts ...ntf.CreateOption) {
	t.Helper()

	ntfc := ntf.New(t, ntfURL())
	defer ntfc.Close(t)

	ntfc.WithJetStreamCluster(t, 3, func(_ testing.TB, nc *nats.Conn, inst *ntf.Instance) {
		mgr, err := jsm.New(nc)
		checkErr(t, err, "could not create manager: %v", err)

		cb(t, nc, mgr, inst)
	}, append([]ntf.CreateOption{
		ntf.WithClusterName("C1"),
		ntf.WithTopLevel(`server_tags: ["az:1"]`),
		ntf.WithJetStreamWait(30 * time.Second),
	}, opts...)...)
}

func ntfAccounts(users string) []ntf.CreateOption {
	return []ntf.CreateOption{
		ntf.WithAuthorization("# every user authenticates"),
		ntf.WithAccounts(fmt.Sprintf(`accounts {
  TF {
    jetstream: enabled
    users = [%s]
  }
  $SYS { users = [ { user: "system", pass: "password" } ] }
}
`, users)),
	}
}

type jsProxy struct {
	url      string
	mu       sync.Mutex
	requests []string
	fault    func(n int, subject string) func(forward func() (*nats.Msg, error)) (*nats.Msg, error)
}

func withJSProxy(t *testing.T, cb func(proxy *jsProxy, nc *nats.Conn, mgr *jsm.Manager)) {
	t.Helper()

	ntfc := ntf.New(t, ntfURL())
	defer ntfc.Close(t)

	ntfc.WithServer(t, func(_ testing.TB, front *nats.Conn, frontInst *ntf.Instance) {
		ntfc.WithJetStreamServer(t, func(_ testing.TB, nc *nats.Conn, _ *ntf.Instance) {
			proxy := &jsProxy{url: frontInst.Servers[0].URL}

			for _, subject := range []string{"$JS.API.>", "$KV.>", "$O.>"} {
				_, err := front.Subscribe(subject, func(req *nats.Msg) {
					forward := func() (*nats.Msg, error) {
						return nc.RequestMsg(&nats.Msg{Subject: req.Subject, Data: req.Data, Header: req.Header}, 5*time.Second)
					}

					proxy.mu.Lock()
					reply := forward
					if proxy.fault != nil {
						if fault := proxy.fault(len(proxy.requests), req.Subject); fault != nil {
							reply = func() (*nats.Msg, error) { return fault(forward) }
						}
					}
					proxy.requests = append(proxy.requests, req.Subject)
					proxy.mu.Unlock()

					msg, err := reply()
					if err != nil {
						return
					}
					req.RespondMsg(&nats.Msg{Data: msg.Data, Header: msg.Header})
				})
				checkErr(t, err, "could not subscribe to %s: %v", subject, err)
			}
			err := front.Flush()
			checkErr(t, err, "could not flush: %v", err)

			mgr, err := jsm.New(nc)
			checkErr(t, err, "could not create manager: %v", err)

			cb(proxy, nc, mgr)
		})
	})
}

func (p *jsProxy) setFault(fault func(n int, subject string) func(forward func() (*nats.Msg, error)) (*nats.Msg, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.requests = nil
	p.fault = fault
}

func (p *jsProxy) clear() {
	p.setFault(nil)
}

func (p *jsProxy) failRequest(i int) {
	p.setFault(func(n int, _ string) func(func() (*nats.Msg, error)) (*nats.Msg, error) {
		if n != i {
			return nil
		}
		return injectedFault
	})
}

func (p *jsProxy) rewriteSubject(prefix string, edit func(map[string]any)) {
	p.setFault(func(_ int, subject string) func(func() (*nats.Msg, error)) (*nats.Msg, error) {
		if !strings.HasPrefix(subject, prefix) {
			return nil
		}
		return func(forward func() (*nats.Msg, error)) (*nats.Msg, error) {
			msg, err := forward()
			if err != nil {
				return nil, err
			}

			var body map[string]any
			err = json.Unmarshal(msg.Data, &body)
			if err != nil {
				return nil, err
			}
			edit(body)

			msg.Data, err = json.Marshal(body)
			return msg, err
		}
	})
}

func (p *jsProxy) failSubject(prefix string) {
	p.setFault(func(_ int, subject string) func(func() (*nats.Msg, error)) (*nats.Msg, error) {
		if !strings.HasPrefix(subject, prefix) {
			return nil
		}
		return injectedFault
	})
}

func (p *jsProxy) setAPILevel(level int) {
	p.rewriteSubject("$JS.API.INFO", func(info map[string]any) {
		if api, ok := info["api"].(map[string]any); ok {
			api["level"] = level
		}
	})
}

func (p *jsProxy) seen() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]string(nil), p.requests...)
}

func (p *jsProxy) connections(t *testing.T) int {
	t.Helper()

	nc, err := nats.Connect(p.url, nats.UserInfo("system", "password"))
	checkErr(t, err, "could not connect to the system account: %v", err)
	defer nc.Close()

	msg, err := nc.Request("$SYS.REQ.SERVER.PING.CONNZ", nil, 5*time.Second)
	checkErr(t, err, "could not request connz: %v", err)

	var connz struct {
		Data struct {
			NumConnections int `json:"num_connections"`
		} `json:"data"`
	}
	err = json.Unmarshal(msg.Data, &connz)
	checkErr(t, err, "could not parse connz: %v", err)

	return connz.Data.NumConnections
}

func injectedFault(func() (*nats.Msg, error)) (*nats.Msg, error) {
	return &nats.Msg{Data: []byte(`{"error":{"code":500,"err_code":10000,"description":"injected fault"}}`)}, nil
}

func providerMeta(t *testing.T, config map[string]any) any {
	t.Helper()

	raw := terraform.NewResourceConfigRaw(config)
	raw.CtyValue = cty.ObjectVal(map[string]cty.Value{
		"credentials":     cty.NullVal(cty.String),
		"credential_data": cty.NullVal(cty.String),
	})

	provider := Provider()
	diags := provider.Configure(context.Background(), raw)
	if diags.HasError() {
		t.Fatalf("could not configure the provider: %v", diags)
	}

	return provider.Meta()
}

func callResource(t *testing.T, proxy *jsProxy, fn func(context.Context, *schema.ResourceData, any) diag.Diagnostics, res *schema.Resource, id string, attrs map[string]any) (*schema.ResourceData, diag.Diagnostics) {
	t.Helper()

	d := schema.TestResourceDataRaw(t, res.Schema, attrs)
	d.SetId(id)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return d, fn(ctx, d, providerMeta(t, map[string]any{"servers": proxy.url}))
}

func testEachRequestFails(t *testing.T, proxy *jsProxy, setup func(*testing.T), call func(*testing.T) diag.Diagnostics) {
	t.Helper()

	setup(t)
	proxy.clear()
	diags := call(t)
	if diags.HasError() {
		t.Fatalf("unexpected error without faults: %v", diags)
	}
	requests := proxy.seen()

	for i, subject := range requests {
		t.Run(strings.ReplaceAll(subject, ".", "_"), func(t *testing.T) {
			setup(t)
			proxy.failRequest(i)
			defer proxy.clear()

			diags := call(t)
			if !diags.HasError() {
				t.Fatalf("expected an error with request %d (%s) failing, requests were %v", i, subject, requests)
			}
		})
	}
}

const testInboxRestrictedUsers = `
    { user: "admin", password: "s3cret" }
    { user: "terraform", password: "s3cret", permissions: {
        publish: { allow: ["$JS.API.>", "$KV.>"] }
        subscribe: { allow: ["_INBOX_terraform.>"] }
    } }
`

func TestProviderWithInboxPrefix(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		js := newJS(t, nc)
		ctx := t.Context()

		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			CheckDestroy:             testStreamDoesNotExist(t, mgr, "KV_TEST"),
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
						testStreamExist(t, mgr, "KV_TEST"),
						testEntryExist(ctx, t, js, "TEST", "foo"),
					),
				},
			},
		})
	}, append(ntfAccounts(testInboxRestrictedUsers), ntf.WithConnectOptions(nats.UserInfo("admin", "s3cret")))...)
}

func TestProviderCredentialDataIsSensitive(t *testing.T) {
	if !Provider().Schema["credential_data"].Sensitive {
		t.Fatal("provider field \"credential_data\" is not marked sensitive")
	}
}

func TestProviderCredentialDataOverridesEnv(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		t.Setenv("NATS_CREDS", "/nonexistent.creds")

		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			Steps: []resource.TestStep{
				{
					Config:      fmt.Sprintf(testProviderConnect, nc.ConnectedUrl(), `credential_data = "not a credentials file"`),
					ExpectError: regexp.MustCompile(`no nkey seed found`),
				},
			},
		})
	})
}

func TestProviderEmptyCredentialsIgnored(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			Steps: []resource.TestStep{
				{
					Config:      fmt.Sprintf(testProviderConnect, nc.ConnectedUrl(), "credentials = \"\"\n  credential_data = \"not a credentials file\""),
					ExpectError: regexp.MustCompile(`no nkey seed found`),
				},
				{
					Config:      fmt.Sprintf(testProviderConnect, nc.ConnectedUrl(), "credentials = \"/nonexistent.creds\"\n  credential_data = \"\""),
					ExpectError: regexp.MustCompile(`open /nonexistent.creds`),
				},
			},
		})
	})
}

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

	nko, err := nats.NkeyOptionFromSeed(seedFile)
	checkErr(t, err, "could not load seed: %v", err)

	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			CheckDestroy:             testStreamDoesNotExist(t, mgr, "KV_TEST"),
			Steps: []resource.TestStep{
				{
					Config: fmt.Sprintf(testProviderConnect, nc.ConnectedUrl(), fmt.Sprintf("nkey = %q", seedFile)),
					Check:  testStreamExist(t, mgr, "KV_TEST"),
				},
			},
		})
	}, append(ntfAccounts(fmt.Sprintf(`{ nkey: %q }`, pub)), ntf.WithConnectOptions(nko))...)
}

func TestProviderAuthConflicts(t *testing.T) {
	for _, tc := range []struct {
		auth string
		err  string
	}{
		{"user = \"u\"\n  nkey = \"/seed.nk\"", `conflicts with`},
		{"user = \"u\"\n  credentials = \"/user.creds\"", `conflicts with`},
		{"user = \"u\"\n  credential_data = \"data\"", `conflicts with`},
		{"nkey = \"/seed.nk\"\n  credentials = \"/user.creds\"", `conflicts with`},
		{"nkey = \"/seed.nk\"\n  credential_data = \"data\"", `conflicts with`},
		{"credentials = \"/path/to/creds\"\n  credential_data = \"data\"", `only one of credentials and credential_data may be set`},
	} {
		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			Steps: []resource.TestStep{
				{
					Config:      fmt.Sprintf(testProviderConnect, "nats://127.0.0.1:4222", tc.auth),
					ExpectError: regexp.MustCompile(tc.err),
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
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		t.Setenv("NATS_URL", nc.ConnectedUrl())

		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			CheckDestroy:             testStreamDoesNotExist(t, mgr, "KV_TEST"),
			Steps: []resource.TestStep{
				{
					Config: testProviderServersFromEnv,
					Check:  testStreamExist(t, mgr, "KV_TEST"),
				},
			},
		})
	})
}

const testProviderConnect = `
provider "jetstream" {
  servers = "%s"
  %s
}

resource "jetstream_kv_bucket" "test" {
  name = "TEST"
}
`

func TestProviderConnectErrors(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			Steps: []resource.TestStep{
				{
					Config:      fmt.Sprintf(testProviderConnect, nc.ConnectedUrl(), `nkey = "/nonexistent.nk"`),
					ExpectError: regexp.MustCompile(`nonexistent.nk`),
				},
				{
					Config:      fmt.Sprintf(testProviderConnect, nc.ConnectedUrl(), `tls { cert_file = "/client.pem" }`),
					ExpectError: regexp.MustCompile(`cert_file and key_file depend on each other`),
				},
			},
		})
	})
}

func TestProviderTLSDataWithoutTempDir(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		for _, field := range []string{"ca_file_data", "cert_file_data", "key_file_data"} {
			t.Run(field, func(t *testing.T) {
				t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))

				connect := providerMeta(t, map[string]any{"servers": nc.ConnectedUrl(), "tls": []any{map[string]any{field: "pem"}}})
				_, _, err := connect.(func() (*nats.Conn, *jsm.Manager, error))()
				if err == nil || !strings.Contains(err.Error(), "no such file or directory") {
					t.Fatalf("expected a temp file error, got %v", err)
				}
			})
		}
	})
}

const testTokenServer = `
server_name: "{{ .ServerName }}"
port: {{ .ClientPort }}
jetstream { store_dir: "{{ .StoreDir }}" }
authorization { token: "s3cret" }
`

func TestProviderWithToken(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			CheckDestroy:             testStreamDoesNotExist(t, mgr, "KV_TEST"),
			Steps: []resource.TestStep{
				{
					Config: fmt.Sprintf(testProviderConnect, nc.ConnectedUrl(), `user = "s3cret"`),
					Check:  testStreamExist(t, mgr, "KV_TEST"),
				},
			},
		})
	}, ntf.WithTemplate(testTokenServer), ntf.WithConnectOptions(nats.Token("s3cret")))
}

const testOperatorServer = `
server_name: "{{ .ServerName }}"
port: {{ .ClientPort }}
jetstream { store_dir: "{{ .StoreDir }}" }
operator: %s
system_account: %s
resolver: MEMORY
resolver_preload: {
  %s: %s
  %s: %s
}
`

func TestProviderWithCredentialData(t *testing.T) {
	operator, err := nkeys.CreateOperator()
	checkErr(t, err, "could not create operator: %v", err)
	system, err := nkeys.CreateAccount()
	checkErr(t, err, "could not create system account: %v", err)
	account, err := nkeys.CreateAccount()
	checkErr(t, err, "could not create account: %v", err)
	user, err := nkeys.CreateUser()
	checkErr(t, err, "could not create user: %v", err)

	operatorPub, _ := operator.PublicKey()
	systemPub, _ := system.PublicKey()
	accountPub, _ := account.PublicKey()
	userPub, _ := user.PublicKey()
	userSeed, _ := user.Seed()

	operatorClaims := jwt.NewOperatorClaims(operatorPub)
	operatorClaims.SystemAccount = systemPub
	operatorJWT, err := operatorClaims.Encode(operator)
	checkErr(t, err, "could not encode operator: %v", err)

	systemJWT, err := jwt.NewAccountClaims(systemPub).Encode(operator)
	checkErr(t, err, "could not encode system account: %v", err)

	accountClaims := jwt.NewAccountClaims(accountPub)
	accountClaims.Limits.JetStreamLimits = jwt.JetStreamLimits{MemoryStorage: -1, DiskStorage: -1, Streams: -1, Consumer: -1}
	accountJWT, err := accountClaims.Encode(operator)
	checkErr(t, err, "could not encode account: %v", err)

	userJWT, err := jwt.NewUserClaims(userPub).Encode(account)
	checkErr(t, err, "could not encode user: %v", err)

	creds, err := jwt.FormatUserConfig(userJWT, userSeed)
	checkErr(t, err, "could not format credentials: %v", err)

	server := fmt.Sprintf(testOperatorServer, operatorJWT, systemPub, systemPub, systemJWT, accountPub, accountJWT)
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			CheckDestroy:             testStreamDoesNotExist(t, mgr, "KV_TEST"),
			Steps: []resource.TestStep{
				{
					Config: fmt.Sprintf(testProviderConnect, nc.ConnectedUrl(), fmt.Sprintf("credential_data = %q", string(creds))),
					Check:  testStreamExist(t, mgr, "KV_TEST"),
				},
			},
		})
	}, ntf.WithTemplate(server), ntf.WithConnectOptions(nats.UserJWTAndSeed(userJWT, string(userSeed))))
}

func TestProviderUnsupportedAPILevel(t *testing.T) {
	withJSProxy(t, func(proxy *jsProxy, nc *nats.Conn, mgr *jsm.Manager) {
		proxy.setAPILevel(0)

		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			Steps: []resource.TestStep{
				{
					Config:      fmt.Sprintf(testProviderConnect, proxy.url, ""),
					ExpectError: regexp.MustCompile(`unsupported api level: 0. Requires NATS Server 2.11 or newer`),
				},
			},
		})
	})
}

func TestProviderClosesConnectionOnError(t *testing.T) {
	withJSProxy(t, func(proxy *jsProxy, nc *nats.Conn, mgr *jsm.Manager) {
		for _, tc := range []struct {
			name  string
			fault func()
		}{
			{"unsupported api level", func() { proxy.setAPILevel(0) }},
			{"api level request fails", func() { proxy.failSubject("$JS.API.INFO") }},
		} {
			t.Run(tc.name, func(t *testing.T) {
				before := proxy.connections(t)

				tc.fault()
				defer proxy.clear()

				connect := providerMeta(t, map[string]any{"servers": proxy.url})
				_, _, err := connect.(func() (*nats.Conn, *jsm.Manager, error))()
				if err == nil {
					t.Fatal("expected the connection to fail")
				}

				deadline := time.Now().Add(2 * time.Second)
				for proxy.connections(t) != before && time.Now().Before(deadline) {
					time.Sleep(50 * time.Millisecond)
				}
				if after := proxy.connections(t); after != before {
					t.Fatalf("expected %d connections after the failed connect, got %d", before, after)
				}
			})
		}
	})
}
