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
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	ntf "github.com/synadia-io/orbit.go/ntf-client"
)

const testKVEntry_basic = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_kv_bucket" "test" {
  name = "TEST"
  ttl = 60
  history = 10
  max_value_size = 1024
  max_bucket_size = 10240
}

resource "jetstream_kv_entry" "test_entry" {
  bucket = jetstream_kv_bucket.test.name
  key = "foo"
  value = "bar"
}
`

func TestResourceKVEntry(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		js := newJS(t, nc)
		ctx := t.Context()

		updateBasicConfig := strings.ReplaceAll(testKVEntry_basic, "bar", "baz")

		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			CheckDestroy:             testEntryDoesNotExist(ctx, t, js, "TEST", "foo"),
			Steps: []resource.TestStep{
				{
					Config: fmt.Sprintf(testKVEntry_basic, nc.ConnectedUrl()),
					Check: resource.ComposeTestCheckFunc(
						testStreamExist(t, mgr, "KV_TEST"),
						testEntryExist(ctx, t, js, "TEST", "foo"),
						resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "bucket", "TEST"),
						resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "key", "foo"),
						resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "value", "bar"),
						resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "revision", "1"),
					),
				},
				{
					ResourceName:      "jetstream_kv_entry.test_entry",
					ImportState:       true,
					ImportStateVerify: true,
				},
				{
					Config: fmt.Sprintf(updateBasicConfig, nc.ConnectedUrl()),
					Check: resource.ComposeTestCheckFunc(
						testStreamExist(t, mgr, "KV_TEST"),
						testEntryExist(ctx, t, js, "TEST", "foo"),
						resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "bucket", "TEST"),
						resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "key", "foo"),
						resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "value", "baz"),
						resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "revision", "2"),
					),
				},
			},
		})
	})
}

func testEntryDoesNotExist(ctx context.Context, t *testing.T, js jetstream.JetStream, bucket string, key string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		err := testEntryExist(ctx, t, js, bucket, key)(s)
		if err == nil {
			return fmt.Errorf("expected entry %q in bucket %q to not exist", key, bucket)
		}

		return nil
	}
}

func testEntryExist(ctx context.Context, t *testing.T, js jetstream.JetStream, bucket string, key string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		kv, err := js.KeyValue(ctx, bucket)
		if err != nil {
			return err
		}

		_, err = kv.Get(ctx, key)
		if err != nil {
			return err
		}

		return nil
	}
}

func TestResourceKVEntryExternalDeletion(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		js := newJS(t, nc)

		ctx := t.Context()

		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			CheckDestroy:             testEntryDoesNotExist(ctx, t, js, "TEST", "foo"),
			Steps: []resource.TestStep{
				{
					Config: fmt.Sprintf(testKVEntry_basic, nc.ConnectedUrl()),
					Check: resource.ComposeTestCheckFunc(
						testStreamExist(t, mgr, "KV_TEST"),
						testEntryExist(ctx, t, js, "TEST", "foo"),
						resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "key", "foo"),
						resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "value", "bar"),
					),
				},
				{
					Config: fmt.Sprintf(testKVEntry_basic, nc.ConnectedUrl()),
					PreConfig: func() {
						kv, err := js.KeyValue(ctx, "TEST")
						if err != nil {
							t.Fatalf("failed to get bucket for external deletion: %s", err)
						}
						err = kv.Delete(ctx, "foo")
						if err != nil {
							t.Fatalf("failed to externally delete entry: %s", err)
						}
					},
					Check: resource.ComposeTestCheckFunc(
						testStreamExist(t, mgr, "KV_TEST"),
						testEntryExist(ctx, t, js, "TEST", "foo"),
						resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "key", "foo"),
						resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "value", "bar"),
					),
				},
			},
		})
	})
}

func TestResourceKVEntryBucketExternalDeletion(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		js := newJS(t, nc)

		ctx := t.Context()

		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			CheckDestroy:             testEntryDoesNotExist(ctx, t, js, "TEST", "foo"),
			Steps: []resource.TestStep{
				{
					Config: fmt.Sprintf(testKVEntry_basic, nc.ConnectedUrl()),
					Check: resource.ComposeTestCheckFunc(
						testStreamExist(t, mgr, "KV_TEST"),
						testEntryExist(ctx, t, js, "TEST", "foo"),
						resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "key", "foo"),
						resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "value", "bar"),
					),
				},
				{
					Config:    fmt.Sprintf(testKVEntry_basic, nc.ConnectedUrl()),
					PreConfig: func() { removeKV(t, js) },
					Check: resource.ComposeTestCheckFunc(
						testStreamExist(t, mgr, "KV_TEST"),
						testEntryExist(ctx, t, js, "TEST", "foo"),
						resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "key", "foo"),
						resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "value", "bar"),
					),
				},
			},
		})
	})
}

func TestResourceKVEntryImportInvalidID(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			Steps: []resource.TestStep{
				{
					Config:        fmt.Sprintf(testKVEntry_basic, nc.ConnectedUrl()),
					ResourceName:  "jetstream_kv_entry.test_entry",
					ImportState:   true,
					ImportStateId: "foo",
					ExpectError:   regexp.MustCompile(`invalid kv entry id "foo"`),
				},
			},
		})
	})
}

func TestResourceKVEntryRequestFailures(t *testing.T) {
	withJSProxy(t, func(proxy *jsProxy, nc *nats.Conn, mgr *jsm.Manager) {
		js := newJS(t, nc)

		attrs := map[string]any{"bucket": "TEST", "key": "foo", "value": "baz"}
		bucket := func(t *testing.T) { ensureKV(t, js) }
		entry := func(t *testing.T) {
			_, err := ensureKV(t, js).Put(context.Background(), "foo", []byte("bar"))
			checkErr(t, err, "could not put entry: %v", err)
		}

		for _, tc := range []struct {
			name  string
			fn    func(context.Context, *schema.ResourceData, any) diag.Diagnostics
			id    string
			setup func(*testing.T)
		}{
			{"create", resourceKVEntryCreate, "", bucket},
			{"read", resourceKVEntryRead, "JETSTREAM_KV_TEST_ENTRY_foo", entry},
			{"update", resourceKVEntryUpdate, "JETSTREAM_KV_TEST_ENTRY_foo", entry},
			{"delete", resourceKVEntryDelete, "JETSTREAM_KV_TEST_ENTRY_foo", entry},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testEachRequestFails(t, proxy, tc.setup, func(t *testing.T) diag.Diagnostics {
					_, diags := callResource(t, proxy, tc.fn, resourceKVEntry(), tc.id, attrs)
					return diags
				})
			})
		}
	})
}
