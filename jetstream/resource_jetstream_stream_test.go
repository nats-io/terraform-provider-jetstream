package jetstream

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats.go"
)

const testStreamConfig_basic = `
provider "jetstream" {
	servers = "%s"
}

resource "jetstream_stream" "test" {
	name = "TEST"
	subjects = ["TEST.*"]
}`

const testStreamConfig_OtherSubjects = `
provider "jetstream" {
	servers = "%s"
}

resource "jetstream_stream" "test" {
	name = "TEST"
	subjects = ["OTHER.*"]
	max_msgs = 10
}
`

func TestResourceStream(t *testing.T) {
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
		Providers:    testJsProviders,
		CheckDestroy: testStreamDoesNotExist(t, mgr, "TEST"),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testStreamConfig_basic, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
					testStreamHasSubjects(t, mgr, "TEST", []string{"TEST.*"}),
					resource.TestCheckResourceAttr("jetstream_stream.test", "max_msgs", "-1"),
				),
			},
			{
				Config: fmt.Sprintf(testStreamConfig_OtherSubjects, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
					testStreamHasSubjects(t, mgr, "TEST", []string{"OTHER.*"}),
					resource.TestCheckResourceAttr("jetstream_stream.test", "max_msgs", "10"),
				),
			},
		},
	})
}
