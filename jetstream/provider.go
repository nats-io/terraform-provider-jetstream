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
	"regexp"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var streamIdRegex = regexp.MustCompile("^JETSTREAM_STREAM_(.+)$")
var consumerIdRegex = regexp.MustCompile("^JETSTREAM_STREAM_(.+?)_CONSUMER_(.+)$")
var kvIdRegex = regexp.MustCompile("^JETSTREAM_KV_(.+)$")
var kvEntryIdRegex = regexp.MustCompile("^JETSTREAM_KV_(.+?)_ENTRY_(.+)$")

func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"servers": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Comma separated list of NATS servers to connect to",
				DefaultFunc: schema.EnvDefaultFunc("NATS_URL", nil),
			},
			"credentials": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Path to the NATS 2.0 credentials file to use for authentication",
				DefaultFunc: schema.EnvDefaultFunc("NATS_CREDS", nil),
			},
			"credential_data": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The contents of the NATS 2.0 Credentials file to use",
			},
			"user": {
				Type:          schema.TypeString,
				Optional:      true,
				Description:   "Connect using an username, used as token when no password is given",
				ConflictsWith: []string{"nkey", "credentials", "credential_data"},
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Connect using a password",
			},
			"nkey": {
				Type:          schema.TypeString,
				Optional:      true,
				Description:   "Connect using a NKEY seed stored in a file",
				ConflictsWith: []string{"user", "credentials", "credential_data"},
			},
			"tls": {
				Type:     schema.TypeSet,
				MaxItems: 1,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"ca_file": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Path to the server root CA file (in PEM format). Needed when the NATS server has TLS enabled with an unknown root CA",
						},
						"ca_file_data": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "The root CA (file) content, in PEM format. Needed when the NATS server has TLS enabled with an unknown root CA",
						},
						"cert_file": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Path to client cert file (in PEM format). Needed when NATS server is configured to verify client certificate",
						},
						"cert_file_data": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "The cert file content (in PEM format). Needed when NATS server is configured to verify client certificate",
						},
						"key_file": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Path to client key file (in PEM format). Needed when NATS server is configured to verify client certificate",
						},
						"key_file_data": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "The key file content (in PEM format). Needed when NATS server is configured to verify client certificate",
						},
					},
				},
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"jetstream_stream":    resourceStream(),
			"jetstream_consumer":  resourceConsumer(),
			"jetstream_kv_bucket": resourceKVBucket(),
			"jetstream_kv_entry":  resourceKVEntry(),
		},

		ConfigureFunc: connectMgr,
	}
}
