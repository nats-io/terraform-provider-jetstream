package jetstream

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
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
    compression = "s2"
	metadata = {
		foo = "bar"
	}
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

const typeStreamConfigMirrorTransformed = `
provider "jetstream" {
	servers = "%s"
}

resource "jetstream_stream" "other" {
	name = "OTHER"
	subjects = ["js.in.OTHER.>"]
	allow_direct = true
}

resource "jetstream_stream" "mirror_transform_test" {
	name = "MIRROR_TRANSFORM_TEST"
    description = "typeStreamConfigMirrorTransformed"
	mirror_direct = true
	mirror {
		name = "OTHER"
		start_seq = 11

		subject_transform {
			source = "js.in.OTHER.1.>"
			destination = "1.>"
		}

		subject_transform {
			source = "js.in.OTHER.2.>"
			destination = "2.>"
		}
	}
}
`

const typeStreamConfigSourcesTransformed = `
provider "jetstream" {
	servers = "%s"
}

resource "jetstream_stream" "other1" {
	name = "OTHER1"
	subjects = ["js.in.OTHER1.>"]
}

resource "jetstream_stream" "other2" {
	name = "OTHER2"
	subjects = ["js.in.OTHER2.>"]
}

resource "jetstream_stream" "source_transform_test" {
	name = "SOURCE_TRANSFORM_TEST"
    description = "typeStreamConfigSourcesTransformed"
	source {
		name = "OTHER1"

		subject_transform {
			source = "js.in.OTHER1.>"
			destination = "1.>"
		}
	}

	source {
		name = "OTHER2"

		subject_transform {
			source = "js.in.OTHER2.>"
			destination = "2.>"
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

const testStreamSubjectTransform = `
provider "jetstream" {
	servers = "%s"
}

resource "jetstream_stream" "test" {
	name = "TEST"
	subjects = ["TEST.*"]
	subject_transform {
		source = "TEST.>"
		destination = "1.>"
	}
}
`

const pedanticMaxAge = `
provider "jetstream" {
	servers = "%s"
}

resource "jetstream_stream" "max_age_test" {
  name             = "MAX_AGE_TEST"
  subjects         = ["MAX_AGE_TEXT.*"]
  duplicate_window = 2
  max_age          = 1
}`

const pedanticMirrorDirect = `
provider "jetstream" {
	servers = "%s"
}

resource "jetstream_stream" "pedantic_source" {
  name         = "PEDANTIC_SOURCE"
  subjects     = ["PEDANTIC_SOURCE.*"]
  allow_direct = false
}

