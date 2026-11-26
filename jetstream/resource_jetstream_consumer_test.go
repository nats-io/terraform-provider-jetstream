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
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats.go"
	ntf "github.com/synadia-io/orbit.go/ntf-client"
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
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
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
	})
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
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
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
	})
}

func TestResourceConsumer(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
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
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
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
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
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
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
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
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
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
	})
}

const testConsumerPriority = `
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
%s
}
`

func TestConsumerPriorityValidation(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			CheckDestroy:             testStreamDoesNotExist(t, mgr, "TEST"),
			Steps: []resource.TestStep{
				{
					Config:      fmt.Sprintf(testConsumerPriority, nc.ConnectedUrl(), `  priority_policy = "overflow"`),
					ExpectError: regexp.MustCompile("setting a priority_policy other than `none` requires priority_groups to be set"),
				},
				{
					Config:      fmt.Sprintf(testConsumerPriority, nc.ConnectedUrl(), `  priority_groups = ["A"]`),
					ExpectError: regexp.MustCompile(`setting priority_groups requires a priority_policy to be set and not be 'none'`),
				},
				{
					Config:      fmt.Sprintf(testConsumerPriority, nc.ConnectedUrl(), "  priority_policy = \"overflow\"\n  priority_groups = [\"A\"]\n  priority_timeout = 60"),
					ExpectError: regexp.MustCompile(`setting priority_timeout requires priority_policy to be set to 'pinned_client'`),
				},
				{
					Config: fmt.Sprintf(testConsumerPriority, nc.ConnectedUrl(), "  max_batch = 1\n  priority_policy = jetstream_stream.test.id == \"\" ? \"none\" : \"prioritized\"\n  priority_groups = [\"A\"]"),
					Check: resource.ComposeTestCheckFunc(
						testConsumerExist(t, mgr, "TEST", "C1"),
						resource.TestCheckResourceAttr("jetstream_consumer.test", "priority_policy", "prioritized"),
					),
				},
			},
		})
	})
}

const testConsumerWithStream = `
provider "jetstream" {
  servers = "%s"
}

resource "jetstream_stream" "test" {
  name     = "TEST"
  subjects = ["TEST.*"]
}

resource "jetstream_consumer" "test" {
  stream_id   = %s
  deliver_all = true
  max_batch   = 1
%s
}
`

func TestConsumerConfigErrors(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		longDescription := fmt.Sprintf("  durable_name = \"C1\"\n  description = %q", strings.Repeat("x", 4097))

		for _, tc := range []struct {
			name     string
			streamID string
			config   string
			err      string
		}{
			{"invalid durable name", "jetstream_stream.test.id", `  durable_name = "C.1"`, `durable_name`},
			{"invalid stream id", `"TEST"`, `  durable_name = "C1"`, `invalid stream id "TEST"`},
			{"long description", "jetstream_stream.test.id", longDescription, `description: String length must be less than or equal to 4096`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				resource.Test(t, resource.TestCase{
					ProtoV5ProviderFactories: testJsProviders,
					Steps: []resource.TestStep{
						{
							Config:      fmt.Sprintf(testConsumerWithStream, nc.ConnectedUrl(), tc.streamID, tc.config),
							ExpectError: regexp.MustCompile(tc.err),
						},
					},
				})
			})
		}

		t.Run("long description on update", func(t *testing.T) {
			resource.Test(t, resource.TestCase{
				ProtoV5ProviderFactories: testJsProviders,
				Steps: []resource.TestStep{
					{
						Config: fmt.Sprintf(testConsumerWithStream, nc.ConnectedUrl(), "jetstream_stream.test.id", `  durable_name = "C1"`),
					},
					{
						Config:      fmt.Sprintf(testConsumerWithStream, nc.ConnectedUrl(), "jetstream_stream.test.id", longDescription),
						ExpectError: regexp.MustCompile(`description: String length must be less than or equal to 4096`),
					},
				},
			})
		})
	})
}

