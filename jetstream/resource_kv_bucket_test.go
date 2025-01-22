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
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats.go"
)

const testKV_basic = `
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
`

func TestResourceKV(t *testing.T) {
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

	resource.Test(t, resource.TestCase{
		ProviderFactories: testJsProviders,
		CheckDestroy:      testBucketDoesNotExist(t, mgr, "TEST"),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testKV_basic, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testBucketExist(t, mgr, "TEST"),
					resource.TestCheckResourceAttr("jetstream_kv_bucket.test", "name", "TEST"),
					resource.TestCheckResourceAttr("jetstream_kv_bucket.test", "ttl", "60"),
					resource.TestCheckResourceAttr("jetstream_kv_bucket.test", "history", "10"),
					resource.TestCheckResourceAttr("jetstream_kv_bucket.test", "max_value_size", "1024"),
					resource.TestCheckResourceAttr("jetstream_kv_bucket.test", "max_bucket_size", "10240"),
				),
			},
		},
	})
}

func testBucketDoesNotExist(t *testing.T, mgr *jsm.Manager, bucket string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		err := testBucketExist(t, mgr, bucket)
		if err == nil {
			return fmt.Errorf("expected bucket %q to not exist", bucket)
		}

		return nil
	}
}

func testBucketExist(t *testing.T, mgr *jsm.Manager, bucket string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		known, err := mgr.IsKnownStream("KV_" + bucket)
		if err != nil {
			return err
		}

		if !known {
			return fmt.Errorf("bucket %s does not exist", bucket)
		}

		return nil
	}
}
