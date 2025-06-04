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
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
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
	srv := createJSServer(t)
	defer srv.Shutdown()

	nc, err := nats.Connect(srv.ClientURL())
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

	updateBasicConfig := strings.ReplaceAll(testKVEntry_basic, "bar", "baz")

	resource.Test(t, resource.TestCase{
		ProviderFactories: testJsProviders,
		CheckDestroy:      testEntryDoesNotExist(ctx, t, js, "TEST", "foo"),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testKVEntry_basic, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testBucketExist(t, mgr, "TEST"),
					testEntryExist(ctx, t, js, "TEST", "foo"),
					resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "bucket", "TEST"),
					resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "key", "foo"),
					resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "value", "bar"),
					resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "revision", "1"),
				),
			},
			{
				Config: fmt.Sprintf(updateBasicConfig, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testBucketExist(t, mgr, "TEST"),
					testEntryExist(ctx, t, js, "TEST", "foo"),
					resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "bucket", "TEST"),
					resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "key", "foo"),
					resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "value", "baz"),
					resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "revision", "2"),
				),
			},
		},
	})
}

func testEntryDoesNotExist(ctx context.Context, t *testing.T, js jetstream.JetStream, bucket string, key string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		err := testEntryExist(ctx, t, js, bucket, key)
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
	srv := createJSServer(t)
	defer srv.Shutdown()

	nc, err := nats.Connect(srv.ClientURL())
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
		ProviderFactories: testJsProviders,
		CheckDestroy:      testEntryDoesNotExist(ctx, t, js, "TEST", "foo"),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testKVEntry_basic, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testBucketExist(t, mgr, "TEST"),
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
					testBucketExist(t, mgr, "TEST"),
					testEntryExist(ctx, t, js, "TEST", "foo"),
					resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "key", "foo"),
					resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "value", "bar"),
				),
			},
		},
	})
}

func TestResourceKVEntryBucketExternalDeletion(t *testing.T) {
	srv := createJSServer(t)
	defer srv.Shutdown()

	nc, err := nats.Connect(srv.ClientURL())
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
		ProviderFactories: testJsProviders,
		CheckDestroy:      testEntryDoesNotExist(ctx, t, js, "TEST", "foo"),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testKVEntry_basic, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testBucketExist(t, mgr, "TEST"),
					testEntryExist(ctx, t, js, "TEST", "foo"),
					resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "key", "foo"),
					resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "value", "bar"),
				),
			},
			{
				Config: fmt.Sprintf(testKVEntry_basic, nc.ConnectedUrl()),
				PreConfig: func() {
					err := js.DeleteKeyValue(ctx, "TEST")
					if err != nil {
						t.Fatalf("failed to externally delete bucket: %s", err)
					}
				},
				Check: resource.ComposeTestCheckFunc(
					testBucketExist(t, mgr, "TEST"),
					testEntryExist(ctx, t, js, "TEST", "foo"),
					resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "key", "foo"),
					resource.TestCheckResourceAttr("jetstream_kv_entry.test_entry", "value", "bar"),
				),
			},
		},
	})
}
