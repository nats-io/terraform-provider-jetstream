package jetstream

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats.go"
)

const testStreamConfigBasic = `
provider "jetstream" {
	servers = "%s"
}

resource "jetstream_stream" "test" {
	name = "TEST"
	subjects = ["TEST.*"]
    description = "testing stream"
}`

const testStreamConfigOtherSubjects = `
provider "jetstream" {
	servers = "%s"
}

resource "jetstream_stream" "test" {
	name = "TEST"
	subjects = ["OTHER.*"]
	max_msgs = 10
	max_msgs_per_subject = 2
}
`

const testStreamConfigMirror = `
provider "jetstream" {
	servers = "%s"
}

resource "jetstream_stream" "other" {
	name = "OTHER"
	subjects = ["js.in.OTHER"]
}

resource "jetstream_stream" "test" {
	name = "TEST"
	mirror {
		name = "OTHER"
		filter_subject = "js.in.>"
		start_seq = 11
		external {
			api = "js.api.ext"
            deliver = "js.deliver.ext"
		}
	}
}
`

const testStreamConfigSources = `
provider "jetstream" {
	servers = "%s"
}

resource "jetstream_stream" "other1" {
	name = "OTHER1"
	subjects = ["js.in.OTHER1"]
}

resource "jetstream_stream" "other2" {
	name = "OTHER2"
	subjects = ["js.in.OTHER2"]
}

resource "jetstream_stream" "test" {
	name = "TEST"
	source {
		name = "OTHER1"
		start_seq = 11
	}

	source {
		name = "OTHER2"
		start_seq = 11
	}
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
				Config: fmt.Sprintf(testStreamConfigBasic, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
					testStreamHasSubjects(t, mgr, "TEST", []string{"TEST.*"}),
					resource.TestCheckResourceAttr("jetstream_stream.test", "max_msgs", "-1"),
					resource.TestCheckResourceAttr("jetstream_stream.test", "description", "testing stream"),
					resource.TestCheckResourceAttr("jetstream_stream.test", "deny_delete", "false"),
				),
			},
			{
				Config: fmt.Sprintf(testStreamConfigOtherSubjects, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
					testStreamHasSubjects(t, mgr, "TEST", []string{"OTHER.*"}),
					resource.TestCheckResourceAttr("jetstream_stream.test", "max_msgs", "10"),
					resource.TestCheckResourceAttr("jetstream_stream.test", "max_msgs_per_subject", "2"),
				),
			},
			{
				Config: fmt.Sprintf(testStreamConfigMirror, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
					resource.TestCheckResourceAttr("jetstream_stream.test", "mirror.0.name", "OTHER"),
					resource.TestCheckResourceAttr("jetstream_stream.test", "mirror.0.filter_subject", "js.in.>"),
					resource.TestCheckResourceAttr("jetstream_stream.test", "mirror.0.start_seq", "11"),
					resource.TestCheckResourceAttr("jetstream_stream.test", "mirror.0.external.0.api", "js.api.ext"),
					resource.TestCheckResourceAttr("jetstream_stream.test", "mirror.0.external.0.deliver", "js.deliver.ext"),
					testStreamIsMirrorOf(t, mgr, "TEST", "OTHER"),
					testStreamHasSubjects(t, mgr, "TEST", []string{}),
				),
			},
			{
				Config: fmt.Sprintf(testStreamConfigSources, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
					resource.TestCheckResourceAttr("jetstream_stream.test", "source.0.name", "OTHER1"),
					resource.TestCheckResourceAttr("jetstream_stream.test", "source.1.name", "OTHER2"),
					testStreamIsSourceOf(t, mgr, "TEST", []string{"OTHER1", "OTHER2"}),
					testStreamHasSubjects(t, mgr, "TEST", []string{}),
				),
			},
		},
	})
}
