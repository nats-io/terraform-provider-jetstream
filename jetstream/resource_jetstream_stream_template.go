package jetstream

import (
	"fmt"

	"github.com/nats-io/jsm.go/api"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/nats-io/jsm.go"
)

func resourceStreamTemplate() *schema.Resource {
	return &schema.Resource{
		Create: resourceStreamTemplateCreate,
		Read:   resourceStreamTemplateRead,
		Delete: resourceStreamTemplateDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:        schema.TypeString,
				Description: "The name of the Stream Template",
				Required:    true,
				ForceNew:    true,
			},
			"max_streams": &schema.Schema{
				Type:        schema.TypeInt,
				Description: "Maximum number of Streams this Template can manage",
				Required:    true,
				ForceNew:    true,
			},
			"subjects": &schema.Schema{
				Type:        schema.TypeList,
				MinItems:    1,
				Description: "The list of subjects that will be consumed by the Stream",
				Required:    true,
				ForceNew:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"max_msgs": &schema.Schema{
				Type:        schema.TypeInt,
				Description: "The maximum amount of messages that can be kept in the stream",
				Optional:    true,
				ForceNew:    true,
				Default:     -1,
			},
			"max_bytes": &schema.Schema{
				Type:        schema.TypeInt,
				Description: "The maximum size of all messages that can be kept in the stream",
				Optional:    true,
				ForceNew:    true,
				Default:     -1,
			},
			"max_age": &schema.Schema{
				Type:        schema.TypeInt,
				Description: "The maximum oldest message that can be kept in the stream, duration specified in seconds",
				Optional:    true,
				ForceNew:    true,
				Default:     -1,
			},
			"duplicate_window": &schema.Schema{
				Type:        schema.TypeInt,
				Description: "The size of the duplicate tracking windows, duration specified in seconds",
				Optional:    true,
				ForceNew:    true,
				Default:     120,
			},
			"max_msg_size": &schema.Schema{
				Type:        schema.TypeInt,
				Description: "The maximum individual message size that the stream will accept",
				Optional:    true,
				ForceNew:    true,
				Default:     -1,
			},
			"storage": &schema.Schema{
				Type:         schema.TypeString,
				Description:  "The storage engine to use to back the stream",
				Default:      "file",
				ForceNew:     true,
				Optional:     true,
				ValidateFunc: validateStorageTypeString(),
			},
			"ack": &schema.Schema{
				Type:        schema.TypeBool,
				Description: "If the Stream should support confirming receiving messages via acknowledgements",
				Optional:    true,
				Default:     true,
				ForceNew:    true,
			},
			"retention": &schema.Schema{
				Type:         schema.TypeString,
				Description:  "The retention policy to apply over and above max_msgs, max_bytes and max_age",
				Default:      "limits",
				Optional:     true,
				ForceNew:     true,
				ValidateFunc: validateRetentionTypeString(),
			},
			"max_consumers": &schema.Schema{
				Type:        schema.TypeInt,
				Description: "Number of consumers this stream allows",
				Default:     -1,
				Optional:    true,
				ForceNew:    true,
			},
			"replicas": &schema.Schema{
				Type:        schema.TypeInt,
				Description: "How many replicas of the data to keep in a clustered environment",
				Default:     1,
				Optional:    true,
				ForceNew:    true,
			},
		},
	}
}

func resourceStreamTemplateCreate(d *schema.ResourceData, m interface{}) error {
	cfg, err := streamConfigFromResourceData(d)
	if err != nil {
		return err
	}

	name := cfg.Name
	cfg.Name = ""

	_, err = jsm.NewStreamTemplate(name, uint32(d.Get("max_streams").(int)), cfg)
	if err != nil {
		return err
	}

	d.SetId(fmt.Sprintf("JETSTREAM_STREAMTEMPLATE_%s", name))

	return resourceStreamTemplateRead(d, m)
}

func resourceStreamTemplateRead(d *schema.ResourceData, m interface{}) error {
	tname, err := parseStreamTemplateID(d.Id())
	if err != nil {
		return err
	}

	known, err := jsm.IsKnownStreamTemplate(tname)
	if err != nil {
		return fmt.Errorf("could not determine if stream template %q is known: %s", tname, err)
	}
	if !known {
		d.SetId("")
		return nil
	}

	template, err := jsm.LoadStreamTemplate(tname)
	if err != nil {
		return fmt.Errorf("could not load stream template %q: %s", tname, err)
	}

	cfg := template.Configuration()
	str := cfg.Config

	d.Set("name", cfg.Name)
	d.Set("max_streams", cfg.MaxStreams)
	d.Set("subjects", str.Subjects)
	d.Set("max_consumers", str.MaxConsumers)
	d.Set("max_msgs", int(str.MaxMsgs))
	d.Set("max_age", str.MaxAge.Seconds())
	d.Set("duplicate_window", str.Duplicates.Seconds())
	d.Set("max_msg_size", int(str.MaxMsgSize))
	d.Set("replicas", str.Replicas)
	d.Set("ack", !str.NoAck)

	if str.MaxAge == -1 {
		d.Set("max_age", "-1")
	}

	switch str.Storage {
	case api.FileStorage:
		d.Set("storage", "file")
	case api.MemoryStorage:
		d.Set("storage", "memory")
	}

	switch str.Retention {
	case api.LimitsPolicy:
		d.Set("retention", "limits")
	case api.InterestPolicy:
		d.Set("retention", "interest")
	case api.WorkQueuePolicy:
		d.Set("retention", "workqueue")
	}

	return nil
}

func resourceStreamTemplateDelete(d *schema.ResourceData, m interface{}) error {
	name, err := parseStreamTemplateID(d.Id())
	if err != nil {
		return err
	}

	known, err := jsm.IsKnownStreamTemplate(name)
	if err != nil {
		return err
	}
	if !known {
		d.SetId("")
		return nil
	}

	str, err := jsm.LoadStreamTemplate(name)
	if err != nil {
		d.SetId("")
		return nil
	}

	return str.Delete()
}
