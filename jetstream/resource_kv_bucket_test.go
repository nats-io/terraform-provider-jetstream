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
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const testKV_basic = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_kv_bucket" "test" {
  name = "TEST"
  ttl = 60
  storage = "memory"
  history = 10
  max_value_size = 1024
  max_bucket_size = 10240
  limit_marker_ttl = 45
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
		ProtoV5ProviderFactories: testJsProviders,
		CheckDestroy:             testBucketDoesNotExist(t, mgr, "TEST"),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testKV_basic, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testBucketExist(t, mgr, "TEST"),
					resource.TestCheckResourceAttr("jetstream_kv_bucket.test", "name", "TEST"),
					resource.TestCheckResourceAttr("jetstream_kv_bucket.test", "ttl", "60"),
					resource.TestCheckResourceAttr("jetstream_kv_bucket.test", "storage", "memory"),
					resource.TestCheckResourceAttr("jetstream_kv_bucket.test", "history", "10"),
					resource.TestCheckResourceAttr("jetstream_kv_bucket.test", "max_value_size", "1024"),
					resource.TestCheckResourceAttr("jetstream_kv_bucket.test", "max_bucket_size", "10240"),
					resource.TestCheckResourceAttr("jetstream_kv_bucket.test", "limit_marker_ttl", "45"),
				),
			},
			{
				ResourceName:      "jetstream_kv_bucket.test",
				ImportState:       true,
				ImportStateVerify: true,
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

func TestResourceKVExternalDeletion(t *testing.T) {
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
		ProtoV5ProviderFactories: testJsProviders,
		CheckDestroy:             testBucketDoesNotExist(t, mgr, "TEST"),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testKV_basic, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testBucketExist(t, mgr, "TEST"),
					resource.TestCheckResourceAttr("jetstream_kv_bucket.test", "name", "TEST"),
				),
			},
			{
				Config: fmt.Sprintf(testKV_basic, nc.ConnectedUrl()),
				PreConfig: func() {
					err := js.DeleteKeyValue(ctx, "TEST")
					if err != nil {
						t.Fatalf("failed to externally delete bucket: %s", err)
					}
				},
				Check: resource.ComposeTestCheckFunc(
					testBucketExist(t, mgr, "TEST"),
					resource.TestCheckResourceAttr("jetstream_kv_bucket.test", "name", "TEST"),
				),
			},
		},
	})
}

const testKVReplicas = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_kv_bucket" "test" {
  name     = "TEST"
  replicas = %d
}
`

func TestResourceKVReplicasUpdate(t *testing.T) {
	servers := createJSCluster(t)

	nc, err := nats.Connect(servers[0].ClientURL())
	if err != nil {
		t.Fatalf("could not connect: %s", err)
	}
	defer nc.Close()

	mgr, err := jsm.New(nc)
	if err != nil {
		t.Fatalf("could not connect: %s", err)
	}

	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: testJsProviders,
		CheckDestroy:             testBucketDoesNotExist(t, mgr, "TEST"),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testKVReplicas, nc.ConnectedUrl(), 3),
				Check: resource.ComposeTestCheckFunc(
					testStreamHasReplicas(t, mgr, "KV_TEST", 3),
					resource.TestCheckResourceAttr("jetstream_kv_bucket.test", "replicas", "3"),
				),
			},
			{
				Config: fmt.Sprintf(testKVReplicas, nc.ConnectedUrl(), 1),
				Check: resource.ComposeTestCheckFunc(
					testStreamHasReplicas(t, mgr, "KV_TEST", 1),
					resource.TestCheckResourceAttr("jetstream_kv_bucket.test", "replicas", "1"),
				),
			},
		},
	})
}

const testKVPlacement = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_kv_bucket" "test" {
  name              = "TEST"
  description       = "%s"
  placement_cluster = "C1"
  placement_tags    = ["az:1"]
}
`

func TestResourceKVPlacement(t *testing.T) {
	servers := createJSCluster(t)

	nc, err := nats.Connect(servers[0].ClientURL())
	if err != nil {
		t.Fatalf("could not connect: %s", err)
	}
	defer nc.Close()

	mgr, err := jsm.New(nc)
	if err != nil {
		t.Fatalf("could not connect: %s", err)
	}

	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: testJsProviders,
		CheckDestroy:             testBucketDoesNotExist(t, mgr, "TEST"),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testKVPlacement, nc.ConnectedUrl(), "one"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("jetstream_kv_bucket.test", "placement_cluster", "C1"),
					resource.TestCheckResourceAttr("jetstream_kv_bucket.test", "placement_tags.0", "az:1"),
				),
			},
			{
				Config: fmt.Sprintf(testKVPlacement, nc.ConnectedUrl(), "two"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("jetstream_kv_bucket.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("jetstream_kv_bucket.test", "placement_cluster", "C1"),
					resource.TestCheckResourceAttr("jetstream_kv_bucket.test", "placement_tags.0", "az:1"),
					testStreamConfig(t, mgr, "KV_TEST", func(cfg api.StreamConfig) error {
						if cfg.Placement == nil || cfg.Placement.Cluster != "C1" {
							return fmt.Errorf("expected placement in cluster C1, got %+v", cfg.Placement)
						}
						return nil
					}),
				),
			},
			{
				ResourceName:      "jetstream_kv_bucket.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

const testKVUnmanagedSettings = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_kv_bucket" "origin" {
  name = "ORIGIN"
}

resource "jetstream_kv_bucket" "test" {
  name    = "TEST"
  history = %d
}
`

func TestResourceKVUpdateKeepsUnmanagedSettings(t *testing.T) {
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
		ProtoV5ProviderFactories: testJsProviders,
		CheckDestroy: resource.ComposeTestCheckFunc(
			testBucketDoesNotExist(t, mgr, "TEST"),
			testBucketDoesNotExist(t, mgr, "ORIGIN"),
		),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testKVUnmanagedSettings, nc.ConnectedUrl(), 5),
			},
			{
				PreConfig: func() {
					updateStreamConfig(t, mgr, "KV_TEST", func(cfg *api.StreamConfig) {
						cfg.RePublish = &api.RePublish{Source: "$KV.TEST.>", Destination: "rp.TEST.>"}
						cfg.Compression = api.S2Compression
						cfg.Sources = []*api.StreamSource{{
							Name:              "KV_ORIGIN",
							SubjectTransforms: []api.SubjectTransformConfig{{Source: "$KV.ORIGIN.>", Destination: "$KV.TEST.>"}},
						}}
					})
				},
				Config: fmt.Sprintf(testKVUnmanagedSettings, nc.ConnectedUrl(), 10),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("jetstream_kv_bucket.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("jetstream_kv_bucket.test", "history", "10"),
					testStreamConfig(t, mgr, "KV_TEST", func(cfg api.StreamConfig) error {
						if cfg.RePublish == nil || cfg.RePublish.Destination != "rp.TEST.>" {
							return fmt.Errorf("expected republish to rp.TEST.>, got %+v", cfg.RePublish)
						}
						if cfg.Compression != api.S2Compression {
							return fmt.Errorf("expected s2 compression, got %v", cfg.Compression)
						}
						if len(cfg.Sources) != 1 || cfg.Sources[0].Name != "KV_ORIGIN" {
							return fmt.Errorf("expected source KV_ORIGIN, got %+v", cfg.Sources)
						}
						return nil
					}),
				),
			},
		},
	})
}
