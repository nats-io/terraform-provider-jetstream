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
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats.go"
)

const testConsumerConfig_basic = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_stream" "test" {
  name     = "TEST"
  subjects = ["TEST.*"]
}

resource "jetstream_consumer" "TEST_C1" {
  stream_id          = jetstream_stream.test.id
  description        = "testing consumer"
  durable_name       = "C1"
  deliver_all        = true
  inactive_threshold = 60
  max_delivery       = 10
  backoff            = [30, 60]
  max_waiting        = 256
  metadata           = {
    foo = "bar"
  }
  max_batch          = 1
}
`

const testConsumerConfig_str10 = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_stream" "test" {
  name     = "TEST"
  subjects = ["TEST.*"]
}

resource "jetstream_consumer" "TEST_C2" {
  stream_id       = jetstream_stream.test.id
  durable_name    = "C2"
  stream_sequence = 10
  max_ack_pending = -1
  filter_subjects = ["TEST.a", "TEST.b"]
  max_waiting     = 10
  max_batch       = 1
}
`

const testConsumerConfig_singleSubject = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_stream" "test" {
  name     = "TEST"
  subjects = ["TEST.*"]
}

resource "jetstream_consumer" "TEST_C3" {
  stream_id        = jetstream_stream.test.id
  durable_name     = "C3"
  stream_sequence  = 10
  max_ack_pending  = 20
  filter_subject   = "TEST.a"
  delivery_subject = "ORDERS.a"
}
`

const testBackoffPedantic = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_stream" "test" {
  name     = "TEST"
  subjects = ["TEST.*"]
}

resource "jetstream_consumer" "TEST_C4" {
  stream_id        = jetstream_stream.test.id
  durable_name     = "C4"
  stream_sequence  = 10
  filter_subject   = "TEST.a"
  delivery_subject = "ORDERS.a"
  ack_wait         = 10
  backoff          = [1,10,20,60]
}
`

const testConsumerLimitsPedantic = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_stream" "test" {
  name               = "TEST"
  subjects           = ["TEST.*"]
  max_ack_pending    = 1
  inactive_threshold = 1
}

resource "jetstream_consumer" "TEST_C5" {
  stream_id        = jetstream_stream.test.id
  durable_name     = "C5"
  stream_sequence  = 10
  filter_subject   = "TEST.a"
  delivery_subject = "ORDERS.a"
}

`

const testConsumerMaxRequestBatchPedantic = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_stream" "test" {
  name     = "TEST"
  subjects = ["TEST.*"]
}

resource "jetstream_consumer" "TEST_C6" {
  stream_id       = jetstream_stream.test.id
  durable_name    = "C5"
  deliver_all     = true
  filter_subject  = "TEST.received"
  max_batch       = 100
}
`

const testFilterSubjectsStage1 = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_stream" "test" {
  name     = "TEST"
  subjects = ["test.>"]
}

resource "jetstream_consumer" "maqs-e" {
  stream_id                = jetstream_stream.test.id
  durable_name             = "maqs-e"
  filter_subjects          = [
								"test.test-component.*.*.*.*.*.eu.>", 
								"test.test-header.*.*.*.*.*.eu.>", 
								"test.test-operation.*.*.*.*.*.eu.>", 
								"test.test-prod-res-tools.*.*.*.*.*.eu.>", 
							 ]
  deliver_all              = true
  delivery_group           = "maqs-e"
  delivery_subject         = "DELIVER.maqs-e"
  ack_policy               = "explicit"
  replay_policy            = "instant"
  max_ack_pending          = 1000
  replicas                 = 1
}
`

const testFilterSubjectsStage2 = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_stream" "test" {
  name     = "TEST"
  subjects = ["test.>"]
}

resource "jetstream_consumer" "maqs-e" {
  stream_id                = jetstream_stream.test.id
  durable_name             = "maqs-e"
  filter_subjects          = [
								"test.test-component.*.*.*.*.*.eu.>", 
								"test.test-header.*.*.*.*.*.eu.>", 
								"test.test-operation.*.*.*.*.*.eu.>", 
								"test.test-prod-res-tools.*.*.*.*.*.eu.>", 
								"test.test-complete.*.*.*.*.*.eu.>"
							 ]
  deliver_all              = true
  delivery_group           = "maqs-e"
  delivery_subject         = "DELIVER.maqs-e"
  ack_policy               = "explicit"
  replay_policy            = "instant"
  max_ack_pending          = 1000
  replicas                 = 1
}`

