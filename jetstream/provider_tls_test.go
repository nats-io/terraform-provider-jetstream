package jetstream

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats.go"
)

const testTLSConfig_file_data = `
provider "jetstream" {
  servers = "%s"
  tls {
	  ca_file_data = <<EOT
%s
	  EOT
  }
}

resource "jetstream_stream" "test" {
  name     = "TEST"
  subjects = ["TEST.*"]
}
`

const testTLSConfig_file = `
provider "jetstream" {
  servers = "%s"
  tls {
	  ca_file = "%s"
  }
}

resource "jetstream_stream" "test" {
  name     = "TEST"
  subjects = ["TEST.*"]
}
`

func TestProviderWithTLSFromData(t *testing.T) {
	srv := createJSTLSServer(t)
	defer srv.Shutdown()

	caFile, cleanup, err := newTempPEMFile(caPEM)

	if err != nil {
		t.Fatalf("failed to create temp CA PEM: %s", err)
	}
	defer cleanup()

	nc, err := nats.Connect(srv.ClientURL(), nats.RootCAs(caFile))
	if err != nil {
		t.Fatalf("could not connect: %s", err)
	}
	defer nc.Close()

	mgr, err := jsm.New(nc)
	if err != nil {
		t.Fatalf("could not connect: %s", err)
	}

	resource.Test(t, resource.TestCase{
		Providers: testJsProviders,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testTLSConfig_file_data, nc.ConnectedUrl(), caPEM),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
				),
			},
		},
	})
}

func TestProviderWithTLSFromFile(t *testing.T) {
	srv := createJSTLSServer(t)
	defer srv.Shutdown()

	caFile, cleanup, err := newTempPEMFile(caPEM)
	if err != nil {
		t.Fatalf("failed to create temp CA PEM: %s", err)
	}
	defer cleanup()

	nc, err := nats.Connect(srv.ClientURL(), nats.RootCAs(caFile))
	if err != nil {
		t.Fatalf("could not connect: %s", err)
	}
	defer nc.Close()

	mgr, err := jsm.New(nc)
	if err != nil {
		t.Fatalf("could not connect: %s", err)
	}

	resource.Test(t, resource.TestCase{
		Providers: testJsProviders,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testTLSConfig_file, nc.ConnectedUrl(), caFile),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
				),
			},
		},
	})
}