func TestConsumerSingleFilterSubject(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			CheckDestroy:             testStreamDoesNotExist(t, mgr, "TEST"),
			Steps: []resource.TestStep{
				{
					Config: fmt.Sprintf(testConsumerWithStream, nc.ConnectedUrl(), "jetstream_stream.test.id", "  durable_name = \"C1\"\n  filter_subjects = [\"TEST.a\"]"),
					Check: resource.ComposeTestCheckFunc(
						testConsumerHasFilterSubject(t, mgr, "TEST", "C1", "TEST.a"),
						testConsumerHasFilterSubjects(t, mgr, "TEST", "C1", nil),
						resource.TestCheckResourceAttr("jetstream_consumer.test", "filter_subjects.0", "TEST.a"),
						resource.TestCheckResourceAttr("jetstream_consumer.test", "filter_subject", ""),
					),
				},
			},
		})
	})
}

func TestConsumerUnsupportedAPILevel(t *testing.T) {
	withJSProxy(t, func(proxy *jsProxy, nc *nats.Conn, mgr *jsm.Manager) {
		prioritized := "  durable_name = \"C1\"\n  priority_policy = \"prioritized\"\n  priority_groups = [\"one\"]"

		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			CheckDestroy:             testStreamDoesNotExist(t, mgr, "TEST"),
			Steps: []resource.TestStep{
				{
					PreConfig:   func() { proxy.setAPILevel(1) },
					Config:      fmt.Sprintf(testConsumerWithStream, proxy.url, "jetstream_stream.test.id", prioritized),
					ExpectError: regexp.MustCompile(`unsupported api level: 1. Requires NATS API level 2 or newer`),
				},
				{
					PreConfig: proxy.clear,
					Config:    fmt.Sprintf(testConsumerWithStream, proxy.url, "jetstream_stream.test.id", `  durable_name = "C1"`),
				},
			},
		})
	})
}

func TestConsumerInvalidSampleFrequency(t *testing.T) {
	withJSProxy(t, func(proxy *jsProxy, nc *nats.Conn, mgr *jsm.Manager) {
		config := fmt.Sprintf(testConsumerWithStream, proxy.url, "jetstream_stream.test.id", `  durable_name = "C1"`)

		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			CheckDestroy:             testStreamDoesNotExist(t, mgr, "TEST"),
			Steps: []resource.TestStep{
				{
					Config: config,
					Check:  resource.TestCheckResourceAttr("jetstream_consumer.test", "sample_freq", "0"),
				},
				{
					PreConfig: func() {
						proxy.rewriteSubject("$JS.API.CONSUMER.INFO.TEST.C1", func(info map[string]any) {
							if cfg, ok := info["config"].(map[string]any); ok {
								cfg["sample_freq"] = "often"
							}
						})
					},
					Config:      config,
					ExpectError: regexp.MustCompile(`failed to parse consumer sampling configuration`),
				},
				{
					PreConfig: proxy.clear,
					Config:    config,
				},
			},
		})
	})
}

func TestConsumerExternalDeletion(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		config := fmt.Sprintf(testConsumerWithStream, nc.ConnectedUrl(), "jetstream_stream.test.id", `  durable_name = "C1"`)

		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			CheckDestroy:             testStreamDoesNotExist(t, mgr, "TEST"),
			Steps: []resource.TestStep{
				{
					Config: config,
					Check:  testConsumerExist(t, mgr, "TEST", "C1"),
				},
				{
					PreConfig: func() {
						cons, err := mgr.LoadConsumer("TEST", "C1")
						checkErr(t, err, "could not load consumer: %v", err)
						err = cons.Delete()
						checkErr(t, err, "could not delete consumer: %v", err)
					},
					Config: config,
					Check:  testConsumerExist(t, mgr, "TEST", "C1"),
				},
				{
					PreConfig: func() { removeStream(t, mgr, "TEST") },
					Config:    config,
					Check:     testConsumerExist(t, mgr, "TEST", "C1"),
				},
			},
		})
	})
}

func TestConsumerImportInvalidID(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, _ *ntf.Instance) {
		resource.Test(t, resource.TestCase{
			ProtoV5ProviderFactories: testJsProviders,
			Steps: []resource.TestStep{
				{
					Config:        fmt.Sprintf(testConsumerWithStream, nc.ConnectedUrl(), "jetstream_stream.test.id", `  durable_name = "C1"`),
					ResourceName:  "jetstream_consumer.test",
					ImportState:   true,
					ImportStateId: "C1",
					ExpectError:   regexp.MustCompile(`invalid consumer id "C1"`),
				},
			},
		})
	})
}

