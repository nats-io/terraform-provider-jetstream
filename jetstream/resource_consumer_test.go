package jetstream

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/nats-io/jsm.go"
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
  stream_id      = jetstream_stream.test.id
  durable_name   = "C1"
  deliver_all    = true
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
}
`

func TestResourceConsumer(t *testing.T) {
	srv := createJSServer(t)
	defer srv.Shutdown()

	jsm.Connect(srv.ClientURL())

	resource.Test(t, resource.TestCase{
		Providers:    testJsProviders,
		CheckDestroy: testConsumerDoesNotExist(t, "TEST", "C1"),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testConsumerConfig_basic, jsm.Connection().ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, "TEST"),
					testConsumerExist(t, "TEST", "C1"),
					resource.TestCheckResourceAttr("jetstream_consumer.TEST_C1", "stream_sequence", "0"),
				),
			},
			{
				Config: fmt.Sprintf(testConsumerConfig_str10, jsm.Connection().ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, "TEST"),
					testConsumerExist(t, "TEST", "C2"),
					resource.TestCheckResourceAttr("jetstream_consumer.TEST_C2", "stream_sequence", "10"),
				),
			},
		},
	})
}