const testFilterSubjectsStage3 = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_stream" "test" {
  name     = "TEST"
  subjects = ["test.>"]
}

resource "jetstream_consumer" "maqs-e" {
  stream_id                = jetstream_stream.test.id
  durable_name             = "maqs-e"
  filter_subject           = "test.test-complete.*.*.*.*.*.eu.>"
  deliver_all              = true
  delivery_group           = "maqs-e"
  delivery_subject         = "DELIVER.maqs-e"
  ack_policy               = "explicit"
  replay_policy            = "instant"
  max_ack_pending          = 1000
  replicas                 = 1
}`

const testAckPolicies = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_stream" "test" {
  name     = "TEST"
  subjects = ["TEST.*"]
}

resource "jetstream_consumer" "ack_explicit" {
  stream_id    = jetstream_stream.test.id
  durable_name = "ack_explicit"
  deliver_all  = true
  ack_policy   = "explicit"
  max_batch    = 1
}

resource "jetstream_consumer" "ack_all" {
  stream_id    = jetstream_stream.test.id
  durable_name = "ack_all"
  deliver_all  = true
  ack_policy   = "all"
  max_batch    = 1
}

resource "jetstream_consumer" "ack_none" {
  stream_id    = jetstream_stream.test.id
  durable_name = "ack_none"
  deliver_all  = true
  ack_policy   = "none"
  max_batch    = 1
}

resource "jetstream_consumer" "ack_flow_control" {
  stream_id        = jetstream_stream.test.id
  durable_name     = "ack_flow_control"
  deliver_all      = true
  ack_policy       = "flow_control"
  delivery_subject = "DELIVER.fc"
  flow_control     = true
  heartbeat        = 1
  ack_wait         = 0
}
`

func TestConsumerAckPolicies(t *testing.T) {
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
			testConsumerDoesNotExist(t, mgr, "TEST", "ack_explicit"),
			testConsumerDoesNotExist(t, mgr, "TEST", "ack_all"),
			testConsumerDoesNotExist(t, mgr, "TEST", "ack_none"),
			testConsumerDoesNotExist(t, mgr, "TEST", "ack_flow_control"),
		),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testAckPolicies, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
					testConsumerExist(t, mgr, "TEST", "ack_explicit"),
					testConsumerHasAckPolicy(t, mgr, "TEST", "ack_explicit", api.AckExplicit),
					resource.TestCheckResourceAttr("jetstream_consumer.ack_explicit", "ack_policy", "explicit"),
					testConsumerExist(t, mgr, "TEST", "ack_all"),
					testConsumerHasAckPolicy(t, mgr, "TEST", "ack_all", api.AckAll),
					resource.TestCheckResourceAttr("jetstream_consumer.ack_all", "ack_policy", "all"),
					testConsumerExist(t, mgr, "TEST", "ack_none"),
					testConsumerHasAckPolicy(t, mgr, "TEST", "ack_none", api.AckNone),
					resource.TestCheckResourceAttr("jetstream_consumer.ack_none", "ack_policy", "none"),
					testConsumerExist(t, mgr, "TEST", "ack_flow_control"),
					testConsumerHasAckPolicy(t, mgr, "TEST", "ack_flow_control", api.AckFlowControl),
					resource.TestCheckResourceAttr("jetstream_consumer.ack_flow_control", "ack_policy", "flow_control"),
				),
			},
		}})
}

