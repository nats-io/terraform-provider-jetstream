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

const testClientTLSConfig_file_data = `
provider "jetstream" {
  servers = "%s"
  tls {
    ca_file_data = <<EOT
%s
    EOT
    cert_file_data = <<EOT
%s
    EOT
    key_file_data = <<EOT
%s
    EOT
  }
}

resource "jetstream_stream" "test" {
  name     = "TEST"
  subjects = ["TEST.*"]
}
`

const testClientTLSConfig_file = `
provider "jetstream" {
  servers = "%s"
  tls {
    ca_file = "%s"
    cert_file = "%s"
    key_file = "%s"
  }
}

resource "jetstream_stream" "test" {
  name     = "TEST"
  subjects = ["TEST.*"]
}
`

func TestProviderWithTLSFromData(t *testing.T) {
	srv := createJSTLSServer(t, false)
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
	srv := createJSTLSServer(t, false)
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

func TestProviderWithClientTLSFromData(t *testing.T) {
	srv := createJSTLSServer(t, true)
	defer srv.Shutdown()

	caFile, cleanupCa, err := newTempPEMFile(caPEM)
	if err != nil {
		t.Fatalf("failed to create temp CA PEM: %s", err)
	}
	defer cleanupCa()

	certFile, cleanupCert, err := newTempPEMFile(certPEM)
	if err != nil {
		t.Fatalf("failed to create temp cert PEM: %s", err)
	}
	defer cleanupCert()

	keyFile, cleanupKey, err := newTempPEMFile(keyPEM)
	if err != nil {
		t.Fatalf("failed to create temp key PEM: %s", err)
	}
	defer cleanupKey()

	nc, err := nats.Connect(srv.ClientURL(), nats.RootCAs(caFile), nats.ClientCert(certFile, keyFile))
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
				Config: fmt.Sprintf(testClientTLSConfig_file_data, nc.ConnectedUrl(), caPEM, certPEM, keyPEM),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
				),
			},
		},
	})
}

func TestProviderWithClientTLSFromFile(t *testing.T) {
	srv := createJSTLSServer(t, true)
	defer srv.Shutdown()

	caFile, cleanupCa, err := newTempPEMFile(caPEM)
	if err != nil {
		t.Fatalf("failed to create temp CA PEM: %s", err)
	}
	defer cleanupCa()

	certFile, cleanupCert, err := newTempPEMFile(certPEM)
	if err != nil {
		t.Fatalf("failed to create temp cert PEM: %s", err)
	}
	defer cleanupCert()

	keyFile, cleanupKey, err := newTempPEMFile(keyPEM)
	if err != nil {
		t.Fatalf("failed to create temp key PEM: %s", err)
	}
	defer cleanupKey()

	nc, err := nats.Connect(srv.ClientURL(), nats.RootCAs(caFile), nats.ClientCert(certFile, keyFile))
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
				Config: fmt.Sprintf(testClientTLSConfig_file, nc.ConnectedUrl(), caFile, certFile, keyFile),
				Check: resource.ComposeTestCheckFunc(
					testStreamExist(t, mgr, "TEST"),
				),
			},
		},
	})
}
