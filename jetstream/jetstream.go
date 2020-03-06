package jetstream

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats.go"
)

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"servers": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("NATS_URL", nil),
			},
			"credentials": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("NATS_CREDS", nil),
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"jetstream_stream":   resourceStream(),
			"jetstream_consumer": resourceConsumer(),
		},

		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	creds := d.Get("credentials").(string)
	servers := d.Get("servers").(string)

	var opts []nats.Option

	if creds != "" {
		opts = append(opts, nats.UserCredentials(creds))
	}

	err := jsm.Connect(servers, opts...)
	return nil, err
}