const testPriorityGroups = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_stream" "test" {
  name     = "TEST"
  subjects = ["test.>"]
}

resource "jetstream_consumer" "pgroup" {
  stream_id                = jetstream_stream.test.id
  deliver_all              = true
  durable_name             = "pgroup"
  max_batch                = 1
  priority_groups          = ["a", "b", "c"]
  priority_timeout         = 20
  priority_policy          = "pinned_client"
}`

func TestFilterSubjects(t *testing.T) {
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
		CheckDestroy:             testConsumerDoesNotExist(t, mgr, "TEST", "maqs-e"),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testFilterSubjectsStage1, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
					testConsumerExist(t, mgr, "TEST", "maqs-e"),
					testConsumerHasFilterSubjects(t, mgr, "TEST", "maqs-e", []string{
						"test.test-component.*.*.*.*.*.eu.>",
						"test.test-header.*.*.*.*.*.eu.>",
						"test.test-operation.*.*.*.*.*.eu.>",
						"test.test-prod-res-tools.*.*.*.*.*.eu.>",
					}),
				),
			},
			{
				Config: fmt.Sprintf(testFilterSubjectsStage2, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
					testConsumerExist(t, mgr, "TEST", "maqs-e"),
					testConsumerHasFilterSubjects(t, mgr, "TEST", "maqs-e", []string{
						"test.test-component.*.*.*.*.*.eu.>",
						"test.test-header.*.*.*.*.*.eu.>",
						"test.test-operation.*.*.*.*.*.eu.>",
						"test.test-prod-res-tools.*.*.*.*.*.eu.>",
						"test.test-complete.*.*.*.*.*.eu.>",
					}),
				),
			},
			{
				Config: fmt.Sprintf(testFilterSubjectsStage3, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
					testConsumerExist(t, mgr, "TEST", "maqs-e"),
					testConsumerHasFilterSubjects(t, mgr, "TEST", "maqs-e", []string{}),
					testConsumerHasFilterSubject(t, mgr, "TEST", "maqs-e", "test.test-complete.*.*.*.*.*.eu.>"),
				),
			},
		}})
}

func TestResourceConsumer(t *testing.T) {
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

	updateBasicConfig := strings.ReplaceAll(testConsumerConfig_basic, "testing consumer", "new description")
	updateBasicConfig = fmt.Sprintf(strings.ReplaceAll(updateBasicConfig, "bar", "baz"), nc.ConnectedUrl())

	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: testJsProviders,
		CheckDestroy:             testConsumerDoesNotExist(t, mgr, "TEST", "C1"),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testConsumerConfig_basic, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
					testConsumerExist(t, mgr, "TEST", "C1"),
					testConsumerHasMetadata(t, mgr, "TEST", "C1", map[string]string{"foo": "bar"}),
					resource.TestCheckResourceAttr("jetstream_consumer.TEST_C1", "stream_sequence", "0"),
					resource.TestCheckResourceAttr("jetstream_consumer.TEST_C1", "description", "testing consumer"),
					resource.TestCheckResourceAttr("jetstream_consumer.TEST_C1", "inactive_threshold", "60"),
				),
			},
			{
				ResourceName:      "jetstream_consumer.TEST_C1",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: updateBasicConfig,
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
					testConsumerExist(t, mgr, "TEST", "C1"),
					testConsumerHasMetadata(t, mgr, "TEST", "C1", map[string]string{"foo": "baz"}),
					resource.TestCheckResourceAttr("jetstream_consumer.TEST_C1", "stream_sequence", "0"),
					resource.TestCheckResourceAttr("jetstream_consumer.TEST_C1", "description", "new description"),
					resource.TestCheckResourceAttr("jetstream_consumer.TEST_C1", "inactive_threshold", "60"),
					resource.TestCheckResourceAttr("jetstream_consumer.TEST_C1", "max_waiting", "256"),
				),
			},
			{
				Config: fmt.Sprintf(testConsumerConfig_str10, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
					testConsumerExist(t, mgr, "TEST", "C2"),
					testConsumerHasFilterSubjects(t, mgr, "TEST", "C2", []string{"TEST.a", "TEST.b"}),
					resource.TestCheckResourceAttr("jetstream_consumer.TEST_C2", "stream_sequence", "10"),
					resource.TestCheckResourceAttr("jetstream_consumer.TEST_C2", "max_ack_pending", "-1"),
					resource.TestCheckResourceAttr("jetstream_consumer.TEST_C2", "inactive_threshold", "0"),
				),
			},
			{
				Config: fmt.Sprintf(testConsumerConfig_singleSubject, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
					testConsumerExist(t, mgr, "TEST", "C3"),
					testConsumerHasFilterSubjects(t, mgr, "TEST", "C3", []string{}),
					resource.TestCheckResourceAttr("jetstream_consumer.TEST_C3", "filter_subject", "TEST.a"),
				),
			},
			{
				Config:      fmt.Sprintf(testBackoffPedantic, nc.ConnectedUrl()),
				ExpectError: regexp.MustCompile(`first backoff value has to equal batch AckWait \(10157\)`),
			},
			{
				Config:      fmt.Sprintf(testConsumerLimitsPedantic, nc.ConnectedUrl()),
				ExpectError: regexp.MustCompile(`inactive_threshold must be set if it's configured in stream limits \(10157\)`),
			},
			{
				Config:      fmt.Sprintf(testConsumerMaxRequestBatchPedantic, nc.ConnectedUrl()),
				ExpectError: regexp.MustCompile(`consumer max request batch exceeds server limit of 1 \(10125\)`),
			},
			{
				Config: fmt.Sprintf(testPriorityGroups, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
					testConsumerExist(t, mgr, "TEST", "pgroup"),
					resource.TestCheckResourceAttr("jetstream_consumer.pgroup", "priority_groups.0", "a"),
					resource.TestCheckResourceAttr("jetstream_consumer.pgroup", "priority_groups.1", "b"),
					resource.TestCheckResourceAttr("jetstream_consumer.pgroup", "priority_groups.2", "c"),
					resource.TestCheckResourceAttr("jetstream_consumer.pgroup", "priority_policy", "pinned_client"),
					resource.TestCheckResourceAttr("jetstream_consumer.pgroup", "priority_timeout", "20"),
				),
			},
		},
	})
}

