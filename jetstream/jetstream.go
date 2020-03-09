package jetstream

import (
	"regexp"

	"github.com/nats-io/jwt"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats.go"
)

var streamIdRegex = regexp.MustCompile("^JETSTREAM_STREAM_(.+)$")
var consumerIdRegex = regexp.MustCompile("^JETSTREAM_STREAM_(.+?)_CONSUMER_(.+)$")
var streamTemplateIdRegex = regexp.MustCompile("^JETSTREAM_STREAMTEMPLATE_(.+)$")

func Provider() terraform.ResourceProvider {
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
		},

		ResourcesMap: map[string]*schema.Resource{
			"jetstream_stream":          resourceStream(),
			"jetstream_consumer":        resourceConsumer(),
			"jetstream_stream_template": resourceStreamTemplate(),
		},

		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	creds := d.Get("credentials").(string)
	credData := []byte(d.Get("credential_data").(string))
	servers := d.Get("servers").(string)

	var opts []nats.Option

	switch {
	case creds != "":
		opts = append(opts, nats.UserCredentials(creds))

	case len(credData) > 0:
		defer wipeSlice(credData)

		userCB := func() (string, error) {
			return jwt.ParseDecoratedJWT(credData)
		}

		sigCB := func(nonce []byte) ([]byte, error) {
			kp, err := jwt.ParseDecoratedNKey(credData)
			if err != nil {
				return nil, err
			}
			defer kp.Wipe()

			return kp.Sign(nonce)
		}

		opts = append(opts, nats.UserJWT(userCB, sigCB))

	}

	err := jsm.Connect(servers, opts...)
	return nil, err
}
