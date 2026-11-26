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
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats.go"
	ntf "github.com/synadia-io/orbit.go/ntf-client"
	"github.com/synadia-io/orbit.go/ntf/api"
)

const testTLSConfig = `
provider "jetstream" {
  servers = "%s"
  tls {
%s
  }
}

resource "jetstream_stream" "test" {
  name     = "TEST"
  subjects = ["TEST.*"]
}
`

func TestProviderWithTLS(t *testing.T) {
	heredoc := func(key string, pem string) string {
		return fmt.Sprintf("%s = <<EOT\n%s\nEOT\n", key, pem)
	}

	for _, tc := range []struct {
		name string
		mode ntf.TLSOpt
		tls  func(t *testing.T, certs *api.TLSMaterial) string
	}{
		{"ca_file_data", ntf.TLSServerOnly(), func(t *testing.T, certs *api.TLSMaterial) string {
			return heredoc("ca_file_data", certs.CAPEM)
		}},
		{"ca_file", ntf.TLSServerOnly(), func(t *testing.T, certs *api.TLSMaterial) string {
			return fmt.Sprintf("ca_file = %q", writePEM(t, certs.CAPEM))
		}},
		{"client_file_data", ntf.TLSMutual(), func(t *testing.T, certs *api.TLSMaterial) string {
			return heredoc("ca_file_data", certs.CAPEM) + heredoc("cert_file_data", certs.ClientCertPEM) + heredoc("key_file_data", certs.ClientKeyPEM)
		}},
		{"client_file", ntf.TLSMutual(), func(t *testing.T, certs *api.TLSMaterial) string {
			return fmt.Sprintf("ca_file = %q\ncert_file = %q\nkey_file = %q", writePEM(t, certs.CAPEM), writePEM(t, certs.ClientCertPEM), writePEM(t, certs.ClientKeyPEM))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager, inst *ntf.Instance) {
				resource.Test(t, resource.TestCase{
					ProtoV5ProviderFactories: testJsProviders,
					Steps: []resource.TestStep{
						{
							Config: fmt.Sprintf(testTLSConfig, nc.ConnectedUrl(), tc.tls(t, inst.TLS)),
							Check:  testStreamExist(t, mgr, "TEST"),
						},
					},
				})
			}, ntf.WithGeneratedTLS(tc.mode))
		})
	}
}

func writePEM(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "file.pem")
	err := os.WriteFile(path, []byte(contents), 0600)
	checkErr(t, err, "could not write PEM file: %v", err)

	return path
}
