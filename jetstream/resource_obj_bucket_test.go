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
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	ntf "github.com/synadia-io/orbit.go/ntf-client"
)

const testObj_basic = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_obj_bucket" "test" {
  name = "TEST"
  ttl = 60
  storage = "memory"
  max_bucket_size = 10240
  compression = true
}
`

func TestResourceObj(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			CheckDestroy:             testStreamDoesNotExist(t, mgr, "OBJ_TEST"),
			Steps: []resource.TestStep{
				{
					Config: fmt.Sprintf(testObj_basic, nc.ConnectedUrl()),
					Check: resource.ComposeTestCheckFunc(
						testStreamExist(t, mgr, "OBJ_TEST"),
						resource.TestCheckResourceAttr("jetstream_obj_bucket.test", "name", "TEST"),
						resource.TestCheckResourceAttr("jetstream_obj_bucket.test", "ttl", "60"),
						resource.TestCheckResourceAttr("jetstream_obj_bucket.test", "storage", "memory"),
						resource.TestCheckResourceAttr("jetstream_obj_bucket.test", "max_bucket_size", "10240"),
						resource.TestCheckResourceAttr("jetstream_obj_bucket.test", "compression", "true"),
					),
				},
				{
					ResourceName:      "jetstream_obj_bucket.test",
					ImportState:       true,
					ImportStateVerify: true,
				},
			},
		})
	})
}

func TestResourceObjExternalDeletion(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		js := newJS(t, nc)

		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			CheckDestroy:             testStreamDoesNotExist(t, mgr, "OBJ_TEST"),
			Steps: []resource.TestStep{
				{
					Config: fmt.Sprintf(testObj_basic, nc.ConnectedUrl()),
					Check: resource.ComposeTestCheckFunc(
						testStreamExist(t, mgr, "OBJ_TEST"),
						resource.TestCheckResourceAttr("jetstream_obj_bucket.test", "name", "TEST"),
					),
				},
				{
					Config:    fmt.Sprintf(testObj_basic, nc.ConnectedUrl()),
					PreConfig: func() { removeObj(t, js) },
					Check: resource.ComposeTestCheckFunc(
						testStreamExist(t, mgr, "OBJ_TEST"),
						resource.TestCheckResourceAttr("jetstream_obj_bucket.test", "name", "TEST"),
					),
				},
			},
		})
	})
}

const testObjPlacement = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_obj_bucket" "test" {
  name              = "TEST"
  description       = "%s"
  placement_cluster = "C1"
  placement_tags    = ["az:1"]
}
`

func TestResourceObjPlacement(t *testing.T) {
	withJSCluster(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			CheckDestroy:             testStreamDoesNotExist(t, mgr, "OBJ_TEST"),
			Steps: []resource.TestStep{
				{
					Config: fmt.Sprintf(testObjPlacement, nc.ConnectedUrl(), "one"),
					Check: resource.ComposeTestCheckFunc(
						resource.TestCheckResourceAttr("jetstream_obj_bucket.test", "placement_cluster", "C1"),
						resource.TestCheckResourceAttr("jetstream_obj_bucket.test", "placement_tags.0", "az:1"),
					),
				},
				{
					Config: fmt.Sprintf(testObjPlacement, nc.ConnectedUrl(), "two"),
					ConfigPlanChecks: resource.ConfigPlanChecks{
						PreApply: []plancheck.PlanCheck{
							plancheck.ExpectResourceAction("jetstream_obj_bucket.test", plancheck.ResourceActionUpdate),
						},
					},
					Check: resource.ComposeTestCheckFunc(
						resource.TestCheckResourceAttr("jetstream_obj_bucket.test", "placement_cluster", "C1"),
						resource.TestCheckResourceAttr("jetstream_obj_bucket.test", "placement_tags.0", "az:1"),
						testStreamConfig(t, mgr, "OBJ_TEST", func(cfg api.StreamConfig) error {
							if cfg.Placement == nil || cfg.Placement.Cluster != "C1" {
								return fmt.Errorf("expected placement in cluster C1, got %+v", cfg.Placement)
							}
							return nil
						}),
					),
				},
				{
					ResourceName:      "jetstream_obj_bucket.test",
					ImportState:       true,
					ImportStateVerify: true,
				},
			},
		})
	})
}

