package main

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
	"github.com/nats-io/terraform-provider-jetstream/jetstream"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{ProviderFunc: jetstream.Provider})
}