resource "jetstream_stream" "pedantic_mirror" {
  name          = "PEDANTIC_MIRROR"
  mirror_direct = true
  mirror {
    name = "PEDANTIC_SOURCE"
  }

  depends_on    = [ jetstream_stream.pedantic_source ]
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
		ProviderFactories: testJsProviders,
		CheckDestroy: resource.ComposeTestCheckFunc(
			testStreamDoesNotExist(t, mgr, "TEST"),
			testStreamDoesNotExist(t, mgr, "OTHER"),
			testStreamDoesNotExist(t, mgr, "OTHER1"),
			testStreamDoesNotExist(t, mgr, "OTHER2")),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testStreamConfigBasic, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
					testStreamHasSubjects(t, mgr, "TEST", []string{"TEST.*"}),
					testStreamHasMetadata(t, mgr, "TEST", map[string]string{"foo": "bar"}),
					testStreamIsCompressed(t, mgr, "TEST"),
					resource.TestCheckResourceAttr("jetstream_stream.test", "max_msgs", "-1"),
					resource.TestCheckResourceAttr("jetstream_stream.test", "description", "testing stream"),
					resource.TestCheckResourceAttr("jetstream_stream.test", "deny_delete", "false"),
					resource.TestCheckResourceAttr("jetstream_stream.test", "compression", "s2"),
				),
			},
			{
				Config: fmt.Sprintf(testStreamConfigOtherSubjects, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
					testStreamIsUnCompressed(t, mgr, "TEST"),
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
				Config: fmt.Sprintf(typeStreamConfigMirrorTransformed, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "MIRROR_TRANSFORM_TEST"),
					resource.TestCheckResourceAttr("jetstream_stream.mirror_transform_test", "mirror.0.name", "OTHER"),
					resource.TestCheckResourceAttr("jetstream_stream.mirror_transform_test", "mirror.0.start_seq", "11"),
					resource.TestCheckResourceAttr("jetstream_stream.mirror_transform_test", "mirror.0.subject_transform.0.source", "js.in.OTHER.1.>"),
					resource.TestCheckResourceAttr("jetstream_stream.mirror_transform_test", "mirror.0.subject_transform.0.destination", "1.>"),
					resource.TestCheckResourceAttr("jetstream_stream.mirror_transform_test", "mirror.0.subject_transform.1.source", "js.in.OTHER.2.>"),
					resource.TestCheckResourceAttr("jetstream_stream.mirror_transform_test", "mirror.0.subject_transform.1.destination", "2.>"),
					testStreamIsMirrorOf(t, mgr, "MIRROR_TRANSFORM_TEST", "OTHER"),
					testStreamIsMirrorTransformed(t, mgr, "MIRROR_TRANSFORM_TEST", api.SubjectTransformConfig{Source: "js.in.OTHER.1.>", Destination: "1.>"}),
					testStreamHasSubjects(t, mgr, "MIRROR_TRANSFORM_TEST", []string{}),
				),
			},
			{
				Config: fmt.Sprintf(typeStreamConfigSourcesTransformed, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "SOURCE_TRANSFORM_TEST"),
					resource.TestCheckResourceAttr("jetstream_stream.source_transform_test", "source.0.name", "OTHER1"),
					resource.TestCheckResourceAttr("jetstream_stream.source_transform_test", "source.0.subject_transform.0.source", "js.in.OTHER1.>"),
					resource.TestCheckResourceAttr("jetstream_stream.source_transform_test", "source.0.subject_transform.0.destination", "1.>"),
					resource.TestCheckResourceAttr("jetstream_stream.source_transform_test", "source.1.name", "OTHER2"),
					resource.TestCheckResourceAttr("jetstream_stream.source_transform_test", "source.1.subject_transform.0.source", "js.in.OTHER2.>"),
					resource.TestCheckResourceAttr("jetstream_stream.source_transform_test", "source.1.subject_transform.0.destination", "2.>"),
					testStreamIsSourceOf(t, mgr, "SOURCE_TRANSFORM_TEST", []string{"OTHER1", "OTHER2"}),
					testStreamIsSourceTransformed(t, mgr, "SOURCE_TRANSFORM_TEST", "OTHER1", api.SubjectTransformConfig{Source: "js.in.OTHER1.>", Destination: "1.>"}),
					testStreamIsSourceTransformed(t, mgr, "SOURCE_TRANSFORM_TEST", "OTHER2", api.SubjectTransformConfig{Source: "js.in.OTHER2.>", Destination: "2.>"}),
					testStreamHasSubjects(t, mgr, "SOURCE_TRANSFORM_TEST", []string{}),
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
			{
				Config: fmt.Sprintf(testStreamSubjectTransform, nc.ConnectedUrl()),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
					resource.TestCheckResourceAttr("jetstream_stream.test", "subject_transform.0.source", "TEST.>"),
					resource.TestCheckResourceAttr("jetstream_stream.test", "subject_transform.0.destination", "1.>"),
					testStreamHasSubjects(t, mgr, "TEST", []string{"TEST.*"}),
					testStreamIsTransformed(t, mgr, "TEST", api.SubjectTransformConfig{Source: "TEST.>", Destination: "1.>"}),
				),
			},
			{
				Config:      fmt.Sprintf(pedanticMaxAge, nc.ConnectedUrl()),
				ExpectError: regexp.MustCompile(`duplicates window can not be larger then max age \(10052\)`),
			},
			{
				Config:      fmt.Sprintf(pedanticMirrorDirect, nc.ConnectedUrl()),
				ExpectError: regexp.MustCompile(`origin stream has direct get set, mirror has it disabled \(10157\)`),
			},
		},
	})
}