const testConsumerPushAckNone = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_stream" "test" {
  name     = "TEST"
  subjects = ["TEST.*"]
}

resource "jetstream_consumer" "push_none" {
  stream_id        = jetstream_stream.test.id
  durable_name     = "PUSHNONE"
  delivery_subject = "push.none"
  deliver_all      = true
  ack_policy       = "none"
  %s
}

resource "jetstream_stream" "limited" {
  name            = "LIMITED"
  subjects        = ["LIMITED.*"]
  max_ack_pending = 100
}

resource "jetstream_consumer" "limited_push_none" {
  stream_id        = jetstream_stream.limited.id
  durable_name     = "PUSHNONE"
  delivery_subject = "push.limited"
  deliver_all      = true
  ack_policy       = "none"
}
`

func TestConsumerPushAckNone(t *testing.T) {
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
			testConsumerDoesNotExist(t, mgr, "TEST", "PUSHNONE"),
			testConsumerDoesNotExist(t, mgr, "LIMITED", "PUSHNONE"),
		),
		Steps: []resource.TestStep{
			{
				Config:      fmt.Sprintf(testConsumerPushAckNone, nc.ConnectedUrl(), "max_ack_pending = 10"),
				ExpectError: regexp.MustCompile(`max_ack_pending cannot be set on a push consumer with ack_policy none`),
			},
			{
				Config: fmt.Sprintf(testConsumerPushAckNone, nc.ConnectedUrl(), ""),
				Check: resource.ComposeTestCheckFunc(
					testConsumerExist(t, mgr, "TEST", "PUSHNONE"),
					testConsumerHasMaxAckPending(t, mgr, "TEST", "PUSHNONE", -1),
					testConsumerExist(t, mgr, "LIMITED", "PUSHNONE"),
					testConsumerHasMaxAckPending(t, mgr, "LIMITED", "PUSHNONE", -1),
				),
			},
			{
				ResourceName:      "jetstream_consumer.push_none",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

const testConsumerMaxWaiting = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_stream" "test" {
  name     = "TEST"
  subjects = ["TEST.*"]
}

resource "jetstream_consumer" "pull" {
  stream_id    = jetstream_stream.test.id
  durable_name = "PULL"
  deliver_all  = true
  max_batch    = 1
  max_waiting  = %d
}
`