const testObjUnmanagedSettings = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_obj_bucket" "test" {
  name        = "TEST"
  description = "%s"
}
`

func TestResourceObjUpdateKeepsUnmanagedSettings(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			CheckDestroy:             testStreamDoesNotExist(t, mgr, "OBJ_TEST"),
			Steps: []resource.TestStep{
				{
					Config: fmt.Sprintf(testObjUnmanagedSettings, nc.ConnectedUrl(), "one"),
				},
				{
					PreConfig: func() {
						updateStreamConfig(t, mgr, "OBJ_TEST", func(cfg *api.StreamConfig) {
							if cfg.Metadata == nil {
								cfg.Metadata = map[string]string{}
							}
							cfg.Metadata["foo"] = "bar"
						})
					},
					Config: fmt.Sprintf(testObjUnmanagedSettings, nc.ConnectedUrl(), "two"),
					ConfigPlanChecks: resource.ConfigPlanChecks{
						PreApply: []plancheck.PlanCheck{
							plancheck.ExpectResourceAction("jetstream_obj_bucket.test", plancheck.ResourceActionUpdate),
						},
					},
					Check: resource.ComposeTestCheckFunc(
						resource.TestCheckResourceAttr("jetstream_obj_bucket.test", "description", "two"),
						testStreamConfig(t, mgr, "OBJ_TEST", func(cfg api.StreamConfig) error {
							if cfg.Metadata["foo"] != "bar" {
								return fmt.Errorf("expected metadata foo=bar, got %v", cfg.Metadata)
							}
							return nil
						}),
					),
				},
			},
		})
	})
}

func TestResourceObjAlreadyExists(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		js := newJS(t, nc)

		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			Steps: []resource.TestStep{
				{
					PreConfig:   func() { ensureObj(t, js) },
					Config:      fmt.Sprintf(testObj_basic, nc.ConnectedUrl()),
					ExpectError: regexp.MustCompile(`bucket TEST already exist`),
				},
			},
		})
	})
}

func TestResourceObjImportInvalidID(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			Steps: []resource.TestStep{
				{
					Config:        fmt.Sprintf(testObj_basic, nc.ConnectedUrl()),
					ResourceName:  "jetstream_obj_bucket.test",
					ImportState:   true,
					ImportStateId: "TEST",
					ExpectError:   regexp.MustCompile(`invalid object store bucket id "TEST"`),
				},
			},
		})
	})
}

var testObjAttrs = map[string]any{"name": "TEST"}

func ensureObj(t *testing.T, js jetstream.JetStream) {
	t.Helper()

	removeObj(t, js)
	_, err := js.CreateObjectStore(context.Background(), jetstream.ObjectStoreConfig{Bucket: "TEST"})
	checkErr(t, err, "could not create object store: %v", err)
}

func removeObj(t *testing.T, js jetstream.JetStream) {
	t.Helper()

	err := js.DeleteObjectStore(context.Background(), "TEST")
	if err != nil && !errors.Is(err, jetstream.ErrStreamNotFound) {
		t.Fatalf("could not delete object store: %v", err)
	}
}

func TestResourceObjRequestFailures(t *testing.T) {
	withJSProxy(t, func(proxy *jsProxy, nc *nats.Conn, mgr *jsm.Manager) {
		js := newJS(t, nc)

		t.Run("create", func(t *testing.T) {
			testEachRequestFails(t, proxy, func(t *testing.T) { removeObj(t, js) }, func(t *testing.T) diag.Diagnostics {
				_, diags := callResource(t, proxy, resourceObjBucketCreate, resourceObjBucket(), "", testObjAttrs)
				return diags
			})
		})

		for _, tc := range []struct {
			name string
			fn   func(context.Context, *schema.ResourceData, any) diag.Diagnostics
		}{
			{"read", resourceObjBucketRead},
			{"update", resourceObjBucketUpdate},
			{"delete", resourceObjBucketDelete},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testEachRequestFails(t, proxy, func(t *testing.T) { ensureObj(t, js) }, func(t *testing.T) diag.Diagnostics {
					_, diags := callResource(t, proxy, tc.fn, resourceObjBucket(), "JETSTREAM_OBJ_TEST", testObjAttrs)
					return diags
				})
			})
		}
	})
}

func TestResourceObjDeleteMissing(t *testing.T) {
	withJSProxy(t, func(proxy *jsProxy, nc *nats.Conn, mgr *jsm.Manager) {
		_, diags := callResource(t, proxy, resourceObjBucketDelete, resourceObjBucket(), "JETSTREAM_OBJ_TEST", testObjAttrs)
		if diags.HasError() {
			t.Fatalf("unexpected error: %v", diags)
		}
	})
}
