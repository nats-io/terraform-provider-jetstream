package jetstream

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/nats-io/jsm.go"
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

	jsm.Connect(srv.ClientURL())

	resource.Test(t, resource.TestCase{
		Providers:    testJsProviders,
		CheckDestroy: testStreamDoesNotExist(t, "TEST"),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testStreamConfig_basic, jsm.Connection().ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, "TEST"),
					testStreamHasSubjects(t, "TEST", []string{"TEST.*"}),
					resource.TestCheckResourceAttr("jetstream_stream.test", "max_msgs", "-1"),
				),
			},
			{
				Config: fmt.Sprintf(testStreamConfig_OtherSubjects, jsm.Connection().ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, "TEST"),
					testStreamHasSubjects(t, "TEST", []string{"OTHER.*"}),
					resource.TestCheckResourceAttr("jetstream_stream.test", "max_msgs", "10"),
				),
			},
		},
	})
}
