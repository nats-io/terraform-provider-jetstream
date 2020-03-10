package jetstream

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/nats-io/jsm.go"
)

const testStreamTemplateConfig_basic = `
provider "jetstream" {
	servers = "%s"
}

resource "jetstream_stream_template" "test" {
	name        = "JS_1H"
	subjects    = ["JS.1H.*"]
	storage     = "file"
	max_age     = 60 * 60
	max_streams = 60
}
`

const testStreamTemplateConfig_memory = `
provider "jetstream" {
	servers = "%s"
}

resource "jetstream_stream_template" "test" {
	name        = "JS_1H"
	subjects    = ["JS.1H.*"]
	storage     = "memory"
	max_age     = 60 * 60
	max_streams = 60
}
`

func TestResourceStreamTemplate(t *testing.T) {
	srv := createJSServer(t)
	defer srv.Shutdown()

	jsm.Connect(srv.ClientURL())

	resource.Test(t, resource.TestCase{
		Providers:    testJsProviders,
		CheckDestroy: testStreamTemplateDoesNotExist(t, "JS_1H"),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testStreamTemplateConfig_basic, jsm.Connection().ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamTemplateExist(t, "JS_1H"),
					resource.TestCheckResourceAttr("jetstream_stream_template.test", "storage", "file"),
				),
			},
			{
				Config: fmt.Sprintf(testStreamTemplateConfig_memory, jsm.Connection().ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamTemplateExist(t, "JS_1H"),
					resource.TestCheckResourceAttr("jetstream_stream_template.test", "storage", "memory"),
				),
			},
		},
	})
}
