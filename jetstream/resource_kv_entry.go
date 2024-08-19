package jetstream

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats.go"
)

func resourceKVEntry() *schema.Resource {
	return &schema.Resource{
		Create: resourceKVEntryCreate,
		Read:   resourceKVEntryRead,
		Update: resourceKVEntryUpdate,
		Delete: resourceKVEntryDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"bucket": {
				Type:        schema.TypeString,
				Description: "The name of the bucket",
				Required:    true,
				ForceNew:    true,
			},
			"key": {
				Type:        schema.TypeString,
				Description: "The key of the entry",
				Required:    true,
				ForceNew:    true,
			},
			"value": {
				Type:        schema.TypeString,
				Description: "The value of the entry",
				Required:    true,
				ForceNew:    false,
			},
			"revision": {
				Type:        schema.TypeInt,
				Description: "The revision of the entry",
				Computed:    true,
			},
		},
	}
}

func resourceKVEntryCreate(d *schema.ResourceData, m any) error {
	nc, _, err := m.(func() (*nats.Conn, *jsm.Manager, error))()
	if err != nil {
		return err
	}
	defer nc.Close()

	bucket := d.Get("bucket").(string)
	key := d.Get("key").(string)
	value := d.Get("value").(string)

	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	kv, err := js.KeyValue(bucket)
	if err != nil {
		return err
	}
	_, err = kv.Put(key, []byte(value))
	if err != nil {
		return err
	}

	d.SetId(fmt.Sprintf("JETSTREAM_KV_%s_ENTRY_%s", bucket, key))

	return resourceKVEntryRead(d, m)
}

func resourceKVEntryRead(d *schema.ResourceData, m any) error {
	bucket, key, err := parseStreamKVEntryID(d.Id())
	if err != nil {
		return err
	}

	nc, _, err := m.(func() (*nats.Conn, *jsm.Manager, error))()
	if err != nil {
		return err
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	kv, err := js.KeyValue(bucket)
	if err != nil {
		if err == nats.ErrBucketNotFound {
			d.SetId("")
			return nil
		}
		return err
	}
	entry, err := kv.Get(key)
	if err != nil {
		if err == nats.ErrKeyNotFound {
			d.SetId("")
			return nil
		}
		return err
	}

	d.Set("bucket", entry.Bucket())
	d.Set("key", entry.Key())
	d.Set("value", string(entry.Value()))
	d.Set("revision", entry.Revision())

	return nil
}

func resourceKVEntryUpdate(d *schema.ResourceData, m any) error {
	bucket := d.Get("bucket").(string)

	nc, _, err := m.(func() (*nats.Conn, *jsm.Manager, error))()
	if err != nil {
		return err
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	kv, err := js.KeyValue(bucket)
	if err != nil {
		return err
	}

	key := d.Get("key").(string)
	value := d.Get("value").(string)

	_, err = kv.Put(key, []byte(value))
	if err != nil {
		return err
	}

	return resourceKVEntryRead(d, m)
}

func resourceKVEntryDelete(d *schema.ResourceData, m any) error {
	bucket := d.Get("bucket").(string)

	nc, _, err := m.(func() (*nats.Conn, *jsm.Manager, error))()
	if err != nil {
		return err
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	kv, err := js.KeyValue(bucket)
	if err != nil {
		return err
	}
	err = kv.Delete(d.Get("key").(string))
	if err != nil {
		return err
	}

	return nil
}
