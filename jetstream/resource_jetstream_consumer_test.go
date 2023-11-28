package jetstream

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
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
  metadata           = {
    foo = "bar"
  }
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
  max_ack_pending = 20
  filter_subjects = ["TEST.a", "TEST.b"]
}
`

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
		Providers:    testJsProviders,
		CheckDestroy: testConsumerDoesNotExist(t, mgr, "TEST", "C1"),
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
				),
			},
			{
				Config: fmt.Sprintf(testConsumerConfig_str10, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
					testConsumerExist(t, mgr, "TEST", "C2"),
					testConsumerHasFilterSubjects(t, mgr, "TEST", "C2", []string{"TEST.a", "TEST.b"}),
					resource.TestCheckResourceAttr("jetstream_consumer.TEST_C2", "stream_sequence", "10"),
					resource.TestCheckResourceAttr("jetstream_consumer.TEST_C2", "max_ack_pending", "20"),
					resource.TestCheckResourceAttr("jetstream_consumer.TEST_C2", "inactive_threshold", "0"),
				),
			},
		},
	})
}