func TestConsumerMaxWaitingForcesReplace(t *testing.T) {
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
		CheckDestroy:             testConsumerDoesNotExist(t, mgr, "TEST", "PULL"),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testConsumerMaxWaiting, nc.ConnectedUrl(), 10),
			},
			{
				Config: fmt.Sprintf(testConsumerMaxWaiting, nc.ConnectedUrl(), 20),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("jetstream_consumer.pull", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.TestCheckResourceAttr("jetstream_consumer.pull", "max_waiting", "20"),
			},
			{
				Config: fmt.Sprintf(testConsumerMaxWaiting, nc.ConnectedUrl(), 0),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("jetstream_consumer.pull", plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

const testConsumerAllAttributes = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_stream" "test" {
  name     = "TEST"
  subjects = ["TEST.*"]
}

resource "jetstream_consumer" "test" {
  stream_id    = jetstream_stream.test.id
  durable_name = "C1"
  deliver_all  = true
  max_batch    = 1
  headers_only = %t
  max_bytes    = %d
  max_expires  = %d
  sample_freq  = %d
}
`

func TestConsumerAllAttributes(t *testing.T) {
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
		CheckDestroy:             testConsumerDoesNotExist(t, mgr, "TEST", "C1"),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testConsumerAllAttributes, nc.ConnectedUrl(), true, 1024, 30, 50),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("jetstream_consumer.test", "headers_only", "true"),
					resource.TestCheckResourceAttr("jetstream_consumer.test", "max_bytes", "1024"),
					resource.TestCheckResourceAttr("jetstream_consumer.test", "max_expires", "30"),
					resource.TestCheckResourceAttr("jetstream_consumer.test", "sample_freq", "50"),
				),
			},
			{
				Config: fmt.Sprintf(testConsumerAllAttributes, nc.ConnectedUrl(), false, 2048, 60, 75),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("jetstream_consumer.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("jetstream_consumer.test", "headers_only", "false"),
					resource.TestCheckResourceAttr("jetstream_consumer.test", "max_bytes", "2048"),
					resource.TestCheckResourceAttr("jetstream_consumer.test", "max_expires", "60"),
					resource.TestCheckResourceAttr("jetstream_consumer.test", "sample_freq", "75"),
				),
			},
			{
				ResourceName:      "jetstream_consumer.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

const testConsumerForceNew = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_stream" "test" {
  name     = "TEST"
  subjects = ["TEST.*"]
}

resource "jetstream_stream" "other" {
  name     = "OTHER"
  subjects = ["OTHER.*"]
}

resource "jetstream_consumer" "test" {
%s
}
`

func TestConsumerForceNewAttributes(t *testing.T) {
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

	const pull = "  stream_id = jetstream_stream.test.id\n  durable_name = \"C1\"\n  max_batch = 1\n"
	const pullAll = pull + "  deliver_all = true\n"
	const push = "  stream_id = jetstream_stream.test.id\n  durable_name = \"C1\"\n  delivery_subject = \"push.c1\"\n  deliver_all = true\n"

	cases := []struct {
		attr   string
		before string
		after  string
		want   string
	}{
		{"stream_id", pullAll, "  stream_id = jetstream_stream.other.id\n  durable_name = \"C1\"\n  max_batch = 1\n  deliver_all = true\n", "JETSTREAM_STREAM_OTHER"},
		{"durable_name", pullAll, "  stream_id = jetstream_stream.test.id\n  durable_name = \"C2\"\n  max_batch = 1\n  deliver_all = true\n", "C2"},
		{"deliver_new", pullAll, pull + "  deliver_new = true", "true"},
		{"deliver_last", pullAll, pull + "  deliver_last = true", "true"},
		{"deliver_last_per_subject", pullAll + "  filter_subject = \"TEST.a\"", pull + "  filter_subject = \"TEST.a\"\n  deliver_last_per_subject = true", "true"},
		{"stream_sequence", pull + "  stream_sequence = 10", pull + "  stream_sequence = 20", "20"},
		{"start_time", pull + "  start_time = \"2025-01-01T00:00:00Z\"", pull + "  start_time = \"2025-06-01T00:00:00Z\"", "2025-06-01T00:00:00Z"},
		{"ack_policy", pullAll + "  ack_policy = \"explicit\"", pullAll + "  ack_policy = \"all\"", "all"},
		{"replay_policy", pullAll + "  replay_policy = \"instant\"", pullAll + "  replay_policy = \"original\"", "original"},
		{"memory", pullAll + "  memory = false", pullAll + "  memory = true", "true"},
		{"replicas", pullAll + "  replicas = 0", pullAll + "  replicas = 1", "1"},
		{"priority_policy", pullAll + "  priority_policy = \"overflow\"\n  priority_groups = [\"A\"]", pullAll + "  priority_policy = \"pinned_client\"\n  priority_groups = [\"A\"]\n  priority_timeout = 120", "pinned_client"},
		{"priority_groups.0", pullAll + "  priority_policy = \"overflow\"\n  priority_groups = [\"A\"]", pullAll + "  priority_policy = \"overflow\"\n  priority_groups = [\"B\"]", "B"},
		{"priority_timeout", pullAll + "  priority_policy = \"pinned_client\"\n  priority_groups = [\"A\"]\n  priority_timeout = 60", pullAll + "  priority_policy = \"pinned_client\"\n  priority_groups = [\"A\"]\n  priority_timeout = 120", "120"},
		{"delivery_group", push, push + "  delivery_group = \"group\"", "group"},
		{"ratelimit", push + "  ratelimit = 0", push + "  ratelimit = 1000000", "1000000"},
		{"heartbeat", push + "  heartbeat = 0", push + "  heartbeat = 5", "5"},
		{"flow_control", push + "  heartbeat = 5\n  flow_control = false", push + "  heartbeat = 5\n  flow_control = true", "true"},
	}

	for _, c := range cases {
		t.Run(c.attr, func(t *testing.T) {
			resource.Test(t, resource.TestCase{
				ProtoV5ProviderFactories: testJsProviders,
				CheckDestroy: resource.ComposeTestCheckFunc(
					testConsumerDoesNotExist(t, mgr, "TEST", "C1"),
					testConsumerDoesNotExist(t, mgr, "TEST", "C2"),
					testConsumerDoesNotExist(t, mgr, "OTHER", "C1"),
				),
				Steps: []resource.TestStep{
					{
						Config: fmt.Sprintf(testConsumerForceNew, nc.ConnectedUrl(), c.before),
					},
					{
						Config: fmt.Sprintf(testConsumerForceNew, nc.ConnectedUrl(), c.after),
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply: []plancheck.PlanCheck{
								plancheck.ExpectResourceAction("jetstream_consumer.test", plancheck.ResourceActionReplace),
							},
						},
						Check: resource.TestCheckResourceAttr("jetstream_consumer.test", c.attr, c.want),
					},
					{
						ResourceName:      "jetstream_consumer.test",
						ImportState:       true,
						ImportStateVerify: true,
					},
				},
			})
		})
	}
}
