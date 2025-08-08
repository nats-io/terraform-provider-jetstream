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

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/nats-io/jsm.go"
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
		ProviderFactories: testJsProviders,
		CheckDestroy:      testConsumerDoesNotExist(t, mgr, "TEST", "maqs-e"),
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
		ProviderFactories: testJsProviders,
		CheckDestroy:      testConsumerDoesNotExist(t, mgr, "TEST", "C1"),
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