var testConsumerAttrs = map[string]any{"stream_id": "JETSTREAM_STREAM_TEST", "durable_name": "C1", "deliver_all": true}

const testConsumerID = "JETSTREAM_STREAM_TEST_CONSUMER_C1"

func ensureConsumer(t *testing.T, mgr *jsm.Manager) {
	t.Helper()

	ensureStream(t, mgr)
	_, err := mgr.NewConsumerFromDefault("TEST", api.ConsumerConfig{Durable: "C1", AckPolicy: api.AckExplicit, DeliverPolicy: api.DeliverAll})
	checkErr(t, err, "could not create consumer: %v", err)
}

func TestConsumerRequestFailures(t *testing.T) {
	withJSProxy(t, func(proxy *jsProxy, nc *nats.Conn, mgr *jsm.Manager) {
		t.Run("create", func(t *testing.T) {
			testEachRequestFails(t, proxy, func(t *testing.T) { ensureStream(t, mgr) }, func(t *testing.T) diag.Diagnostics {
				_, diags := callResource(t, proxy, resourceConsumerCreate, resourceConsumer(), "", testConsumerAttrs)
				return diags
			})
		})

		for _, tc := range []struct {
			name string
			fn   func(context.Context, *schema.ResourceData, any) diag.Diagnostics
		}{
			{"read", resourceConsumerRead},
			{"update", resourceConsumerUpdate},
			{"delete", resourceConsumerDelete},
		} {
			t.Run(tc.name, func(t *testing.T) {
				testEachRequestFails(t, proxy, func(t *testing.T) { ensureConsumer(t, mgr) }, func(t *testing.T) diag.Diagnostics {
					_, diags := callResource(t, proxy, tc.fn, resourceConsumer(), testConsumerID, testConsumerAttrs)
					return diags
				})
			})
		}
	})
}

func TestConsumerMissingOnServer(t *testing.T) {
	withJSProxy(t, func(proxy *jsProxy, nc *nats.Conn, mgr *jsm.Manager) {
		for _, tc := range []struct {
			name string
			fn   func(context.Context, *schema.ResourceData, any) diag.Diagnostics
		}{
			{"update", resourceConsumerUpdate},
			{"delete", resourceConsumerDelete},
		} {
			t.Run(tc.name, func(t *testing.T) {
				ensureStream(t, mgr)

				d, diags := callResource(t, proxy, tc.fn, resourceConsumer(), testConsumerID, testConsumerAttrs)
				if diags.HasError() {
					t.Fatalf("unexpected error: %v", diags)
				}
				if d.Id() != "" {
					t.Fatalf("expected the id to be cleared, got %q", d.Id())
				}
			})
		}
	})
}

func TestConsumerUpdateUnsupportedAPILevel(t *testing.T) {
	withJSProxy(t, func(proxy *jsProxy, nc *nats.Conn, mgr *jsm.Manager) {
		ensureConsumer(t, mgr)
		proxy.setAPILevel(1)

		attrs := map[string]any{"stream_id": "JETSTREAM_STREAM_TEST", "durable_name": "C1", "deliver_all": true, "priority_policy": "prioritized", "priority_groups": []any{"one"}}
		_, diags := callResource(t, proxy, resourceConsumerUpdate, resourceConsumer(), testConsumerID, attrs)
		if !diags.HasError() || diags[0].Summary != "unsupported api level: 1. Requires NATS API level 2 or newer" {
			t.Fatalf("expected an unsupported api level error, got %v", diags)
		}
	})
}

func TestConsumerInvalidID(t *testing.T) {
	withJSProxy(t, func(proxy *jsProxy, nc *nats.Conn, mgr *jsm.Manager) {
		_, diags := callResource(t, proxy, resourceConsumerDelete, resourceConsumer(), "C1", testConsumerAttrs)
		if !diags.HasError() || diags[0].Summary != `invalid consumer id "C1"` {
			t.Fatalf("expected an invalid consumer id error, got %v", diags)
		}

		attrs := map[string]any{"stream_id": "TEST", "durable_name": "C1", "deliver_all": true}
		_, diags = callResource(t, proxy, resourceConsumerUpdate, resourceConsumer(), testConsumerID, attrs)
		if !diags.HasError() || diags[0].Summary != `invalid stream id "TEST"` {
			t.Fatalf("expected an invalid stream id error, got %v", diags)
		}
	})
}
